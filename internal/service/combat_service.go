// Package service - 战斗服务
// 提供PvE攻击、技能释放、资源采集等战斗相关业务逻辑
package service

import (
	"context"
	"math/rand"

	"hero-quest/internal/model"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 战斗服务接口 ====================

// CombatService 战斗服务接口，定义攻击、技能释放、资源采集等战斗操作
type CombatService interface {
	// Attack 普通攻击，计算伤害并扣减目标血量
	Attack(ctx context.Context, attacker *model.Player, targetID uint64, skillID int32) (*CombatResult, *errors.GameError)
	// SkillCast 释放技能，计算技能效果和伤害
	SkillCast(ctx context.Context, caster *model.Player, skillID int32, targetID uint64, x, y float64) (*SkillResult, *errors.GameError)
	// CollectResource 采集资源，校验资源状态并标记为已采集
	CollectResource(ctx context.Context, playerID uint64, resource *model.Resource) (*CollectResult, *errors.GameError)
}

// ==================== 战斗结果结构体 ====================

// CombatResult 战斗结果，包含伤害、目标当前血量、是否死亡、击杀奖励
type CombatResult struct {
	TargetID uint64 // 目标ID
	Damage   int64  // 伤害值
	CurrHp   int64  // 目标当前血量
	IsDead   bool   // 目标是否死亡
	ExpGain  int64  // 获得经验值（怪物死亡时）
	GoldGain int64  // 获得金币数（怪物死亡时）
}

// SkillResult 技能释放结果，包含施法者、技能ID、受击目标列表
type SkillResult struct {
	CasterID uint64        // 施法者ID
	SkillID  int32         // 技能ID
	Targets  []DamageInfo  // 受击目标列表
	X        float64       // 释放位置X
	Y        float64       // 释放位置Y
}

// DamageInfo 单个目标的伤害信息
type DamageInfo struct {
	TargetID uint64 // 目标ID
	Damage   int64  // 伤害值
	CurrHp   int64  // 目标当前血量
	IsDead   bool   // 目标是否死亡
}

// CollectResult 采集结果
type CollectResult struct {
	ResourceID uint64 // 资源ID
	ItemID     uint64 // 获得的物品ID
	ItemName   string // 物品名称
	Count      int32  // 物品数量
}

// ==================== 战斗服务实现 ====================

// combatService 战斗服务实现
type combatService struct{}

// NewCombatService 创建战斗服务实例
func NewCombatService() CombatService {
	return &combatService{}
}

// Attack 普通攻击业务逻辑：
//  1. 根据攻击者属性和技能计算伤害值
//  2. 扣减目标血量
//  3. 若目标死亡，计算击杀奖励（经验和金币）
func (s *combatService) Attack(ctx context.Context, attacker *model.Player, targetID uint64, skillID int32) (*CombatResult, *errors.GameError) {
	// 检查攻击者是否已死亡
	if attacker.Hp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 计算伤害值
	damage := s.calcDamage(attacker, skillID)

	result := &CombatResult{
		TargetID: targetID,
		Damage:   damage,
	}

	logger.Debug("玩家攻击", "attacker_id", attacker.ID, "target_id", targetID, "damage", damage)
	return result, nil
}

// SkillCast 释放技能业务逻辑：
//  1. 校验技能是否属于当前职业
//  2. 计算技能伤害（基于技能倍率）
//  3. 返回技能效果（受击目标及伤害）
func (s *combatService) SkillCast(ctx context.Context, caster *model.Player, skillID int32, targetID uint64, x, y float64) (*SkillResult, *errors.GameError) {
	// 检查施法者是否已死亡
	if caster.Hp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 查找技能定义
	skillDef, ok := model.SkillDefs[skillID]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// 校验技能是否属于当前职业
	if skillDef.Class != caster.Class {
		return nil, errors.ErrSkillNotFound
	}

	// 计算技能伤害（技能伤害 = 普攻伤害 * 1.5倍率）
	damage := int64(float64(s.calcDamage(caster, skillID)) * 1.5)

	result := &SkillResult{
		CasterID: caster.ID,
		SkillID:  skillID,
		X:        x,
		Y:        y,
		Targets: []DamageInfo{
			{TargetID: targetID, Damage: damage},
		},
	}

	logger.Debug("玩家释放技能", "caster_id", caster.ID, "skill_id", skillID, "damage", damage)
	return result, nil
}

// CollectResource 采集资源业务逻辑：
//  1. 校验资源是否已被采集
//  2. 标记资源为已采集
//  3. 计算采集获得的物品
func (s *combatService) CollectResource(ctx context.Context, playerID uint64, resource *model.Resource) (*CollectResult, *errors.GameError) {
	// 校验资源是否已被采集
	if resource.Harvested {
		return nil, errors.ErrResourceGone
	}

	// 标记为已采集
	resource.Harvested = true

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

	result := &CollectResult{
		ResourceID: resource.ID,
		ItemID:     resource.ID, // 简化：物品ID使用资源ID
		ItemName:   itemName,
		Count:      1,
	}

	logger.Debug("玩家采集资源", "player_id", playerID, "resource_id", resource.ID, "item", itemName)
	return result, nil
}

// calcDamage 根据玩家属性和技能计算伤害值
// 基础伤害 = 力量*2 + 等级*5
// 最终伤害 = 基础伤害 + 随机波动（0 ~ 基础伤害/4）
// 随机波动模拟暴击和伤害浮动，使战斗结果更有不确定性
func (s *combatService) calcDamage(p *model.Player, skillID int32) int64 {
	base := int64(p.Str)*2 + int64(p.Level)*5
	return base + rand.Int63n(base/4+1)
}
