// repo 包提供数据访问层的具体实现。
// 接口定义在 repo.go 中，本文件实现 PlayerRepo 接口。
package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 玩家数据访问实现 ====================

// playerRepo 是 PlayerRepo 接口的具体实现，使用 gorm.DB 操作 MySQL。
type playerRepo struct {
	db *gorm.DB
}

// NewPlayerRepo 创建 PlayerRepo 实例。
func NewPlayerRepo(db *database.DB) PlayerRepo {
	return &playerRepo{db: db.DB}
}

// GetPlayerByID 根据玩家 ID 查询玩家基础属性。
// 仅查询 player 表，不关联 dungeon_progress。
func (r *playerRepo) GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error) {
	var p model.PlayerORM
	err := r.db.WithContext(ctx).First(&p, playerID).Error
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query player by id %d: %w", playerID, err)
	}
	// 将 ORM 模型转换为运行时模型
	return &model.Player{
		ID:         p.ID,
		Name:       p.Name,
		Class:      p.Class,
		Level:      p.Level,
		Exp:        p.Exp,
		Gold:       p.Gold,
		Honor:      p.Honor,
		KillValue:  p.KillValue,
		Str:        p.Str,
		Agi:        p.Agi,
		Int:        p.Int,
		Con:        p.Con,
		AttrPoints: p.AttrPoints,
	}, nil
}

// GetMaxLayer 查询玩家地下城通关最高层数。
// 从 dungeon_progress 表读取，若玩家无记录则返回 0。
func (r *playerRepo) GetMaxLayer(ctx context.Context, playerID uint64) (int32, error) {
	var maxLayer int32
	err := r.db.WithContext(ctx).
		Model(&model.DungeonProgressORM{}).
		Select("COALESCE(max_layer, 0)").
		Where("player_id = ?", playerID).
		Scan(&maxLayer).Error
	if err != nil {
		return 0, fmt.Errorf("query max_layer player=%d: %w", playerID, err)
	}
	return maxLayer, nil
}

// SavePlayer 保存玩家基础属性到 player 表。
// 仅更新 player 表，不涉及 dungeon_progress 表。
func (r *playerRepo) SavePlayer(ctx context.Context, p *model.Player) error {
	err := r.db.WithContext(ctx).
		Model(&model.PlayerORM{}).
		Where("id = ?", p.ID).
		Select("name", "class", "level", "exp", "gold", "honor", "kill_value",
			"str", "agi", "int", "con", "attr_points").
		Updates(map[string]interface{}{
			"name":        p.Name,
			"class":       p.Class,
			"level":       p.Level,
			"exp":         p.Exp,
			"gold":        p.Gold,
			"honor":       p.Honor,
			"kill_value":  p.KillValue,
			"str":         p.Str,
			"agi":         p.Agi,
			"int":         p.Int,
			"con":         p.Con,
			"attr_points": p.AttrPoints,
		}).Error
	if err != nil {
		return fmt.Errorf("save player %d: %w", p.ID, err)
	}
	return nil
}

// SaveMaxLayer 保存玩家地下城通关最高层数。
// 使用 GORM Clauses(clause.OnConflict{...}) 实现 INSERT ... ON DUPLICATE KEY UPDATE，
// 若玩家无进度记录则插入，已有记录则更新。
func (r *playerRepo) SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error {
	dp := model.DungeonProgressORM{
		PlayerID: playerID,
		MaxLayer: maxLayer,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "player_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"max_layer"}),
		}).
		Create(&dp).Error
	if err != nil {
		return fmt.Errorf("save max_layer player=%d layer=%d: %w", playerID, maxLayer, err)
	}
	return nil
}

// CreatePlayer 创建新玩家记录，插入 player 表并返回玩家ID。
// 新玩家默认等级1、满血、初始属性点5点。
func (r *playerRepo) CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error) {
	p := model.PlayerORM{
		ID:         id,
		Name:       name,
		Class:      class,
		Level:      1,
		Exp:        0,
		Gold:       0,
		Honor:      0,
		KillValue:  0,
		Str:        0,
		Agi:        0,
		Int:        0,
		Con:        0,
		AttrPoints: 5,
	}
	err := r.db.WithContext(ctx).Create(&p).Error
	if err != nil {
		return 0, fmt.Errorf("create player name=%s class=%d: %w", name, class, err)
	}
	return p.ID, nil
}
