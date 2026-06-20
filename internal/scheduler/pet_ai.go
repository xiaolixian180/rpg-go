// Package scheduler 提供定时任务调度功能
package scheduler

import (
	"math/rand"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// PetAIBehavior 宠物AI行为接口
type PetAIBehavior interface {
	// Execute 执行宠物AI行为，返回是否执行了动作
	Execute(pet *model.Pet, player *model.Player, target *model.Monster, world iface.World) bool
}

// AttackPetBehavior 攻击型宠物AI：高伤害输出，优先攻击低血量目标
type AttackPetBehavior struct{}

func (b *AttackPetBehavior) Execute(pet *model.Pet, player *model.Player, target *model.Monster, world iface.World) bool {
	pet.Mu().RLock()
	petHP := pet.HP
	petAtk := pet.Attack
	petCrit := 0.15 // 攻击型宠物暴击率15%
	pet.Mu().RUnlock()

	if petHP <= 0 || target == nil {
		return false
	}

	target.Mu().RLock()
	targetDead := target.Dead
	target.Mu().RUnlock()
	if targetDead {
		return false
	}

	// 计算伤害
	damage := petAtk + int64(float64(petAtk)*0.2*rand.Float64())
	isCrit := rand.Float64() < petCrit
	if isCrit {
		damage = int64(float64(damage) * 1.8)
	}

	// 对目标施加伤害
	target.Mu().Lock()
	if !target.Dead {
		target.Hp -= damage
		if target.Hp < 0 {
			target.Hp = 0
			target.Dead = true
		}
	}
	newHp := target.Hp
	targetDead = target.Dead
	target.Mu().Unlock()

	logger.Debug("攻击型宠物攻击", "pet", pet.Name, "target", target.Name,
		"damage", damage, "crit", isCrit, "target_hp", newHp)

	// 如果击杀目标，给玩家奖励
	if targetDead {
		petAttackReward(player, target)
	}

	return true
}

// DefensePetBehavior 防御型宠物AI：嘲讽怪物，减少玩家受到的伤害
type DefensePetBehavior struct{}

func (b *DefensePetBehavior) Execute(pet *model.Pet, player *model.Player, target *model.Monster, world iface.World) bool {
	pet.Mu().RLock()
	petHP := pet.HP
	pet.Mu().RUnlock()

	if petHP <= 0 {
		return false
	}

	// 防御型宠物：将怪物仇恨转移到自己身上
	if target != nil {
		target.Mu().Lock()
		// 嘲讽：将怪物目标改为宠物（使用宠物UID的负值作为标识）
		target.TargetID = 0 // 清除怪物对玩家的仇恨
		target.AIState = model.MonsterAIAlert
		target.Mu().Unlock()

		logger.Debug("防御型宠物嘲讽", "pet", pet.Name, "target", target.Name)
	}

	// 防御型宠物吸收伤害：减少玩家受到的伤害
	// 这个效果通过 monster_ai.go 中的伤害计算来实现
	// 宠物存在时，怪物对玩家的伤害降低 petDef/2
	return true
}

// SupportPetBehavior 辅助型宠物AI：定期治疗玩家
type SupportPetBehavior struct {
	lastHealTime int64
	healCD       int64 // 治疗冷却（秒）
}

func NewSupportPetBehavior() *SupportPetBehavior {
	return &SupportPetBehavior{healCD: 5}
}

func (b *SupportPetBehavior) Execute(pet *model.Pet, player *model.Player, target *model.Monster, world iface.World) bool {
	pet.Mu().RLock()
	petHP := pet.HP
	petAtk := pet.Attack // 辅助型宠物的攻击力用于治疗量计算
	pet.Mu().RUnlock()

	if petHP <= 0 {
		return false
	}

	now := time.Now().Unix()
	if now-b.lastHealTime < b.healCD {
		return false
	}

	// 治疗量 = 宠物攻击力 * 0.5
	healAmount := int64(float64(petAtk) * 0.5)
	if healAmount < 1 {
		healAmount = 1
	}

	player.Mu().Lock()
	playerHp := player.Hp
	playerMaxHp := player.MaxHp
	if playerHp < playerMaxHp {
		player.Hp += healAmount
		if player.Hp > playerMaxHp {
			player.Hp = playerMaxHp
		}
		playerHp = player.Hp
	}
	player.Mu().Unlock()

	b.lastHealTime = now

	logger.Debug("辅助型宠物治疗", "pet", pet.Name, "heal", healAmount, "player_hp", playerHp)

	// 通知客户端治疗结果
	world.Hub().SendToPlayer(player.ID, protocol.MsgIDDamage, &protocol.S2CDamage{
		TargetID: player.ID,
		Damage:   -healAmount, // 负数表示治疗
		CurrHp:   playerHp,
	})

	return true
}

// PlunderPetBehavior 掠夺型宠物AI：攻击时偷取金币
type PlunderPetBehavior struct{}

func (b *PlunderPetBehavior) Execute(pet *model.Pet, player *model.Player, target *model.Monster, world iface.World) bool {
	pet.Mu().RLock()
	petHP := pet.HP
	petAtk := pet.Attack
	pet.Mu().RUnlock()

	if petHP <= 0 || target == nil {
		return false
	}

	target.Mu().RLock()
	targetDead := target.Dead
	targetGold := target.GoldReward
	target.Mu().RUnlock()
	if targetDead {
		return false
	}

	// 掠夺型宠物伤害较低，但会偷取金币
	damage := int64(float64(petAtk) * 0.6)
	if damage < 1 {
		damage = 1
	}

	// 对目标施加伤害
	target.Mu().Lock()
	if !target.Dead {
		target.Hp -= damage
		if target.Hp < 0 {
			target.Hp = 0
			target.Dead = true
		}
	}
	target.Mu().Unlock()

	// 偷取金币：怪物金币奖励的20%
	stolenGold := int64(float64(targetGold) * 0.2)
	if stolenGold < 1 {
		stolenGold = 1
	}

	player.Mu().Lock()
	player.Gold += stolenGold
	playerGold := player.Gold
	player.Mu().Unlock()

	logger.Debug("掠夺型宠物偷取", "pet", pet.Name, "damage", damage,
		"stolen_gold", stolenGold, "player_gold", playerGold)

	return true
}

// petAttackReward 攻击型宠物击杀怪物时给玩家发放奖励
func petAttackReward(player *model.Player, monster *model.Monster) {
	expGain := monster.ExpReward
	goldGain := monster.GoldReward

	player.Mu().Lock()
	player.Exp += expGain
	player.Gold += goldGain
	player.Mu().Unlock()

	logger.Debug("宠物击杀奖励", "player_id", player.ID, "exp", expGain, "gold", goldGain)
}

// GetPetAIBehavior 根据宠物类型返回对应的AI行为
func GetPetAIBehavior(petType int32) PetAIBehavior {
	switch petType {
	case model.PetTypeAttack:
		return &AttackPetBehavior{}
	case model.PetTypeDefense:
		return &DefensePetBehavior{}
	case model.PetTypeSupport:
		return NewSupportPetBehavior()
	case model.PetTypePlunder:
		return &PlunderPetBehavior{}
	default:
		return &AttackPetBehavior{}
	}
}
