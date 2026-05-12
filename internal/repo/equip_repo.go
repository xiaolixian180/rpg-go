package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 装备数据访问实现 ====================

// equipRepo 是 EquipRepo 接口的具体实现，使用 gorm.DB 操作 MySQL。
type equipRepo struct {
	db *gorm.DB
}

// NewEquipRepo 创建 EquipRepo 实例。
func NewEquipRepo(db *database.DB) EquipRepo {
	return &equipRepo{db: db.DB}
}

// GetEquipBySlot 查询玩家指定槽位的装备。
// 利用 uk_player_slot 唯一索引保证查询效率。
// 若该槽位无装备，返回 nil 和 nil error。
func (r *equipRepo) GetEquipBySlot(ctx context.Context, playerID uint64, slot int32) (*model.Equipment, error) {
	var e model.PlayerEquipORM
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND slot = ?", playerID, slot).
		First(&e).Error
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query equip player=%d slot=%d: %w", playerID, slot, err)
	}
	// 将 ORM 模型转换为运行时模型
	return &model.Equipment{
		ID:              e.ID,
		PlayerID:        e.PlayerID,
		Slot:            e.Slot,
		EquipID:         e.EquipID,
		StrengthenLevel: e.StrengthenLevel,
		EnchantAttr:     e.EnchantAttr,
	}, nil
}

// SaveEquip 保存装备数据（新增或更新）。
// 使用 GORM Clauses(clause.OnConflict{...}) 实现 INSERT ... ON DUPLICATE KEY UPDATE，
// 依赖 uk_player_slot 唯一索引实现幂等写入。
func (r *equipRepo) SaveEquip(ctx context.Context, equip *model.Equipment) error {
	e := model.PlayerEquipORM{
		PlayerID:        equip.PlayerID,
		Slot:            equip.Slot,
		EquipID:         equip.EquipID,
		StrengthenLevel: equip.StrengthenLevel,
		EnchantAttr:     equip.EnchantAttr,
	}
	// 若有自增ID，设置到ORM模型
	if equip.ID > 0 {
		e.ID = equip.ID
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "player_id"}, {Name: "slot"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"equip_id", "strengthen_level", "enchant_attr",
			}),
		}).
		Create(&e).Error
	if err != nil {
		return fmt.Errorf("save equip player=%d slot=%d: %w", equip.PlayerID, equip.Slot, err)
	}

	// 回写自增主键
	equip.ID = e.ID
	return nil
}

// DeleteEquip 删除玩家指定槽位的装备记录。
// 根据 player_id + slot 条件删除，确保只删除该玩家该槽位的装备。
func (r *equipRepo) DeleteEquip(ctx context.Context, playerID uint64, slot int32) error {
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND slot = ?", playerID, slot).
		Delete(&model.PlayerEquipORM{}).Error
	if err != nil {
		return fmt.Errorf("delete equip player=%d slot=%d: %w", playerID, slot, err)
	}
	return nil
}

// GetEquipTemplates 批量查询装备模板。
// 根据模板 ID 列表从内存中查找（EquipTemplate 为配置数据，通常在内存中维护）。
// 当前返回空 map 占位，待接入配置表后补充实现。
func (r *equipRepo) GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error) {
	// 装备模板属于静态配置数据，通常从配置文件或内存缓存加载，
	// 此处预留接口，待配置系统接入后实现
	return make(map[int32]*model.EquipTemplate), nil
}

// GetForgeRecipe 查询锻造配方。
// 当前返回 nil 占位，待接入配置表后补充实现。
func (r *equipRepo) GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error) {
	// 锻造配方属于静态配置数据，待配置系统接入后实现
	return nil, nil
}
