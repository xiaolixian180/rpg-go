// repo 包 - 技能数据访问实现
// 实现 SkillRepo 接口，使用 gorm.DB 操作 MySQL 中的 player_skill 表
package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 技能数据访问实现 ====================

// skillRepo 是 SkillRepo 接口的具体实现，使用 gorm.DB 操作 MySQL。
type skillRepo struct {
	db *gorm.DB
}

// NewSkillRepo 创建 SkillRepo 实例。
func NewSkillRepo(db *database.DB) SkillRepo {
	return &skillRepo{db: db.DB}
}

// GetSkillLevel 查询玩家指定技能的当前等级。
// 若玩家未学习该技能，返回 0 和 nil error。
func (r *skillRepo) GetSkillLevel(ctx context.Context, playerID uint64, skillID int32) (int32, error) {
	var s model.PlayerSkillORM
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND skill_id = ?", playerID, skillID).
		First(&s).Error
	if err != nil {
		if isNoRows(err) {
			// 技能未学习，返回等级 0
			return 0, nil
		}
		return 0, fmt.Errorf("query skill level player=%d skill=%d: %w", playerID, skillID, err)
	}
	return s.Level, nil
}

// SetSkillLevel 设置玩家指定技能的等级。
// 使用 GORM Clauses(clause.OnConflict{...}) 实现 INSERT ... ON DUPLICATE KEY UPDATE，
// 若玩家未学该技能则插入，已学则更新等级。
func (r *skillRepo) SetSkillLevel(ctx context.Context, playerID uint64, skillID int32, level int32) error {
	s := model.PlayerSkillORM{
		PlayerID: playerID,
		SkillID:  skillID,
		Level:    level,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "player_id"}, {Name: "skill_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"level"}),
		}).
		Create(&s).Error
	if err != nil {
		return fmt.Errorf("set skill level player=%d skill=%d level=%d: %w", playerID, skillID, level, err)
	}
	return nil
}

// GetAllSkills 查询玩家所有已学技能及等级。
// 返回 map[skillID]level，未学任何技能时返回空 map。
func (r *skillRepo) GetAllSkills(ctx context.Context, playerID uint64) (map[int32]int32, error) {
	var rows []model.PlayerSkillORM
	err := r.db.WithContext(ctx).
		Where("player_id = ?", playerID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query all skills player=%d: %w", playerID, err)
	}

	result := make(map[int32]int32, len(rows))
	for _, row := range rows {
		result[row.SkillID] = row.Level
	}
	return result, nil
}

// DeleteAllSkills 删除玩家所有技能记录（技能重置时使用）。
func (r *skillRepo) DeleteAllSkills(ctx context.Context, playerID uint64) error {
	err := r.db.WithContext(ctx).
		Where("player_id = ?", playerID).
		Delete(&model.PlayerSkillORM{}).Error
	if err != nil {
		return fmt.Errorf("delete all skills player=%d: %w", playerID, err)
	}
	return nil
}
