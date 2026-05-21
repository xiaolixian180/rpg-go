// Package pvp - PvP服务
// 提供PvP攻击、金币掠夺、荣誉奖励、红名判定、悬赏、复仇等PvP相关业务逻辑
package pvp

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/rbac"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== PvP服务接口 ====================

// PvpService PvP服务接口，定义PvP攻击、红名查询、悬赏、复仇等操作
type PvpService interface {
	// Attack PvP攻击，对目标玩家造成伤害，含金币掠夺、荣誉奖励、红名判定
	Attack(ctx context.Context, attacker *model.Player, target *model.Player, skillID int32, pvpPenalty float64, pvpHonor int32, redNameThreshold int32) (*PvpResult, *errors.GameError)
	// GetRedNameList 获取红名玩家列表（杀戮值达到阈值的在线玩家）
	GetRedNameList(ctx context.Context, players map[uint64]*model.Player, redNameThreshold int32) []*RedNameInfo
	// BountyHunt 悬赏追杀，对红名玩家发起悬赏，实际进行战斗
	BountyHunt(ctx context.Context, hunter *model.Player, target *model.Player, redNameThreshold int32) (*BountyResult, *errors.GameError)
	// Revenge 复仇，对仇人发起复仇攻击（实际进行战斗）
	Revenge(ctx context.Context, attacker *model.Player, target *model.Player, skillID int32) (*PvpResult, *errors.GameError)
}

// ==================== PvP结果结构体 ====================

// PvpResult PvP战斗结果
type PvpResult struct {
	AttackerID uint64 // 攻击者ID
	TargetID   uint64 // 目标ID
	Damage     int64  // 伤害值
	GoldGain   int64  // 攻击者掠夺的金币
	HonorGain  int32  // 攻击者获得的荣誉值
	IsDead     bool   // 目标是否死亡
	IsRedName  bool   // 攻击者是否成为红名
	RedMsg     string // 红名播报内容（成为红名时）
	IsDodge    bool   // 目标是否闪避
	IsCrit     bool   // 是否暴击
}

// RedNameInfo 红名玩家信息
type RedNameInfo struct {
	PlayerID  uint64 // 玩家ID
	Name      string // 玩家名称
	KillValue int32  // 杀戮值
	Bounty    int64  // 赏金金额
}

// BountyResult 悬赏追杀结果
type BountyResult struct {
	TargetID  uint64 // 目标ID
	Damage    int64  // 造成的伤害
	GoldGain  int64  // 获得金币
	HonorGain int32  // 获得荣誉
	IsDead    bool   // 目标是否被击杀
	IsCrit    bool   // 是否暴击
}

// ==================== PvP服务实现 ====================

// pvpService PvP服务实现
type pvpService struct {
	enforcer *rbac.Enforcer   // Casbin 权限执行器，用于校验PvP区域权限
	cfg      iface.GameConfig // 游戏配置（悬赏奖励等）
}

// NewPvpService 创建PvP服务实例
func NewPvpService(enforcer *rbac.Enforcer, cfg iface.GameConfig) PvpService {
	return &pvpService{enforcer: enforcer, cfg: cfg}
}

// preComputedAttrs holds pre-computed player attributes read under RLock
type preComputedAttrs struct {
	attack     int64
	critRate   float64
	critDamage float64
}

// readAttrs reads player attributes under RLock for safe concurrent access
func readAttrs(p *model.Player) preComputedAttrs {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return preComputedAttrs{
		attack:     p.CalcAttack(),
		critRate:   p.CalcCritRate(),
		critDamage: p.CalcCritDamage(),
	}
}

// readDefAttrs reads defense and dodge rate under RLock
func readDefAttrs(p *model.Player) (def int64, dodgeRate float64) {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return p.CalcDefense(), p.CalcDodgeRate()
}

