// Package equip - 装备服务
// 提供强化、附魔、穿戴/卸下、锻造合成等装备相关业务逻辑
package equip

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 装备服务接口 ====================

// EquipService 装备服务接口，定义强化、附魔、穿戴/卸下、锻造合成等操作
type EquipService interface {
	// Strengthen 装备强化，含成功率计算（+7以上有失败概率），需传入player用于扣减金币
	Strengthen(ctx context.Context, player *model.Player, slot int32) (*StrengthenResult, *errors.GameError)
	// Enchant 装备附魔，消耗材料为装备添加附加属性
	Enchant(ctx context.Context, playerID uint64, slot int32, materialID uint64) (*EnchantResult, *errors.GameError)
	// Wear 穿戴装备，校验装备模板、等级需求、槽位占用
	Wear(ctx context.Context, player *model.Player, slot int32, equipID uint64) *errors.GameError
	// Unload 卸下装备，从指定槽位移除装备
	Unload(ctx context.Context, playerID uint64, slot int32) *errors.GameError
	// Forge 锻造合成，根据配方消耗材料合成新装备
	Forge(ctx context.Context, player *model.Player, recipeID uint64, materials []uint64) (*ForgeResult, *errors.GameError)
}

// ==================== 装备操作结果结构体 ====================

// StrengthenResult 强化结果
type StrengthenResult struct {
	Slot      int32 // 槽位
	NewLevel  int32 // 强化后等级（失败时不变）
	CostGold  int64 // 消耗金币
	IsSuccess bool  // 强化是否成功
}

// EnchantResult 附魔结果
type EnchantResult struct {
	Slot     int32  // 槽位
	AttrName string // 附魔属性名
	AttrVal  int32  // 附魔属性值
}

// ForgeResult 锻造结果
type ForgeResult struct {
	ResultID   uint64 // 产出装备实例ID
	ResultName string // 产出装备名称
	Quality    int32  // 产出品质
}

// ==================== 装备服务实现 ====================

// equipService 装备服务实现
type equipService struct {
	equipRepo     repo.EquipRepo     // 装备数据访问接口
	inventoryRepo repo.InventoryRepo // 背包数据访问接口（材料操作）
}

// NewEquipService 创建装备服务实例（需要 InventoryRepo 依赖用于材料消耗）
func NewEquipService(equipRepo repo.EquipRepo, inventoryRepo repo.InventoryRepo) EquipService {
	return &equipService{
		equipRepo:     equipRepo,
		inventoryRepo: inventoryRepo,
	}
}

// Strengthen 装备强化业务逻辑（修复：实际扣减玩家金币）：
//  1. 校验槽位是否合法
//  2. 查询当前槽位装备的强化等级
//  3. 计算强化费用 = (当前强化等级 + 1) * 100
//  4. 扣减玩家金币
//  5. 计算强化成功率：+7以下100%成功，+7以上失败率 = (当前等级-6)*10%
//  6. 成功则强化等级+1，失败则等级不变，金币均消耗
//  7. 保存强化结果到数据库
func (s *equipService) Strengthen(ctx context.Context, player *model.Player, slot int32) (*StrengthenResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return nil, errors.ErrSlotInvalid
	}

	playerID := player.ID

	// 查询当前槽位装备的强化等级
	equip, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return nil, errors.ErrSlotEmpty
	}

	strengthenLevel := equip.StrengthenLevel

	// 计算强化费用：费用随强化等级递增
	cost := int64((strengthenLevel + 1) * 100)

	// 扣减玩家金币（原子操作：检查+扣减在同一把锁内）
	player.Mu().Lock()
	if player.Gold < cost {
		player.Mu().Unlock()
		return nil, errors.ErrGoldNotEnough
	}
	player.Gold -= cost
	player.Mu().Unlock()

	result := &StrengthenResult{
		Slot:     slot,
		CostGold: cost,
	}

	// +7以上有失败概率
	success := true
	if strengthenLevel >= 7 {
		failRate := float64(strengthenLevel-6) * 0.1
		if rand.Float64() < failRate {
			success = false
		}
	}

	result.IsSuccess = success

	// 强化成功则等级+1并更新数据库
	newLevel := strengthenLevel
	if success {
		newLevel = strengthenLevel + 1
		equip.StrengthenLevel = newLevel
		if saveErr := s.equipRepo.SaveEquip(ctx, equip); saveErr != nil {
			logger.TError(ctx, "保存强化结果失败", "player_id", playerID, "slot", slot, "err", saveErr)
			return nil, errors.ErrInternal
		}
	}

	result.NewLevel = newLevel
	logger.TInfo(ctx, "装备强化", "player_id", playerID, "slot", slot, "success", success, "new_level", newLevel, "cost", cost)
	return result, nil
}

