// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"math"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterMonsterAITask 注册怪物AI行为定时任务。
// 每1秒遍历所有副本层的怪物，根据AI状态机更新怪物行为：
// 空闲→警戒→追击→攻击→返回，模拟原神风格的怪物行动模式。
func RegisterMonsterAITask(s *Scheduler, world iface.World) {
	s.Add("monster_ai", 1*time.Second, func() {
		now := time.Now().Unix()

		for layer := int32(1); layer <= world.MaxLayer(); layer++ {
			d := world.GetDungeon(layer)
			if d == nil {
				continue
			}

			// 获取该层所有玩家位置快照
			players := layerPlayerPositions(world, layer)
			if len(players) == 0 {
				// 没有玩家，所有怪物回归空闲
				resetAllMonsterAI(d)
				continue
			}

			d.Mu().RLock()
			monsters := make([]*model.Monster, 0, len(d.Monsters))
			for _, m := range d.Monsters {
				monsters = append(monsters, m)
			}
			d.Mu().RUnlock()

			for _, m := range monsters {
				m.Mu().RLock()
				dead := m.Dead
				m.Mu().RUnlock()
				if dead {
					continue
				}
				processMonsterAI(m, players, now, world, layer)
			}
		}
	})
}

// playerPos 玩家位置快照
type playerPos struct {
	id uint64
	x  float64
	y  float64
	hp int64
}

// layerPlayerPositions 获取指定层所有玩家的位置快照
func layerPlayerPositions(world iface.World, layer int32) []playerPos {
	playerIDs := world.LayerPlayerIDs(layer)
	positions := make([]playerPos, 0, len(playerIDs))
	for _, pid := range playerIDs {
		p := world.GetOnlinePlayer(pid)
		if p == nil {
			continue
		}
		p.Mu().RLock()
		if p.Hp > 0 {
			positions = append(positions, playerPos{id: p.ID, x: p.X, y: p.Y, hp: p.Hp})
		}
		p.Mu().RUnlock()
	}
	return positions
}

// processMonsterAI 处理单个怪物的AI状态机
func processMonsterAI(m *model.Monster, players []playerPos, now int64, world iface.World, layer int32) {
	m.Mu().Lock()
	defer m.Mu().Unlock()

	switch m.AIState {
	case model.MonsterAIIdle:
		// 空闲状态：检测仇恨范围内的玩家
		nearest := findNearestPlayer(m, players)
		if nearest != nil && distTo(m, nearest) <= m.AggroRange {
			m.AIState = model.MonsterAIAlert
			m.TargetID = nearest.id
			logger.Debug("怪物发现目标", "monster", m.Name, "target", nearest.id)
		}

	case model.MonsterAIAlert:
		// 警戒状态：短暂延迟后进入追击
		target := findPlayerByID(players, m.TargetID)
		if target == nil || distTo(m, target) > m.AggroRange*1.5 {
			// 目标消失或超出警戒范围，返回空闲
			m.AIState = model.MonsterAIIdle
			m.TargetID = 0
			return
		}
		m.AIState = model.MonsterAIChase

	case model.MonsterAIChase:
		// 追击状态：向玩家移动
		target := findPlayerByID(players, m.TargetID)
		if target == nil {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}

		dist := distTo(m, target)

		// 超出最大追击距离（仇恨范围*2），返回出生点
		if dist > m.AggroRange*2 {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}

		// 进入攻击范围，切换到攻击状态
		if dist <= m.AtkRange {
			m.AIState = model.MonsterAIAttack
			return
		}

		// 向玩家移动（根据行为类型调整速度）
		moveSpeed := monsterMoveSpeed(m)
		moveToward(m, target.x, target.y, moveSpeed)

	case model.MonsterAIAttack:
		// 攻击状态：对目标造成伤害
		target := findPlayerByID(players, m.TargetID)
		if target == nil {
			m.AIState = model.MonsterAIReturn
			m.TargetID = 0
			return
		}

		dist := distTo(m, target)

		// 玩家离开攻击范围，切换到追击
		if dist > m.AtkRange*1.2 {
			m.AIState = model.MonsterAIChase
			return
		}

		// 检查攻击冷却
		if now-m.LastAtkTime < int64(m.AtkCD) {
			return
		}

		// 执行攻击
		m.LastAtkTime = now
		monsterAttackPlayer(m, target.id, world, layer)

	case model.MonsterAIReturn:
		// 返回状态：回到出生点
		dx := m.SpawnX - m.X
		dy := m.SpawnY - m.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 1 {
			// 到达出生点，恢复空闲和HP
			m.X = m.SpawnX
			m.Y = m.SpawnY
			m.AIState = model.MonsterAIIdle
			m.Hp = m.MaxHp // 回满血
			return
		}

		// 向出生点移动
		moveSpeed := monsterMoveSpeed(m) * 1.5 // 返回时移动更快
		ratio := moveSpeed / dist
		m.X += dx * ratio
		m.Y += dy * ratio

		// 返回途中遇到玩家，重新进入追击
		nearest := findNearestPlayer(m, players)
		if nearest != nil && distTo(m, nearest) <= m.AggroRange {
			m.AIState = model.MonsterAIChase
			m.TargetID = nearest.id
		}
	}
}

