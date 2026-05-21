// Package skill - 技能服务
// 提供技能升级、重置等技能相关业务逻辑
package skill

import (
	"context"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 技能服务接口 ====================

// SkillService 技能服务接口，定义技能升级和重置操作
type SkillService interface {
	// LevelUp 技能升级，消耗玩家技能点提升技能等级
	LevelUp(ctx context.Context, player *model.Player, skillID int32) (*SkillLevelUpResult, *errors.GameError)
	// Reset 技能重置，消耗金币将所有技能等级归零并返还技能点
	Reset(ctx context.Context, player *model.Player, goldCost int64) (*SkillResetResult, *errors.GameError)
}

// ==================== 技能操作结果结构体 ====================

// SkillLevelUpResult 技能升级结果
type SkillLevelUpResult struct {
	SkillID  int32 // 技能ID
	NewLevel int32 // 技能新等级
}

// SkillResetResult 技能重置结果
type SkillResetResult struct {
	RefundPoints int32 // 返还的技能点数
}

// ==================== 技能服务实现 ====================

// skillService 技能服务实现
type skillService struct {
	skillRepo repo.SkillRepo // 技能数据访问接口
}

// NewSkillService 创建技能服务实例
func NewSkillService(skillRepo repo.SkillRepo) SkillService {
	return &skillService{
		skillRepo: skillRepo,
	}
}

// LevelUp 技能升级业务逻辑：
//  1. 校验技能定义是否存在（使用 SkillDefs 全局表）
//  2. 校验技能是否属于当前职业
//  3. 查询技能当前等级
//  4. 校验技能是否已达到最大等级
//  5. 扣减玩家技能点
//  6. 技能等级+1，保存到数据库
func (s *skillService) LevelUp(ctx context.Context, player *model.Player, skillID int32) (*SkillLevelUpResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	// 读取玩家职业
	player.Mu().RLock()
	playerClass := player.Class
	playerID := player.ID
	player.Mu().RUnlock()

	// 校验技能定义是否存在（使用 SkillDefs 全局表）
	skillDef, ok := model.SkillDefs[skillID]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// 校验技能是否属于当前职业
	if skillDef.Class != playerClass {
		return nil, errors.ErrSkillNotFound
	}

	// 查询技能当前等级
	currentLevel, err := s.skillRepo.GetSkillLevel(ctx, playerID, skillID)
	if err != nil {
		logger.TError(ctx, "查询技能等级失败", "player_id", playerID, "skill_id", skillID, "err", err)
		return nil, errors.ErrInternal
	}

	// 校验技能是否已达到最大等级
	if currentLevel >= skillDef.MaxLevel {
		return nil, errors.ErrSkillMaxLevel
	}

	// 扣减玩家技能点（原子操作：检查+扣减在同一把锁内）
	player.Mu().Lock()
	if player.AttrPoints < 1 {
		player.Mu().Unlock()
		return nil, errors.ErrSkillPointsNotEnough
	}
	player.AttrPoints--
	player.Mu().Unlock()

	// 技能等级+1
	newLevel := currentLevel + 1

	// 保存到数据库
	if saveErr := s.skillRepo.SetSkillLevel(ctx, playerID, skillID, newLevel); saveErr != nil {
		logger.TError(ctx, "保存技能升级结果失败", "player_id", playerID, "skill_id", skillID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &SkillLevelUpResult{
		SkillID:  skillID,
		NewLevel: newLevel,
	}

	logger.TInfo(ctx, "技能升级", "player_id", playerID, "skill_id", skillID, "new_level", newLevel)
	return result, nil
}

// Reset 技能重置业务逻辑：
//  1. 扣减玩家金币
//  2. 查询玩家所有已学技能
//  3. 计算返还的技能点数（所有技能等级之和）
//  4. 删除所有技能记录
//  5. 将返还的技能点加回玩家
func (s *skillService) Reset(ctx context.Context, player *model.Player, goldCost int64) (*SkillResetResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	player.Mu().RLock()
	playerID := player.ID
	player.Mu().RUnlock()

	// 扣减金币（原子操作：检查+扣减在同一把锁内）
	player.Mu().Lock()
	if player.Gold < goldCost {
		player.Mu().Unlock()
		return nil, errors.ErrGoldNotEnough
	}
	player.Gold -= goldCost
	player.Mu().Unlock()

	// 查询玩家所有已学技能
	skills, err := s.skillRepo.GetAllSkills(ctx, playerID)
	if err != nil {
		logger.TError(ctx, "查询玩家技能列表失败", "player_id", playerID, "err", err)
		return nil, errors.ErrInternal
	}

	// 计算返还的技能点数（所有技能等级之和）
	var refundPoints int32
	for _, level := range skills {
		refundPoints += level
	}

	// 删除所有技能记录
	if delErr := s.skillRepo.DeleteAllSkills(ctx, playerID); delErr != nil {
		logger.TError(ctx, "删除玩家技能记录失败", "player_id", playerID, "err", delErr)
		return nil, errors.ErrInternal
	}

	// 将返还的技能点加回玩家
	player.Mu().Lock()
	player.AttrPoints += refundPoints
	player.Mu().Unlock()

	result := &SkillResetResult{
		RefundPoints: refundPoints,
	}

	logger.TInfo(ctx, "技能重置", "player_id", playerID, "refund_points", refundPoints, "gold_cost", goldCost)
	return result, nil
}