// Enchant 装备附魔业务逻辑（修复：实际消耗背包中的材料）：
//  1. 校验槽位是否合法
//  2. 查询指定槽位装备
//  3. 从背包中消耗附魔材料
//  4. 随机生成附魔属性（力量/敏捷/智力/体质随机选一个）
//  5. 附魔属性值 = rand(1~5)
//  6. 保存附魔结果到数据库
func (s *equipService) Enchant(ctx context.Context, playerID uint64, slot int32, materialID uint64) (*EnchantResult, *errors.GameError) {
	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return nil, errors.ErrSlotInvalid
	}

	// 查询指定槽位装备
	equip, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return nil, errors.ErrSlotEmpty
	}

	// 从背包中消耗附魔材料（修复：实际扣减材料）
	ok, removeErr := s.inventoryRepo.RemoveItem(ctx, playerID, int32(materialID), 1)
	if removeErr != nil {
		logger.TError(ctx, "消耗附魔材料失败", "player_id", playerID, "material_id", materialID, "err", removeErr)
		return nil, errors.ErrInternal
	}
	if !ok {
		return nil, errors.ErrMaterialLack
	}

	// 随机生成附魔属性名
	attrNames := []string{"力量", "敏捷", "智力", "体质"}
	attrIdx := rand.Intn(len(attrNames))
	attrName := attrNames[attrIdx]
	attrVal := int32(rand.Intn(5) + 1) // 1~5随机

	// 更新附魔属性
	enchantData := map[string]int32{attrName: attrVal}
	enchantJSON, _ := json.Marshal(enchantData)
	equip.EnchantAttr = string(enchantJSON)

	// 保存到数据库
	if saveErr := s.equipRepo.SaveEquip(ctx, equip); saveErr != nil {
		logger.TError(ctx, "保存附魔结果失败", "player_id", playerID, "slot", slot, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &EnchantResult{
		Slot:     slot,
		AttrName: attrName,
		AttrVal:  attrVal,
	}

	logger.TInfo(ctx, "装备附魔", "player_id", playerID, "slot", slot, "attr", attrName, "val", attrVal, "material_id", materialID)
	return result, nil
}

// Wear 穿戴装备业务逻辑（修复：校验装备模板、等级需求、槽位占用）：
//  1. 校验槽位是否合法
//  2. 校验装备模板是否存在
//  3. 校验模板槽位是否匹配
//  4. 校验玩家等级是否满足需求
//  5. 校验目标槽位是否已被占用
//  6. 创建装备实例并放入指定槽位
//  7. 保存到数据库
func (s *equipService) Wear(ctx context.Context, player *model.Player, slot int32, equipID uint64) *errors.GameError {
	if player == nil {
		return errors.ErrNotLogin
	}

	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return errors.ErrSlotInvalid
	}

	// 校验装备模板是否存在
	template, ok := model.EquipTemplates[int32(equipID)]
	if !ok {
		return errors.ErrEquipNotFound
	}

	// 校验模板槽位是否匹配
	if template.Slot != slot {
		return errors.ErrSlotInvalid
	}

	// 校验玩家等级是否满足需求
	player.Mu().RLock()
	playerLevel := player.Level
	playerID := player.ID
	player.Mu().RUnlock()

	if playerLevel < template.RequireLevel {
		return errors.ErrShopLevelLow
	}

	// 校验目标槽位是否已被占用
	existing, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil {
		logger.TError(ctx, "查询槽位装备失败", "player_id", playerID, "slot", slot, "err", err)
		return errors.ErrInternal
	}
	if existing != nil {
		return errors.ErrSlotInvalid // 槽位已被占用
	}

	// 创建装备实例
	equip := &model.Equipment{
		PlayerID: playerID,
		Slot:     slot,
		EquipID:  int32(equipID),
		Quality:  template.Quality,
	}

	// 保存到数据库
	if saveErr := s.equipRepo.SaveEquip(ctx, equip); saveErr != nil {
		logger.TError(ctx, "保存穿戴装备失败", "player_id", playerID, "slot", slot, "err", saveErr)
		return errors.ErrInternal
	}

	logger.TInfo(ctx, "穿戴装备", "player_id", playerID, "slot", slot, "equip_id", equipID, "equip_name", template.Name)
	return nil
}