// findNearestPlayer 找到距离怪物最近的存活玩家
func findNearestPlayer(m *model.Monster, players []playerPos) *playerPos {
	var nearest *playerPos
	var minDist float64 = 1e9
	for i := range players {
		d := distTo(m, &players[i])
		if d < minDist {
			minDist = d
			nearest = &players[i]
		}
	}
	return nearest
}

// findPlayerByID 按ID查找玩家
func findPlayerByID(players []playerPos, id uint64) *playerPos {
	for i := range players {
		if players[i].id == id {
			return &players[i]
		}
	}
	return nil
}

// distTo 计算怪物与玩家的距离
func distTo(m *model.Monster, p *playerPos) float64 {
	dx := m.X - p.x
	dy := m.Y - p.y
	return math.Sqrt(dx*dx + dy*dy)
}

// moveToward 向目标位置移动一定距离
func moveToward(m *model.Monster, tx, ty, speed float64) {
	dx := tx - m.X
	dy := ty - m.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < speed {
		m.X = tx
		m.Y = ty
	} else {
		ratio := speed / dist
		m.X += dx * ratio
		m.Y += dy * ratio
	}
}

// monsterMoveSpeed 根据怪物行为类型返回移动速度
func monsterMoveSpeed(m *model.Monster) float64 {
	switch m.Behavior {
	case model.MonsterBehaviorMelee:
		return 3.0 // 近战型移动快
	case model.MonsterBehaviorRanged:
		return 1.5 // 远程型移动慢
	case model.MonsterBehaviorTank:
		return 1.0 // 坦克型移动最慢
	case model.MonsterBehaviorHealer:
		return 2.0 // 辅助型中等速度
	default:
		return 2.0
	}
}

// monsterAttackPlayer 怪物攻击玩家（通过World接口发送伤害）
func monsterAttackPlayer(m *model.Monster, playerID uint64, world iface.World, layer int32) {
	p := world.GetOnlinePlayer(playerID)
	if p == nil {
		return
	}

	p.Mu().RLock()
	playerHp := p.Hp
	p.Mu().RUnlock()
	if playerHp <= 0 {
		return
	}

	// 怪物伤害计算：基础攻击 + 随机波动(0~15%)
	damage := m.Atk + int64(float64(m.Atk)*0.15*float64(time.Now().UnixNano()%100)/100)
	if damage < 1 {
		damage = 1
	}

	// 对玩家造成伤害
	p.Mu().Lock()
	p.Hp -= damage
	if p.Hp < 0 {
		p.Hp = 0
	}
	newHp := p.Hp
	p.Mu().Unlock()

	// 发送伤害通知给玩家
	world.Hub().SendToPlayer(playerID, protocol.MsgIDDamage, &protocol.S2CDamage{
		TargetID: playerID,
		Damage:   damage,
		CurrHp:   newHp,
		IsDead:   newHp <= 0,
	})

	logger.Debug("怪物攻击玩家", "monster", m.Name, "player_id", playerID,
		"damage", damage, "player_hp", newHp)

	// 玩家死亡通知
	if newHp <= 0 {
		world.Hub().SendToPlayer(playerID, protocol.MsgIDPlayerDie, &protocol.S2CPlayerDie{
			PlayerID:   playerID,
			KillerID:   m.ID,
			KillerName: m.Name,
		})
	}
}

// resetAllMonsterAI 将所有怪物AI状态重置为空闲（无玩家时）
func resetAllMonsterAI(d *model.DungeonLayer) {
	d.Mu().RLock()
	monsters := make([]*model.Monster, 0, len(d.Monsters))
	for _, m := range d.Monsters {
		monsters = append(monsters, m)
	}
	d.Mu().RUnlock()

	for _, m := range monsters {
		m.Mu().Lock()
		if m.AIState != model.MonsterAIIdle && !m.Dead {
			m.AIState = model.MonsterAIIdle
			m.TargetID = 0
			// 回到出生点
			m.X = m.SpawnX
			m.Y = m.SpawnY
			m.Hp = m.MaxHp // 回满血
		}
		m.Mu().Unlock()
	}
}
