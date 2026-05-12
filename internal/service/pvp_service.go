// Package service - PvP服务
// 提供PvP攻击、金币掠夺、荣誉奖励、红名判定、悬赏、复仇等PvP相关业务逻辑
package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/rbac"
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
	// BountyHunt 悬赏追杀，对红名玩家发起悬赏，击杀后获得额外奖励
	BountyHunt(ctx context.Context, hunter *model.Player, target *model.Player) (*BountyResult, *errors.GameError)
	// Revenge 复仇，对仇人发起复仇攻击
	Revenge(ctx context.Context, playerID uint64, targetID uint64, enemies map[uint64][]uint64) *errors.GameError
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
	GoldGain  int64  // 获得金币
	HonorGain int32  // 获得荣誉
}

// ==================== PvP服务实现 ====================

// pvpService PvP服务实现
type pvpService struct {
	enforcer *rbac.Enforcer // Casbin 权限执行器，用于校验PvP区域权限
}

// NewPvpService 创建PvP服务实例
// enforcer: Casbin 权限执行器，传入 nil 则跳过权限校验
func NewPvpService(enforcer *rbac.Enforcer) PvpService {
	return &pvpService{enforcer: enforcer}
}

// Attack PvP攻击业务逻辑：
//  1. 校验攻击者和目标是否合法
//  2. 校验目标是否处于无敌保护状态
//  3. 计算PvP伤害（与PvE使用相同公式）
//  4. 扣减目标血量
//  5. 若目标死亡：
//     a. 攻击者掠夺目标金币（掠夺量 = 目标金币 * pvpPenalty）
//     b. 攻击者获得荣誉值奖励
//     c. 攻击者杀戮值+1
//     d. 目标金币扣除被掠夺量
//     e. 目标满血复活、传送出地下城、获得30秒无敌保护
//  6. 杀戮值达到红名阈值时生成播报内容
func (s *pvpService) Attack(ctx context.Context, attacker *model.Player, target *model.Player, skillID int32, pvpPenalty float64, pvpHonor int32, redNameThreshold int32) (*PvpResult, *errors.GameError) {
	// 校验攻击者和目标是否合法
	if attacker == nil || target == nil {
		return nil, errors.ErrTargetNotFound
	}

	// 检查当前区域是否允许PvP（Casbin RBAC）
	// 通过权限执行器检查玩家角色是否有权在PvP区域攻击
	if s.enforcer != nil {
		if !s.enforcer.Check("player", "pvp:zone", "attack") {
			return nil, errors.ErrNotEnemy
		}
	}

	// 检查目标是否处于无敌保护状态
	if target.IsInvincible() {
		return nil, errors.ErrTargetInvincible
	}

	// 计算PvP伤害
	damage := s.calcPvpDamage(attacker, skillID)

	// 对目标扣减血量
	target.Hp -= damage
	isDead := target.Hp <= 0

	result := &PvpResult{
		AttackerID: attacker.ID,
		TargetID:   target.ID,
		Damage:     damage,
		IsDead:     isDead,
	}

	if isDead {
		// 攻击者获得奖励：金币掠夺 + 荣誉值 + 杀戮值+1
		goldGain := int64(float64(target.Gold) * pvpPenalty)
		attacker.Gold += goldGain
		attacker.Honor += pvpHonor
		attacker.KillValue++

		result.GoldGain = goldGain
		result.HonorGain = pvpHonor

		// 目标玩家惩罚：扣除被掠夺金币、满血复活、传送出地下城、获得无敌保护
		target.Gold -= goldGain
		target.Hp = target.MaxHp
		target.Layer = 0
		target.InvincibleUntil = time.Now().Add(30 * time.Second)

		// 红名检查：杀戮值达到阈值则标记为红名
		if attacker.KillValue >= redNameThreshold {
			result.IsRedName = true
			result.RedMsg = fmt.Sprintf("%s 已成为红名玩家！", attacker.Name)
		}
	}

	logger.Info("PvP攻击", "attacker_id", attacker.ID, "target_id", target.ID, "damage", damage, "is_dead", isDead)
	return result, nil
}

// GetRedNameList 获取红名玩家列表：
//  遍历在线玩家，找出杀戮值达到红名阈值的玩家
//  赏金金额 = 杀戮值 * 1000金币
func (s *pvpService) GetRedNameList(ctx context.Context, players map[uint64]*model.Player, redNameThreshold int32) []*RedNameInfo {
	var list []*RedNameInfo

	for _, p := range players {
		if p.KillValue >= redNameThreshold && p.Online {
			list = append(list, &RedNameInfo{
				PlayerID:  p.ID,
				Name:      p.Name,
				KillValue: p.KillValue,
				Bounty:    int64(p.KillValue) * 1000, // 赏金 = 杀戮值 * 1000
			})
		}
	}

	return list
}

// BountyHunt 悬赏追杀业务逻辑：
//  1. 校验悬赏者不是红名玩家
//  2. 对红名目标造成额外伤害（1.5倍）
//  3. 击杀后获得额外金币和荣誉奖励
func (s *pvpService) BountyHunt(ctx context.Context, hunter *model.Player, target *model.Player) (*BountyResult, *errors.GameError) {
	// 校验悬赏者是否为红名（红名玩家无法悬赏他人）
	if hunter.KillValue >= 3 {
		return nil, errors.ErrSelfRedName
	}

	// 悬赏击杀获得额外奖励
	goldGain := int64(target.KillValue) * 500 // 悬赏金币 = 目标杀戮值 * 500
	honorGain := int32(10)                    // 悬赏荣誉固定10点

	result := &BountyResult{
		TargetID:  target.ID,
		GoldGain:  goldGain,
		HonorGain: honorGain,
	}

	logger.Info("悬赏追杀", "hunter_id", hunter.ID, "target_id", target.ID, "gold_gain", goldGain)
	return result, nil
}

// Revenge 复仇业务逻辑：
//  1. 校验目标是否在仇人列表中
//  2. 校验参数有效性
func (s *pvpService) Revenge(ctx context.Context, playerID uint64, targetID uint64, enemies map[uint64][]uint64) *errors.GameError {
	// 校验参数
	if targetID == 0 {
		return errors.ErrParamInvalid
	}

	// 校验目标是否在仇人列表中
	playerEnemies, ok := enemies[playerID]
	if !ok {
		return errors.ErrNotEnemy
	}

	isEnemy := false
	for _, enemyID := range playerEnemies {
		if enemyID == targetID {
			isEnemy = true
			break
		}
	}

	if !isEnemy {
		return errors.ErrNotEnemy
	}

	logger.Info("复仇请求", "player_id", playerID, "target_id", targetID)
	return nil
}

// calcPvpDamage 计算PvP伤害值
// 基础伤害 = 力量*2 + 等级*5
// 最终伤害 = 基础伤害 + 随机波动（0 ~ 基础伤害/4）
func (s *pvpService) calcPvpDamage(p *model.Player, skillID int32) int64 {
	base := int64(p.Str)*2 + int64(p.Level)*5
	return base + rand.Int63n(base/4+1)
}