// calcPvpDamage 计算PvP伤害值（修复：使用预计算属性值，在RLock内读取）
// 流程：基础攻击力 -> 技能倍率 -> 暴击判定 -> 防御减伤 -> 随机波动
func (s *pvpService) calcPvpDamage(attrs preComputedAttrs, targetDef int64, skillID int32) (damage int64, isCrit bool) {
	// 基础攻击力
	attack := attrs.attack

	// 技能倍率
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

	// 防御减伤
	damage = damage - targetDef
	if damage < 1 {
		damage = 1
	}

	return
}

// checkPvpDodge 闪避判定（使用攻击者的闪避率，PvP中双方都有闪避可能）
func checkPvpDodge(dodgeRate float64) bool {
	return rand.Float64() < dodgeRate
}

// applyPvpDamage 对目标施加PvP伤害，返回目标剩余HP
func applyPvpDamage(target *model.Player, damage int64) (currHp int64, isDead bool) {
	target.Hp -= damage
	currHp = target.Hp
	isDead = target.Hp <= 0
	return
}

// Attack PvP攻击业务逻辑（修复：使用真实伤害计算，含防御、闪避、暴击）：
//  1. 校验攻击者和目标是否合法
//  2. 校验攻击者是否存活
//  3. 校验目标是否处于无敌保护状态
//  4. 闪避判定（使用目标闪避率）
//  5. 计算PvP伤害（含技能倍率、暴击、防御减伤）
//  6. 扣减目标血量
//  7. 若目标死亡：
//     a. 攻击者掠夺目标金币（掠夺量 = min(目标金币, 目标金币 * pvpPenalty)）
//     b. 攻击者获得荣誉值奖励
//     c. 攻击者杀戮值+1
//     d. 目标金币扣除被掠夺量（不低于0）
//     e. 目标满血复活、传送出地下城、获得30秒无敌保护
//  8. 杀戮值达到红名阈值时生成播报内容
func (s *pvpService) Attack(ctx context.Context, attacker *model.Player, target *model.Player, skillID int32, pvpPenalty float64, pvpHonor int32, redNameThreshold int32) (*PvpResult, *errors.GameError) {
	// 校验攻击者和目标是否合法
	if attacker == nil || target == nil {
		return nil, errors.ErrTargetNotFound
	}

	// 校验攻击者是否存活
	attacker.Mu().RLock()
	attackerHp := attacker.Hp
	attacker.Mu().RUnlock()
	if attackerHp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 检查当前区域是否允许PvP（Casbin RBAC）
	if s.enforcer != nil {
		subject := fmt.Sprintf("player:%d", attacker.ID)
		if !s.enforcer.Check(subject, "pvp:zone", "attack") {
			return nil, errors.ErrNotEnemy
		}
	}

	// 检查目标是否处于无敌保护状态
	if target.IsInvincible() {
		return nil, errors.ErrTargetInvincible
	}

	// 读取目标防御和闪避率（在RLock内读取）
	targetDef, targetDodgeRate := readDefAttrs(target)

	// 闪避判定
	if checkPvpDodge(targetDodgeRate) {
		return &PvpResult{
			AttackerID: attacker.ID,
			TargetID:   target.ID,
			Damage:     0,
			IsDodge:    true,
		}, nil
	}

	// 计算PvP伤害（预计算攻击者属性）
	attackerAttrs := readAttrs(attacker)
	damage, isCrit := s.calcPvpDamage(attackerAttrs, targetDef, skillID)

	// 按ID排序加锁，防止两个玩家互攻时ABBA死锁
	first, second := attacker, target
	if first.ID > second.ID {
		first, second = second, first
	}
	first.Mu().Lock()
	second.Mu().Lock()

	// 对目标扣减血量
	_, isDead := applyPvpDamage(target, damage)
	result := &PvpResult{
		AttackerID: attacker.ID,
		TargetID:   target.ID,
		Damage:     damage,
		IsDead:     isDead,
		IsCrit:     isCrit,
	}

	if isDead {
		// 攻击者获得奖励：金币掠夺 + 荣誉值 + 杀戮值+1
		goldGain := int64(float64(target.Gold) * pvpPenalty)
		if goldGain > target.Gold {
			goldGain = target.Gold // 防止负金币
		}
		if goldGain < 0 {
			goldGain = 0
		}
		attacker.Gold += goldGain
		attacker.Honor += pvpHonor
		attacker.KillValue++
		isRedName := attacker.KillValue >= redNameThreshold
		attackerName := attacker.Name

		result.GoldGain = goldGain
		result.HonorGain = pvpHonor

		// 目标玩家惩罚
		target.Gold -= goldGain
		if target.Gold < 0 {
			target.Gold = 0
		}
		target.Hp = target.MaxHp
		target.Layer = 0
		target.InvincibleUntil = time.Now().Add(time.Duration(s.cfg.InvincibleSec) * time.Second)

		second.Mu().Unlock()
		first.Mu().Unlock()

		// 红名检查
		if isRedName {
			result.IsRedName = true
			result.RedMsg = fmt.Sprintf("%s 已成为红名玩家！", attackerName)
		}
	} else {
		second.Mu().Unlock()
		first.Mu().Unlock()
	}

	logger.TInfo(ctx, "PvP攻击", "attacker_id", attacker.ID, "target_id", target.ID,
		"damage", damage, "is_dead", isDead, "is_crit", isCrit)
	return result, nil
}

