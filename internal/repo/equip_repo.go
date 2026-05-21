package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
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
	return &model.Equipment{
		ID:              e.ID,
		PlayerID:        e.PlayerID,
		Slot:            e.Slot,
		EquipID:         e.EquipID,
		StrengthenLevel: e.StrengthenLevel,
		EnchantAttr:     e.EnchantAttr,
		Quality:         e.Quality,
	}, nil
}

// GetAllEquips 查询玩家所有装备
func (r *equipRepo) GetAllEquips(ctx context.Context, playerID uint64) ([]*model.Equipment, error) {
	var rows []model.PlayerEquipORM
	err := r.db.WithContext(ctx).
		Where("player_id = ?", playerID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("query all equips player=%d: %w", playerID, err)
	}
	equips := make([]*model.Equipment, 0, len(rows))
	for _, e := range rows {
		equips = append(equips, &model.Equipment{
			ID:              e.ID,
			PlayerID:        e.PlayerID,
			Slot:            e.Slot,
			EquipID:         e.EquipID,
			StrengthenLevel: e.StrengthenLevel,
			EnchantAttr:     e.EnchantAttr,
			Quality:         e.Quality,
		})
	}
	return equips, nil
}

// SaveEquip 保存装备数据（新增或更新）。
func (r *equipRepo) SaveEquip(ctx context.Context, equip *model.Equipment) error {
	e := model.PlayerEquipORM{
		PlayerID:        equip.PlayerID,
		Slot:            equip.Slot,
		EquipID:         equip.EquipID,
		StrengthenLevel: equip.StrengthenLevel,
		EnchantAttr:     equip.EnchantAttr,
		Quality:         equip.Quality,
	}
	if equip.ID > 0 {
		e.ID = equip.ID
	}

	err := r.db.WithContext(ctx).
		Clauses(gormClauseOnConflict(
			[]string{"player_id", "slot"},
			[]string{"equip_id", "strengthen_level", "enchant_attr", "quality"},
		)).
		Create(&e).Error
	if err != nil {
		return fmt.Errorf("save equip player=%d slot=%d: %w", equip.PlayerID, equip.Slot, err)
	}
	equip.ID = e.ID
	return nil
}

// DeleteEquip 删除玩家指定槽位的装备记录。
func (r *equipRepo) DeleteEquip(ctx context.Context, playerID uint64, slot int32) error {
	err := r.db.WithContext(ctx).
		Where("player_id = ? AND slot = ?", playerID, slot).
		Delete(&model.PlayerEquipORM{}).Error
	if err != nil {
		return fmt.Errorf("delete equip player=%d slot=%d: %w", playerID, slot, err)
	}
	return nil
}

// GetEquipTemplates 批量查询装备模板，从静态数据表加载。
func (r *equipRepo) GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error) {
	result := make(map[int32]*model.EquipTemplate, len(ids))
	for _, id := range ids {
		if t, ok := model.EquipTemplates[id]; ok {
			result[id] = t
		}
	}
	return result, nil
}

// GetForgeRecipe 查询锻造配方，从静态数据表加载。
func (r *equipRepo) GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error) {
	for _, r := range model.StaticForgeRecipes {
		if r.ID == recipeID {
			return r, nil
		}
	}
	return nil, nil
}
