// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"context"
	"time"

	"hero-quest/internal/eventbus"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service/combat"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterAutoBattleTask 注册自动战斗定时任务。
// 每2秒遍历所有在线玩家，对开启了自动战斗的玩家自动攻击当前层最近的怪物。
// 包含宠物AI协同作战。
func RegisterAutoBattleTask(s *Scheduler, world iface.World, combatSvc combat.CombatService, bus *eventbus.Bus) {
	s.Add("auto_battle", 2*time.Second, func() {
		world.AllPlayers(func(playerID uint64, p *model.Player) {
			p.Mu().RLock()
			autoEnabled := p.AutoBattle
			layer := p.Layer
			hp := p.Hp
			activePet := p.ActivePet
			p.Mu().RUnlock()

			if !autoEnabled || layer <= 0 || hp <= 0 {
				return
			}

			// 获取当前层的地下城
			dungeon := world.GetDungeon(layer)
			if dungeon == nil {
				return
			}

			// 查找最近的存活怪物
			monster := findNearestAliveMonster(dungeon, p)
			if monster == nil {
				return
			}

			// 通过 CombatService 执行攻击（使用普攻，skillID=0）
			cr, ge := combatSvc.Attack(context.Background(), p, monster.ID, 0)
			if ge != nil {
				return
			}

			// 向玩家发送攻击结果（包含击杀奖励信息）
			world.Hub().SendToPlayer(playerID, protocol.MsgIDDamage, &protocol.S2CDamage{
				TargetID:  cr.TargetID,
				Damage:    cr.Damage,
				CurrHp:    cr.CurrHp,
				IsDead:    cr.IsDead,
				ExpGain:   cr.ExpGain,
				GoldGain:  cr.GoldGain,
				LevelUp:   cr.LevelUped,
				NewLevel:  cr.NewLevel,
				PetDamage: cr.PetDamage,
				PetCrit:   cr.PetCrit,
				PetDead:   cr.PetDead,
			})

			// 击杀后更新排行榜
			if cr.IsDead && bus != nil {
				bus.Publish(eventbus.TopicLevelUp, &eventbus.LevelUpEvent{
					PlayerID: playerID,
					NewLevel: p.Level,
				})
			}

			// 宠物AI协同行为（独立于玩家攻击的宠物主动行为）
			if activePet != nil {
				petBehavior := GetPetAIBehavior(activePet.Type)
				petBehavior.Execute(activePet, p, monster, world)
			}

			logger.Debug("自动战斗攻击", "player_id", playerID, "target_id", monster.ID,
				"damage", cr.Damage, "is_dead", cr.IsDead, "pet_damage", cr.PetDamage)
		})
	})
}

// findNearestAliveMonster 在指定层中查找距离玩家最近的存活怪物
func findNearestAliveMonster(d *model.DungeonLayer, p *model.Player) *model.Monster {
	p.Mu().RLock()
	px, py := p.X, p.Y
	p.Mu().RUnlock()

	d.Mu().RLock()
	defer d.Mu().RUnlock()

	var nearest *model.Monster
	var minDist float64 = 1e9

	for _, m := range d.Monsters {
		m.Mu().RLock()
		dead := m.Dead
		mx, my := m.X, m.Y
		m.Mu().RUnlock()

		if dead {
			continue
		}

		dx := mx - px
		dy := my - py
		dist := dx*dx + dy*dy // 使用距离平方避免开方
		if dist < minDist {
			minDist = dist
			nearest = m
		}
	}

	return nearest
}