// Unload 卸下装备业务逻辑：
//  1. 校验槽位是否合法
//  2. 查询指定槽位装备（校验槽位非空）
//  3. 从数据库删除该槽位的装备记录
func (s *equipService) Unload(ctx context.Context, playerID uint64, slot int32) *errors.GameError {
	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return errors.ErrSlotInvalid
	}

	// 查询指定槽位装备（校验槽位非空）
	equip, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return errors.ErrSlotEmpty
	}

	// 从数据库删除该槽位的装备记录
	if delErr := s.equipRepo.DeleteEquip(ctx, playerID, slot); delErr != nil {
		logger.TError(ctx, "删除卸下装备记录失败", "player_id", playerID, "slot", slot, "err", delErr)
		return errors.ErrInternal
	}

	logger.TInfo(ctx, "卸下装备", "player_id", playerID, "slot", slot)
	return nil
}

// Forge 锻造合成业务逻辑（修复：校验等级、消耗背包材料、扣减金币、创建装备实例）：
//  1. 查询锻造配方
//  2. 校验玩家等级是否满足配方需求
//  3. 从背包中校验并消耗配方所需的每种材料
//  4. 扣减锻造金币
//  5. 查询产出装备模板信息
//  6. 创建产出装备实例并保存到背包
func (s *equipService) Forge(ctx context.Context, player *model.Player, recipeID uint64, materials []uint64) (*ForgeResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	playerID := player.ID

	// 查询锻造配方
	recipe, err := s.equipRepo.GetForgeRecipe(ctx, recipeID)
	if err != nil || recipe == nil {
		return nil, errors.ErrForgeNotFound
	}

	// 校验玩家等级是否满足配方需求
	player.Mu().RLock()
	playerLevel := player.Level
	player.Mu().RUnlock()

	if playerLevel < recipe.RequireLevel {
		return nil, errors.ErrShopLevelLow
	}

	// 从背包中校验并消耗配方所需的每种材料
	for materialID, needCount := range recipe.Materials {
		haveCount, countErr := s.inventoryRepo.GetItemCount(ctx, playerID, int32(materialID))
		if countErr != nil {
			logger.TError(ctx, "查询材料数量失败", "player_id", playerID, "material_id", materialID, "err", countErr)
			return nil, errors.ErrInternal
		}
		if haveCount < needCount {
			return nil, errors.ErrMaterialLack
		}
	}

	// 扣减锻造金币
	player.Mu().Lock()
	if player.Gold < recipe.Cost {
		player.Mu().Unlock()
		return nil, errors.ErrGoldNotEnough
	}
	player.Gold -= recipe.Cost
	player.Mu().Unlock()

	// 消耗材料
	for materialID, needCount := range recipe.Materials {
		ok, removeErr := s.inventoryRepo.RemoveItem(ctx, playerID, int32(materialID), needCount)
		if removeErr != nil {
			logger.TError(ctx, "消耗锻造材料失败", "player_id", playerID, "material_id", materialID, "err", removeErr)
			return nil, errors.ErrInternal
		}
		if !ok {
			logger.TError(ctx, "消耗锻造材料失败，库存不足", "player_id", playerID, "material_id", materialID)
			return nil, errors.ErrMaterialLack
		}
	}

	// 查询产出装备模板信息
	template, ok := model.EquipTemplates[recipe.ResultID]
	if !ok {
		return nil, errors.ErrEquipNotFound
	}

	// 将产出装备添加到玩家背包
	if addErr := s.inventoryRepo.AddItem(ctx, playerID, recipe.ResultID, 1); addErr != nil {
		logger.TError(ctx, "添加锻造产物到背包失败", "player_id", playerID, "result_id", recipe.ResultID, "err", addErr)
		return nil, errors.ErrInternal
	}

	result := &ForgeResult{
		ResultID:   uint64(recipe.ResultID),
		ResultName: template.Name,
		Quality:    recipe.ResultQuality,
	}

	logger.TInfo(ctx, "锻造合成", "player_id", playerID, "recipe_id", recipeID,
		"result_name", template.Name, "quality", recipe.ResultQuality, "cost", recipe.Cost)
	return result, nil
}

// formatEquipInfo 格式化装备信息（辅助方法）
func formatEquipInfo(name string, quality int32, level int32) string {
	qualityName := "普通"
	if q, ok := model.QualityName[quality]; ok {
		qualityName = q
	}
	return fmt.Sprintf("%s(%s)+%d", name, qualityName, level)
}