// GetRedNameList 获取红名玩家列表：
//
//	遍历在线玩家，找出杀戮值达到红名阈值的玩家
//	赏金金额 = 杀戮值 * 1000金币
func (s *pvpService) GetRedNameList(ctx context.Context, players map[uint64]*model.Player, redNameThreshold int32) []*RedNameInfo {
	var list []*RedNameInfo

	for _, p := range players {
		p.Mu().RLock()
		killValue := p.KillValue
		online := p.Online
		name := p.Name
		id := p.ID
		p.Mu().RUnlock()

		if killValue >= redNameThreshold && online {
			list = append(list, &RedNameInfo{
				PlayerID:  id,
				Name:      name,
				KillValue: killValue,
				Bounty:    int64(killValue) * 1000,
			})
		}
	}

	return list
}

// BountyHunt 悬赏追杀业务逻辑（修复：实际执行战斗，对目标造成伤害）：
//  1. 校验悬赏者不是红名玩家
//  2. 校验目标是红名玩家
//  3. 对红名目标造成额外伤害（1.5倍）
//  4. 实际扣减目标血量
//  5. 击杀后获得额外金币和荣誉奖励，目标复活+无敌保护
func (s *pvpService) BountyHunt(ctx context.Context, hunter *model.Player, target *model.Player, redNameThreshold int32) (*BountyResult, *errors.GameError) {
	if hunter == nil || target == nil {
		return nil, errors.ErrTargetNotFound
	}

	// 校验悬赏者是否为红名（红名玩家无法悬赏他人）
	hunter.Mu().RLock()
	killValue := hunter.KillValue
	hunter.Mu().RUnlock()

	if killValue >= redNameThreshold {
		return nil, errors.ErrSelfRedName
	}

	// 读取目标杀戮值和防御（在RLock内读取）
	target.Mu().RLock()
	targetKillValue := target.KillValue
	targetID := target.ID
	target.Mu().RUnlock()
	targetDef, _ := readDefAttrs(target)

	// 校验目标是红名
	if targetKillValue < redNameThreshold {
		return nil, errors.ErrNotEnemy
	}

	// 检查目标是否处于无敌保护状态
	if target.IsInvincible() {
		return nil, errors.ErrTargetInvincible
	}

	// 对红名目标造成1.5倍伤害（使用普攻伤害 * 1.5）
	hunterAttrs := readAttrs(hunter)
	baseDamage, isCrit := s.calcPvpDamage(hunterAttrs, targetDef, 0)
	damage := int64(float64(baseDamage) * 1.5)

	// 按ID排序加锁
	first, second := hunter, target
	if first.ID > second.ID {
		first, second = second, first
	}
	first.Mu().Lock()
	second.Mu().Lock()

	// 对目标扣减血量
	target.Hp -= damage
	isDead := target.Hp <= 0

	result := &BountyResult{
		TargetID: targetID,
		Damage:   damage,
		IsDead:   isDead,
		IsCrit:   isCrit,
	}

	if isDead {
		// 悬赏击杀获得额外奖励
		goldGain := int64(targetKillValue) * s.cfg.BountyGoldPerKill
		honorGain := s.cfg.BountyHonorGain

		hunter.Gold += goldGain
		hunter.Honor += honorGain
		hunter.KillValue-- // 悬赏击杀减少杀戮值（正义行为）

		result.GoldGain = goldGain
		result.HonorGain = honorGain

		// 目标复活+无敌保护
		target.Hp = target.MaxHp
		target.Layer = 0
		target.InvincibleUntil = time.Now().Add(time.Duration(s.cfg.InvincibleSec) * time.Second)

		// 目标杀戮值减少
		target.KillValue -= 2
		if target.KillValue < 0 {
			target.KillValue = 0
		}
	}

	second.Mu().Unlock()
	first.Mu().Unlock()

	logger.TInfo(ctx, "悬赏追杀", "hunter_id", hunter.ID, "target_id", targetID,
		"damage", damage, "is_dead", isDead, "gold_gain", result.GoldGain)
	return result, nil
}

