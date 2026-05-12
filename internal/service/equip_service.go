// Package service - 装备服务
// 提备强化、附魔、穿戴/卸下、锻造合成等装备相关业务逻辑
package service

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
	// Strengthen 装备强化，含成功率计算（+7以上有失败概率）
	Strengthen(ctx context.Context, playerID uint64, slot int32) (*StrengthenResult, *errors.GameError)
	// Enchant 装备附魔，消耗材料为装备添加附加属性
	Enchant(ctx context.Context, playerID uint64, slot int32, materialID uint64) (*EnchantResult, *errors.GameError)
	// Wear 穿戴装备，将装备放入指定槽位
	Wear(ctx context.Context, playerID uint64, slot int32, equipID uint64) *errors.GameError
	// Unload 卸下装备，从指定槽位移除装备
	Unload(ctx context.Context, playerID uint64, slot int32) *errors.GameError
	// Forge 锻造合成，根据配方消耗材料合成新装备
	Forge(ctx context.Context, playerID uint64, recipeID uint64, materials []uint64) (*ForgeResult, *errors.GameError)
}

// ==================== 装备操作结果结构体 ====================

// StrengthenResult 强化结果
type StrengthenResult struct {
	Slot      int32  // 槽位
	NewLevel  int32  // 强化后等级（失败时不变）
	CostGold  int64  // 消耗金币
	IsSuccess bool   // 强化是否成功
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
	equipRepo repo.EquipRepo // 装备数据访问接口
}

// NewEquipService 创建装备服务实例
func NewEquipService(equipRepo repo.EquipRepo) EquipService {
	return &equipService{
		equipRepo: equipRepo,
	}
}

// Strengthen 装备强化业务逻辑：
//  1. 校验槽位是否合法
//  2. 查询当前槽位装备的强化等级
//  3. 计算强化费用 = (当前强化等级 + 1) * 100
//  4. 校验金币是否充足
//  5. 计算强化成功率：+7以下100%成功，+7以上失败率 = (当前等级-6)*10%
//  6. 成功则强化等级+1，失败则等级不变，金币均消耗
//  7. 保存强化结果到数据库
func (s *equipService) Strengthen(ctx context.Context, playerID uint64, slot int32) (*StrengthenResult, *errors.GameError) {
	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return nil, errors.ErrSlotInvalid
	}

	// 查询当前槽位装备的强化等级
	equip, err := s.equipRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return nil, errors.ErrSlotEmpty
	}

	strengthenLevel := equip.StrengthenLevel

	// 计算强化费用：费用随强化等级递增
	cost := int64((strengthenLevel + 1) * 100)

	result := &StrengthenResult{
		Slot:     slot,
		CostGold: cost,
	}

	// 注意：金币扣减由调用方（GameManager）负责，此处只计算费用和判定成功与否

	// +7以上有失败概率
	success := true
	if strengthenLevel >= 7 {
		failRate := float64(strengthenLevel - 6) * 0.1
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
			logger.Error("保存强化结果失败", "player_id", playerID, "slot", slot, "err", saveErr)
			return nil, errors.ErrInternal
		}
	}

	result.NewLevel = newLevel
	logger.Info("装备强化", "player_id", playerID, "slot", slot, "success", success, "new_level", newLevel)
	return result, nil
}

// Enchant 装备附魔业务逻辑：
//  1. 校验槽位是否合法
//  2. 查询指定槽位装备
//  3. 消耗附魔材料（简化：材料校验由调用方负责）
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
		logger.Error("保存附魔结果失败", "player_id", playerID, "slot", slot, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &EnchantResult{
		Slot:     slot,
		AttrName: attrName,
		AttrVal:  attrVal,
	}

	logger.Info("装备附魔", "player_id", playerID, "slot", slot, "attr", attrName, "val", attrVal)
	return result, nil
}

// Wear 穿戴装备业务逻辑：
//  1. 校验槽位是否合法
//  2. 创建装备实例并放入指定槽位
//  3. 保存到数据库
func (s *equipService) Wear(ctx context.Context, playerID uint64, slot int32, equipID uint64) *errors.GameError {
	// 校验槽位范围
	if slot < 0 || slot >= model.SlotMax {
		return errors.ErrSlotInvalid
	}

	// 创建装备实例
	equip := &model.Equipment{
		PlayerID: playerID,
		Slot:     slot,
		EquipID:  int32(equipID),
	}

	// 保存到数据库
	if err := s.equipRepo.SaveEquip(ctx, equip); err != nil {
		logger.Error("保存穿戴装备失败", "player_id", playerID, "slot", slot, "err", err)
		return errors.ErrInternal
	}

	logger.Info("穿戴装备", "player_id", playerID, "slot", slot, "equip_id", equipID)
	return nil
}

// Unload 卸下装备业务逻辑：
//  1. 校验槽位是否合法
//  2. 查询指定槽位装备
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
		logger.Error("删除卸下装备记录失败", "player_id", playerID, "slot", slot, "err", delErr)
		return errors.ErrInternal
	}

	logger.Info("卸下装备", "player_id", playerID, "slot", slot)
	return nil
}

// Forge 锻造合成业务逻辑：
//  1. 查询锻造配方
//  2. 校验材料是否满足配方要求
//  3. 查询产出装备模板信息
//  4. 创建产出装备实例
//  5. 保存到数据库
func (s *equipService) Forge(ctx context.Context, playerID uint64, recipeID uint64, materials []uint64) (*ForgeResult, *errors.GameError) {
	// 查询锻造配方
	recipe, err := s.equipRepo.GetForgeRecipe(ctx, recipeID)
	if err != nil || recipe == nil {
		return nil, errors.ErrForgeNotFound
	}

	// 校验材料数量是否满足配方要求
	if int32(len(materials)) < int32(len(recipe.Materials)) {
		return nil, errors.ErrMaterialLack
	}

	// 查询产出装备模板信息
	templates, templateErr := s.equipRepo.GetEquipTemplates(ctx, []int32{recipe.ResultID})
	if templateErr != nil {
		logger.Error("查询锻造产出装备模板失败", "result_id", recipe.ResultID, "err", templateErr)
		return nil, errors.ErrInternal
	}

	template, ok := templates[recipe.ResultID]
	if !ok {
		return nil, errors.ErrEquipNotFound
	}

	result := &ForgeResult{
		ResultID:   uint64(recipe.ResultID),
		ResultName: template.Name,
		Quality:    recipe.ResultQuality,
	}

	logger.Info("锻造合成", "player_id", playerID, "recipe_id", recipeID,
		"result_name", fmt.Sprintf("%s", template.Name), "quality", recipe.ResultQuality)
	return result, nil
}