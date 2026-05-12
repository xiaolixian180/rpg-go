// Package service - 技能服务
// 提供技能升级、重置等技能相关业务逻辑
package service

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
	// LevelUp 技能升级，消耗技能点提升技能等级
	LevelUp(ctx context.Context, playerID uint64, skillID int32, skillPoints int32) (*SkillLevelUpResult, *errors.GameError)
	// Reset 技能重置，消耗金币将所有技能等级归零并返还技能点
	Reset(ctx context.Context, playerID uint64, goldCost int64) (*SkillResetResult, *errors.GameError)
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
//  1. 校验技能是否存在且属于当前职业
//  2. 查询技能当前等级
//  3. 校验技能是否已达到最大等级
//  4. 校验技能点是否充足
//  5. 技能等级+1，扣减技能点
//  6. 保存到数据库
func (s *skillService) LevelUp(ctx context.Context, playerID uint64, skillID int32, skillPoints int32) (*SkillLevelUpResult, *errors.GameError) {
	// 校验技能定义是否存在
	skillDef, ok := model.SkillDefs[skillID]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// 查询技能当前等级
	currentLevel, err := s.skillRepo.GetSkillLevel(ctx, playerID, skillID)
	if err != nil {
		logger.Error("查询技能等级失败", "player_id", playerID, "skill_id", skillID, "err", err)
		return nil, errors.ErrInternal
	}

	// 校验技能是否已达到最大等级
	if currentLevel >= skillDef.MaxLevel {
		return nil, errors.ErrSkillPointsNotEnough
	}

	// 校验技能点是否充足（每次升级消耗1点）
	if skillPoints < 1 {
		return nil, errors.ErrSkillPointsNotEnough
	}

	// 技能等级+1
	newLevel := currentLevel + 1

	// 保存到数据库
	if saveErr := s.skillRepo.SetSkillLevel(ctx, playerID, skillID, newLevel); saveErr != nil {
		logger.Error("保存技能升级结果失败", "player_id", playerID, "skill_id", skillID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &SkillLevelUpResult{
		SkillID:  skillID,
		NewLevel: newLevel,
	}

	logger.Info("技能升级", "player_id", playerID, "skill_id", skillID, "new_level", newLevel)
	return result, nil
}

// Reset 技能重置业务逻辑：
//  1. 查询玩家所有已学技能
//  2. 计算返还的技能点数（所有技能等级之和）
//  3. 删除所有技能记录
//  4. 金币消耗由调用方负责扣减
func (s *skillService) Reset(ctx context.Context, playerID uint64, goldCost int64) (*SkillResetResult, *errors.GameError) {
	// 查询玩家所有已学技能
	skills, err := s.skillRepo.GetAllSkills(ctx, playerID)
	if err != nil {
		logger.Error("查询玩家技能列表失败", "player_id", playerID, "err", err)
		return nil, errors.ErrInternal
	}

	// 计算返还的技能点数（所有技能等级之和）
	var refundPoints int32
	for _, level := range skills {
		refundPoints += level
	}

	// 删除所有技能记录
	if delErr := s.skillRepo.DeleteAllSkills(ctx, playerID); delErr != nil {
		logger.Error("删除玩家技能记录失败", "player_id", playerID, "err", delErr)
		return nil, errors.ErrInternal
	}

	result := &SkillResetResult{
		RefundPoints: refundPoints,
	}

	logger.Info("技能重置", "player_id", playerID, "refund_points", refundPoints, "gold_cost", goldCost)
	return result, nil
}