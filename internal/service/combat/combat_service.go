// Package combat - 战斗服务
// 提供PvE攻击、技能释放、资源采集等战斗相关业务逻辑
package combat

import (
	"context"
	"math/rand"

	"hero-quest/internal/model"
	"hero-quest/internal/service/iface"
	"hero-quest/internal/service/player"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 战斗服务接口 ====================

// CombatService 战斗服务接口，定义攻击、技能释放、资源采集等战斗操作
type CombatService interface {
	// Attack 普通攻击，查找目标（怪物/Boss），计算伤害，扣减血量，处理死亡
	Attack(ctx context.Context, attacker *model.Player, targetID uint64, skillID int32) (*CombatResult, *errors.GameError)
	// SkillCast 释放技能，查找目标，计算技能效果和伤害
	SkillCast(ctx context.Context, caster *model.Player, skillID int32, targetID uint64, x, y float64) (*SkillResult, *errors.GameError)
	// CollectResource 采集资源，校验资源状态并标记为已采集
	CollectResource(ctx context.Context, playerID uint64, resource *model.Resource) (*CollectResult, *errors.GameError)
}

// ==================== 战斗结果结构体 ====================

// CombatResult 战斗结果，包含伤害、目标当前血量、是否死亡、击杀奖励
type CombatResult struct {
	TargetID  uint64 // 目标ID
	Damage    int64  // 实际造成的伤害值
	CurrHp    int64  // 目标当前血量
	IsDead    bool   // 目标是否死亡
	ExpGain   int64  // 获得经验值（怪物/Boss死亡时）
	GoldGain  int64  // 获得金币数（怪物/Boss死亡时）
	IsBoss    bool   // 目标是否为Boss
	IsDodge   bool   // 目标是否闪避了攻击
	IsCrit    bool   // 攻击是否暴击
	LevelUped bool   // 攻击者是否升级
	NewLevel  int32  // 升级后等级（未升级时为0）
}

// SkillResult 技能释放结果，包含施法者、技能ID、受击目标列表
type SkillResult struct {
	CasterID uint64       // 施法者ID
	SkillID  int32        // 技能ID
	Targets  []DamageInfo // 受击目标列表
	X        float64      // 释放位置X
	Y        float64      // 释放位置Y
}

// DamageInfo 单个目标的伤害信息
type DamageInfo struct {
	TargetID uint64 // 目标ID
	Damage   int64  // 伤害值
	CurrHp   int64  // 目标当前血量
	IsDead   bool   // 目标是否死亡
	IsDodge  bool   // 目标是否闪避
	IsCrit   bool   // 是否暴击
}

// CollectResult 采集结果
type CollectResult struct {
	ResourceID uint64 // 资源ID
	ItemID     uint64 // 获得的物品ID
	ItemName   string // 物品名称
	Count      int32  // 物品数量
}

// ==================== 战斗服务实现 ====================

// combatTarget 战斗目标，持有实体引用用于加锁操作
type combatTarget struct {
	monster *model.Monster // 怪物引用（非Boss时）
	boss    *model.Boss    // Boss引用（Boss时）
	isBoss  bool
}

// expReward 获取击杀经验奖励
func (t *combatTarget) expReward() int64 {
	if t.isBoss {
		return t.boss.MaxHp / 5 // Boss经验 = 最大血量的20%
	}
	return t.monster.ExpReward
}

// goldReward 获取击杀金币奖励
func (t *combatTarget) goldReward() int64 {
	if t.isBoss {
		return t.boss.MaxHp / 10 // Boss金币 = 最大血量的10%
	}
	return t.monster.GoldReward
}

// targetDef 获取目标防御力
func (t *combatTarget) targetDef() int64 {
	if t.isBoss {
		return 0 // Boss暂无防御属性
	}
	return t.monster.Def
}

// applyDamage 对目标施加伤害，返回剩余HP和是否首次击杀
// 使用 CAS 模式：对怪物设置 Dead 标记，只有第一个将 Dead 从 false 设为 true 的攻击者获得击杀判定
func (t *combatTarget) applyDamage(damage int64) (currHp int64, isDead bool) {
	if damage < 1 {
		damage = 1
	}
	if t.isBoss {
		t.boss.Mu().Lock()
		t.boss.Hp -= damage
		currHp = t.boss.Hp
		isDead = t.boss.Hp <= 0
		t.boss.Mu().Unlock()
	} else {
		t.monster.Mu().Lock()
		// 已死亡的怪物不再受击
		if t.monster.Dead {
			t.monster.Mu().Unlock()
			return t.monster.Hp, false
		}
		t.monster.Hp -= damage
		currHp = t.monster.Hp
		isDead = t.monster.Hp <= 0
		// CAS：只有首个将怪物击杀的攻击者获得 isDead=true
		if isDead {
			t.monster.Dead = true
		}
		t.monster.Mu().Unlock()
	}
	return
}

// combatService 战斗服务实现
type combatService struct {
	world     iface.World          // 游戏世界状态（查找怪物/Boss）
	playerSvc player.PlayerService // 玩家服务（用于升级）
}

// NewCombatService 创建战斗服务实例
func NewCombatService(world iface.World, playerSvc player.PlayerService) CombatService {
	return &combatService{world: world, playerSvc: playerSvc}
}

// findTarget 在玩家当前层查找战斗目标（怪物或Boss），返回实体引用
func (s *combatService) findTarget(playerLayer int32, targetID uint64) (*combatTarget, *errors.GameError) {
	// 先在当前层怪物表中查找
	dungeon := s.world.GetDungeon(playerLayer)
	if dungeon != nil {
		dungeon.Mu().RLock()
		if m, ok := dungeon.Monsters[targetID]; ok {
			m.Mu().RLock()
			dead := m.Dead
			m.Mu().RUnlock()
			dungeon.Mu().RUnlock()
			if dead {
				return nil, errors.ErrTargetNotFound
			}
			return &combatTarget{monster: m, isBoss: false}, nil
		}
		dungeon.Mu().RUnlock()
	}

	// 再在Boss表中查找
	if boss := s.world.GetBoss(targetID); boss != nil {
		return &combatTarget{boss: boss, isBoss: true}, nil
	}

	return nil, errors.ErrTargetNotFound
}

// playerCombatAttrs holds pre-computed player attributes read under RLock
type playerCombatAttrs struct {
	attack     int64
	critRate   float64
	critDamage float64
}

// readCombatAttrs reads player combat attributes under RLock
func readCombatAttrs(p *model.Player) playerCombatAttrs {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return playerCombatAttrs{
		attack:     p.CalcAttack(),
		critRate:   p.CalcCritRate(),
		critDamage: p.CalcCritDamage(),
	}
}

// calcDamage 根据预计算的玩家属性和技能计算伤害值
// 流程：基础攻击力 -> 技能倍率 -> 暴击判定 -> 随机波动 -> 最低1点
func (s *combatService) calcDamage(attrs playerCombatAttrs, skillID int32) (damage int64, isCrit bool) {
	// 基础攻击力
	attack := attrs.attack

	// 技能倍率（skillID > 0 时查表）
	multiplier := 1.0
	if skillID > 0 {
		if skillDef, ok := model.SkillDefs[skillID]; ok {
			multiplier = skillDef.Multiplier
		}
	}

	damage = int64(float64(attack) * multiplier)

	// 暴击判定
	isCrit = rand.Float64() < attrs.critRate
	if isCrit {
		damage = int64(float64(damage) * attrs.critDamage)
	}

	// 随机波动（0 ~ 10%）
	damage += rand.Int63n(damage/10 + 1)

	// 保底至少1点伤害
	if damage < 1 {
		damage = 1
	}

	return
}

// applyDefense 根据目标防御力减免伤害
func applyDefense(damage int64, def int64) int64 {
	result := damage - def
	if result < 1 {
		return 1
	}
	return result
}

// checkDodge 闪避判定（怪物无闪避，此处仅对怪物做随机闪避模拟）
// 怪物闪避率很低（5%），Boss闪避率为0
func checkDodge(target *combatTarget) bool {
	if target.isBoss {
		return false
	}
	// 普通怪物有5%概率闪避
	return rand.Float64() < 0.05
}

// rewardAttacker 给攻击者发放击杀奖励（经验和金币）
func rewardAttacker(attacker *model.Player, target *combatTarget) (expGain, goldGain int64) {
	expGain = target.expReward()
	goldGain = target.goldReward()

	attacker.Mu().Lock()
	attacker.Exp += expGain
	attacker.Gold += goldGain
	attacker.Mu().Unlock()

	return
}

// Attack 普通攻击业务逻辑：
//  1. 检查攻击者是否存活
//  2. 在当前层查找目标（怪物 -> Boss）
//  3. 计算伤害值（含技能倍率、暴击、防御减免）
//  4. 闪避判定
//  5. 扣减目标血量
//  6. 若目标死亡，计算击杀奖励（经验和金币），尝试升级
func (s *combatService) Attack(ctx context.Context, attacker *model.Player, targetID uint64, skillID int32) (*CombatResult, *errors.GameError) {
	// 检查攻击者是否已死亡
	attacker.Mu().RLock()
	attackerHp := attacker.Hp
	attackerLayer := attacker.Layer
	attackerID := attacker.ID
	attacker.Mu().RUnlock()

	if attackerHp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 查找目标
	target, ge := s.findTarget(attackerLayer, targetID)
	if ge != nil {
		return nil, ge
	}

	result := &CombatResult{
		TargetID: targetID,
		IsBoss:   target.isBoss,
	}

	// 闪避判定
	if checkDodge(target) {
		result.IsDodge = true
		result.Damage = 0
		// 闪避时也要返回当前HP
		if target.isBoss {
			target.boss.Mu().RLock()
			result.CurrHp = target.boss.Hp
			target.boss.Mu().RUnlock()
		} else {
			target.monster.Mu().RLock()
			result.CurrHp = target.monster.Hp
			target.monster.Mu().RUnlock()
		}
		logger.TDebug(ctx, "玩家攻击被闪避", "attacker_id", attackerID, "target_id", targetID)
		return result, nil
	}

	// 计算伤害值（含技能倍率和暴击），预计算属性值
	attackerAttrs := readCombatAttrs(attacker)
	rawDamage, isCrit := s.calcDamage(attackerAttrs, skillID)
	result.IsCrit = isCrit

	// 防御减伤
	def := target.targetDef()
	damage := applyDefense(rawDamage, def)
	result.Damage = damage

	// 对目标施加伤害（加锁保护）
	result.CurrHp, result.IsDead = target.applyDamage(damage)

	// 目标死亡时计算奖励（包括Boss和普通怪物）
	if result.IsDead {
		result.ExpGain, result.GoldGain = rewardAttacker(attacker, target)

		// 通过 PlayerService 统一处理升级逻辑
		if s.playerSvc != nil {
			expResult, _ := s.playerSvc.AddExp(ctx, attacker, result.ExpGain)
			if expResult != nil {
				result.LevelUped = expResult.LevelUped
				result.NewLevel = expResult.NewLevel
			}
		}
	}

	logger.TDebug(ctx, "玩家攻击", "attacker_id", attackerID, "target_id", targetID,
		"damage", damage, "is_dead", result.IsDead, "is_crit", isCrit)
	return result, nil
}

// SkillCast 释放技能业务逻辑：
//  1. 校验技能是否属于当前职业
//  2. 查找目标（怪物 -> Boss）
//  3. 计算技能伤害（基于技能倍率，含暴击和防御减免）
//  4. 闪避判定
//  5. 扣减目标血量
//  6. 返回技能效果（受击目标及伤害）
func (s *combatService) SkillCast(ctx context.Context, caster *model.Player, skillID int32, targetID uint64, x, y float64) (*SkillResult, *errors.GameError) {
	// 检查施法者是否已死亡
	caster.Mu().RLock()
	casterHp := caster.Hp
	casterLayer := caster.Layer
	casterID := caster.ID
	caster.Mu().RUnlock()

	if casterHp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 查找技能定义
	skillDef, ok := model.SkillDefs[skillID]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// 读取施法者职业
	caster.Mu().RLock()
	casterClass := caster.Class
	caster.Mu().RUnlock()

	// 校验技能是否属于当前职业
	if skillDef.Class != casterClass {
		return nil, errors.ErrSkillNotFound
	}

	// 查找目标
	target, ge := s.findTarget(casterLayer, targetID)
	if ge != nil {
		return nil, ge
	}

	// 闪避判定
	if checkDodge(target) {
		var currHp int64
		if target.isBoss {
			target.boss.Mu().RLock()
			currHp = target.boss.Hp
			target.boss.Mu().RUnlock()
		} else {
			target.monster.Mu().RLock()
			currHp = target.monster.Hp
			target.monster.Mu().RUnlock()
		}
		result := &SkillResult{
			CasterID: casterID,
			SkillID:  skillID,
			X:        x,
			Y:        y,
			Targets: []DamageInfo{
				{TargetID: targetID, Damage: 0, CurrHp: currHp, IsDodge: true},
			},
		}
		logger.TDebug(ctx, "技能攻击被闪避", "caster_id", casterID, "target_id", targetID)
		return result, nil
	}

	// 计算技能伤害（预计算属性值，使用技能倍率 + 暴击 + 防御减免）
	casterAttrs := readCombatAttrs(caster)
	rawDamage, isCrit := s.calcDamage(casterAttrs, skillID)
	def := target.targetDef()
	damage := applyDefense(rawDamage, def)

	// 对目标施加伤害（加锁保护）
	currHp, isDead := target.applyDamage(damage)

	result := &SkillResult{
		CasterID: casterID,
		SkillID:  skillID,
		X:        x,
		Y:        y,
		Targets: []DamageInfo{
			{TargetID: targetID, Damage: damage, CurrHp: currHp, IsDead: isDead, IsCrit: isCrit},
		},
	}

	// 目标死亡时计算奖励（与 Attack 一致，包括Boss）
	if isDead {
		expGain, goldGain := rewardAttacker(caster, target)
		// 奖励信息通过日志记录，不直接放入 SkillResult（SkillResult 是纯伤害结果）
		logger.TDebug(ctx, "技能击杀获得奖励", "caster_id", casterID, "target_id", targetID,
			"exp", expGain, "gold", goldGain)

		// 通过 PlayerService 统一处理升级逻辑（Issue 26: SkillCast triggers level-up）
		if s.playerSvc != nil {
			s.playerSvc.AddExp(ctx, caster, expGain)
		}
	}

	logger.TDebug(ctx, "玩家释放技能", "caster_id", casterID, "skill_id", skillID,
		"damage", damage, "is_crit", isCrit)
	return result, nil
}

// CollectResource 采集资源业务逻辑：
//  1. 校验资源是否已被采集
//  2. 标记资源为已采集
//  3. 计算采集获得的物品
func (s *combatService) CollectResource(ctx context.Context, playerID uint64, resource *model.Resource) (*CollectResult, *errors.GameError) {
	// 校验资源是否已被采集（加锁防止并发采集）
	resource.Mu().Lock()
	if resource.Harvested {
		resource.Mu().Unlock()
		return nil, errors.ErrResourceGone
	}
	resource.Harvested = true
	resource.Mu().Unlock()

	// 根据资源类型计算获得的物品
	var itemName string
	switch resource.Type {
	case 0:
		itemName = "矿石"
	case 1:
		itemName = "草药"
	case 2:
		itemName = "木材"
	default:
		itemName = "未知材料"
	}

	// 使用资源模板中定义的产出物品ID和数量
	itemID := uint64(resource.ItemID)
	count := resource.Count
	if count <= 0 {
		count = 1
	}

	result := &CollectResult{
		ResourceID: resource.ID,
		ItemID:     itemID,
		ItemName:   itemName,
		Count:      count,
	}

	logger.TDebug(ctx, "玩家采集资源", "player_id", playerID, "resource_id", resource.ID, "item", itemName)
	return result, nil
}