// Revenge 复仇业务逻辑（修复：实际执行战斗，对仇人造成伤害）：
//  1. 校验攻击者和目标合法性
//  2. 校验攻击者存活
//  3. 校验目标不处于无敌状态
//  4. 计算伤害并扣减目标血量
func (s *pvpService) Revenge(ctx context.Context, attacker *model.Player, target *model.Player, skillID int32) (*PvpResult, *errors.GameError) {
	if attacker == nil || target == nil {
		return nil, errors.ErrTargetNotFound
	}

	// 校验攻击者存活
	attacker.Mu().RLock()
	attackerHp := attacker.Hp
	attacker.Mu().RUnlock()
	if attackerHp <= 0 {
		return nil, errors.ErrSelfDead
	}

	// 校验目标不处于无敌状态
	if target.IsInvincible() {
		return nil, errors.ErrTargetInvincible
	}

	// 计算伤害（在RLock内预计算属性）
	targetDef, targetDodgeRate := readDefAttrs(target)

	// 闪避判定
	if checkPvpDodge(targetDodgeRate) {
		return &PvpResult{
			AttackerID: attacker.ID,
			TargetID:   target.ID,
			Damage:     0,
			IsDodge:    true,
		}, nil
	}

	attackerAttrs := readAttrs(attacker)
	damage, isCrit := s.calcPvpDamage(attackerAttrs, targetDef, skillID)
	// 复仇伤害加成20%
	damage = int64(float64(damage) * 1.2)

	// 按ID排序加锁
	first, second := attacker, target
	if first.ID > second.ID {
		first, second = second, first
	}
	first.Mu().Lock()
	second.Mu().Lock()

	_, isDead := applyPvpDamage(target, damage)
	result := &PvpResult{
		AttackerID: attacker.ID,
		TargetID:   target.ID,
		Damage:     damage,
		IsDead:     isDead,
		IsCrit:     isCrit,
	}

	if isDead {
		// 复仇击杀：获得少量金币，不影响杀戮值
		goldGain := target.Gold / 10
		if goldGain < 0 {
			goldGain = 0
		}
		attacker.Gold += goldGain
		result.GoldGain = goldGain
		result.HonorGain = 5 // 复仇击杀获得少量荣誉

		// 目标复活+无敌保护
		target.Gold -= goldGain
		if target.Gold < 0 {
			target.Gold = 0
		}
		target.Hp = target.MaxHp
		target.Layer = 0
		target.InvincibleUntil = time.Now().Add(time.Duration(s.cfg.InvincibleSec) * time.Second)
	}

	second.Mu().Unlock()
	first.Mu().Unlock()

	logger.TInfo(ctx, "复仇攻击", "attacker_id", attacker.ID, "target_id", target.ID,
		"damage", damage, "is_dead", isDead)
	return result, nil
}
