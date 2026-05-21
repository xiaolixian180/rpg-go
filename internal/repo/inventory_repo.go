package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 背包数据访问实现 ====================

type inventoryRepo struct {
	db *gorm.DB
}

// NewInventoryRepo 创建背包数据仓库
func NewInventoryRepo(db *database.DB) InventoryRepo {
	return &inventoryRepo{db: db.DB}
}

func (r *inventoryRepo) GetItemCount(ctx context.Context, playerID uint64, itemID int32) (int32, error) {
	var count int32
	err := r.db.WithContext(ctx).
		Model(&model.PlayerInventoryORM{}).
		Select("COALESCE(SUM(count), 0)").
		Where("player_id = ? AND item_id = ?", playerID, itemID).
		Scan(&count).Error
	if err != nil {
		return 0, fmt.Errorf("get item count player=%d item=%d: %w", playerID, itemID, err)
	}
	return count, nil
}

func (r *inventoryRepo) AddItem(ctx context.Context, playerID uint64, itemID int32, count int32) error {
	inv := model.PlayerInventoryORM{
		PlayerID: playerID,
		ItemID:   itemID,
		Count:    count,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "player_id"}, {Name: "item_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"count": gorm.Expr("count + ?", count)}),
		}).
		Create(&inv).Error
	if err != nil {
		return fmt.Errorf("add item player=%d item=%d count=%d: %w", playerID, itemID, count, err)
	}
	return nil
}

func (r *inventoryRepo) RemoveItem(ctx context.Context, playerID uint64, itemID int32, count int32) (bool, error) {
	var success bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 在事务内执行 UPDATE
		result := tx.Model(&model.PlayerInventoryORM{}).
			Where("player_id = ? AND item_id = ? AND count >= ?", playerID, itemID, count).
			Update("count", gorm.Expr("count - ?", count))
		if result.Error != nil {
			return fmt.Errorf("remove item player=%d item=%d: %w", playerID, itemID, result.Error)
		}
		if result.RowsAffected == 0 {
			success = false
			return nil
		}
		// 在同一事务内清理 count <= 0 的行
		tx.Where("player_id = ? AND item_id = ? AND count <= 0", playerID, itemID).
			Delete(&model.PlayerInventoryORM{})
		success = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return success, nil
}

func (r *inventoryRepo) ListItems(ctx context.Context, playerID uint64) ([]*model.PlayerInventoryORM, error) {
	var rows []*model.PlayerInventoryORM
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND count > 0", playerID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list items player=%d: %w", playerID, err)
	}
	return rows, nil
}
