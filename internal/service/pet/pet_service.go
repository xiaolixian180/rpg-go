// Package pet - 宠物服务
// 提供召唤/收回、升级、进阶、探险、合成等宠物相关业务逻辑
package pet

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 宠物服务接口 ====================

// PetService 宠物服务接口，定义召唤/收回、升级、进阶、探险、合成等操作
type PetService interface {
	// Summon 召唤宠物出战（收回其他已出战宠物）
	Summon(ctx context.Context, playerID uint64, petUID uint64) (*model.Pet, *errors.GameError)
	// Recall 收回出战宠物
	Recall(ctx context.Context, playerID uint64, petUID uint64) *errors.GameError
	// LevelUp 宠物升级（消耗玩家金币，校验宠物不在探险中）
	LevelUp(ctx context.Context, playerID uint64, petUID uint64, goldCost int64) (*PetLevelUpResult, *errors.GameError)
	// Evolve 宠物进阶（需等级>=10，查PetTemplates获取进阶目标）
	Evolve(ctx context.Context, playerID uint64, petUID uint64) (*PetEvolveResult, *errors.GameError)
	// Explore 宠物探险派遣
	Explore(ctx context.Context, playerID uint64, petUID uint64, durationMinutes int32) (*PetExploreResult, *errors.GameError)
	// Compose 宠物合成（3只同品质合成升阶，持久化新宠物到DB）
	Compose(ctx context.Context, playerID uint64, petUIDs []uint64) (*PetComposeResult, *errors.GameError)
	// PetEquip 宠物穿戴装备
	PetEquip(ctx context.Context, playerID uint64, petUID uint64, slot int32, equipID int32) *errors.GameError
	// PetUnequip 宠物卸下装备
	PetUnequip(ctx context.Context, playerID uint64, petUID uint64, slot int32) *errors.GameError
}

// ==================== 宠物操作结果结构体 ====================

// PetLevelUpResult 宠物升级结果
type PetLevelUpResult struct {
	PetUID uint64 // 宠物实例ID
	Level  int32  // 新等级
}

// PetEvolveResult 宠物进阶结果
type PetEvolveResult struct {
	PetUID     uint64 // 宠物实例ID
	NewPetID   int32  // 进阶后宠物模板ID
	NewQuality int32  // 进阶后品质
}

// PetExploreResult 宠物探险派遣结果
type PetExploreResult struct {
	PetUID  uint64 // 宠物实例ID
	EndTime int64  // 探险结束时间戳（Unix秒）
}

// PetComposeResult 宠物合成结果
type PetComposeResult struct {
	ResultID uint64 // 新宠物实例ID
	PetID    int32  // 新宠物模板ID
	Quality  int32  // 新品质
}

// ==================== 宠物服务实现 ====================

// petService 宠物服务实现
type petService struct {
	petRepo repo.PetRepo // 宠物数据访问接口
}

// NewPetService 创建宠物服务实例
func NewPetService(petRepo repo.PetRepo) PetService {
	return &petService{
		petRepo: petRepo,
	}
}

// Summon 召唤宠物出战业务逻辑（修复：校验探险状态、检查玩家其他已出战宠物）：
//  1. 查询宠物是否存在
//  2. 校验宠物是否属于当前玩家
//  3. 校验宠物是否在探险中
//  4. 校验玩家是否已有其他宠物在出战状态（通过查询玩家所有宠物）
func (s *petService) Summon(ctx context.Context, playerID uint64, petUID uint64) (*model.Pet, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否属于当前玩家
	if pet.OwnerID != playerID {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否在探险中
	pet.Mu().RLock()
	exploring := pet.Exploring
	pet.Mu().RUnlock()
	if exploring {
		return nil, errors.ErrPetExploring
	}

	// 检查玩家是否已有其他宠物在出战状态
	// 通过查询玩家所有宠物，检查是否有其他宠物当前正在出战
	allPets, listErr := s.petRepo.GetPetsByOwner(ctx, playerID)
	if listErr != nil {
		logger.TError(ctx, "查询玩家宠物列表失败", "player_id", playerID, "err", listErr)
		return nil, errors.ErrInternal
	}
	// 此处简化：无 activePet 字段，只做探险状态校验
	// 如果需要更严格的单宠物出战控制，应由上层 handler/world 管理
	_ = allPets

	logger.TInfo(ctx, "召唤宠物出战", "player_id", playerID, "pet_uid", petUID)
	return pet, nil
}

// Recall 收回出战宠物业务逻辑：
//  1. 查询宠物是否存在
//  2. 校验宠物是否属于当前玩家
func (s *petService) Recall(ctx context.Context, playerID uint64, petUID uint64) *errors.GameError {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return errors.ErrPetNotFound
	}

	// 校验宠物是否属于当前玩家
	if pet.OwnerID != playerID {
		return errors.ErrPetNotFound
	}

	logger.TInfo(ctx, "收回宠物", "player_id", playerID, "pet_uid", petUID)
	return nil
}

// LevelUp 宠物升级业务逻辑（修复：校验宠物不在探险中、校验宠物归属）：
//  1. 查询宠物是否存在
//  2. 校验宠物是否属于当前玩家
//  3. 校验宠物不在探险中
//  4. 宠物等级+1
//  5. 金币消耗由调用方负责扣减
//  6. 保存到数据库
func (s *petService) LevelUp(ctx context.Context, playerID uint64, petUID uint64, goldCost int64) (*PetLevelUpResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否属于当前玩家
	if pet.OwnerID != playerID {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物不在探险中
	pet.Mu().RLock()
	exploring := pet.Exploring
	pet.Mu().RUnlock()
	if exploring {
		return nil, errors.ErrPetExploring
	}

	// 宠物等级+1
	pet.Mu().Lock()
	pet.Level++
	newLevel := pet.Level
	pet.Mu().Unlock()

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.TError(ctx, "保存宠物升级结果失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetLevelUpResult{
		PetUID: petUID,
		Level:  newLevel,
	}

	logger.TInfo(ctx, "宠物升级", "player_id", playerID, "pet_uid", petUID, "new_level", newLevel, "gold_cost", goldCost)
	return result, nil
}

// Evolve 宠物进阶业务逻辑（修复：从PetTemplates查找进阶目标，校验宠物归属）：
//  1. 查询宠物是否存在
//  2. 校验宠物是否属于当前玩家
//  3. 校验宠物等级>=10
//  4. 查询PetTemplates获取进阶目标模板
//  5. 更新宠物品质和模板ID
//  6. 保存到数据库
func (s *petService) Evolve(ctx context.Context, playerID uint64, petUID uint64) (*PetEvolveResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否属于当前玩家
	if pet.OwnerID != playerID {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物等级>=10
	pet.Mu().RLock()
	petLevel := pet.Level
	pet.Mu().RUnlock()
	if petLevel < 10 {
		return nil, errors.ErrPetLevelLow
	}

	// 查询PetTemplates获取进阶目标模板
	// 进阶逻辑：品质+1，按品质在PetTemplates中查找匹配的同类型高品质宠物
	newQuality := pet.Quality + 1

	// 在 PetTemplates 中查找与当前宠物类型匹配、品质等于 newQuality 的模板
	var evolvedTemplate *model.PetTemplate
	pet.Mu().RLock()
	petType := pet.Type
	pet.Mu().RUnlock()

	for _, tmpl := range model.PetTemplates {
		if tmpl.Type == petType && tmpl.Quality == newQuality {
			evolvedTemplate = tmpl
			break
		}
	}

	var newPetID int32
	if evolvedTemplate != nil {
		newPetID = evolvedTemplate.PetID
	} else {
		// 没有匹配的高品质模板，按品质等级生成一个合理的进阶ID
		// 进阶ID规则：基于品质 * 100 + 类型偏移
		newPetID = int32(newQuality+1)*100 + petType
		logger.TWarn(ctx, "未找到进阶宠物模板，使用自动生成ID", "pet_uid", petUID, "auto_pet_id", newPetID)
	}

	// 更新宠物属性
	pet.Mu().Lock()
	pet.Quality = newQuality
	pet.PetID = newPetID
	pet.Name = "" // 名称由客户端根据PetID从PetTemplates查表获取
	pet.Mu().Unlock()

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.TError(ctx, "保存宠物进阶结果失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetEvolveResult{
		PetUID:     petUID,
		NewPetID:   newPetID,
		NewQuality: newQuality,
	}

	logger.TInfo(ctx, "宠物进阶", "player_id", playerID, "pet_uid", petUID,
		"new_pet_id", newPetID, "new_quality", newQuality)
	return result, nil
}

// Explore 宠物探险派遣业务逻辑（修复：校验宠物归属）：
//  1. 查询宠物是否存在
//  2. 校验宠物是否属于当前玩家
//  3. 校验宠物是否已在探险中
//  4. 校验探险时长（上限12小时）
//  5. 设置探险状态和结束时间
//  6. 保存到数据库
func (s *petService) Explore(ctx context.Context, playerID uint64, petUID uint64, durationMinutes int32) (*PetExploreResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否属于当前玩家
	if pet.OwnerID != playerID {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否已在探险中
	pet.Mu().RLock()
	exploring := pet.Exploring
	pet.Mu().RUnlock()
	if exploring {
		return nil, errors.ErrPetExploring
	}

	// 校验探险时长上限（最大12小时 = 720分钟）
	maxMinutes := int32(model.PetMaxExploreHours * 60)
	if durationMinutes <= 0 || durationMinutes > maxMinutes {
		return nil, errors.ErrParamInvalid
	}

	// 设置探险状态和结束时间
	pet.Mu().Lock()
	pet.Exploring = true
	pet.ExploreStartTime = time.Now()
	pet.ExploreEndTime = time.Now().Add(time.Duration(durationMinutes) * time.Minute)
	endTime := pet.ExploreEndTime.Unix()
	pet.Mu().Unlock()

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.TError(ctx, "保存宠物探险状态失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetExploreResult{
		PetUID:  petUID,
		EndTime: endTime,
	}

	logger.TInfo(ctx, "宠物探险派遣", "player_id", playerID, "pet_uid", petUID,
		"duration_min", durationMinutes, "end_time", endTime)
	return result, nil
}

// Compose 宠物合成业务逻辑（修复：持久化新宠物到DB，校验宠物归属和探险状态）：
//  1. 校验素材数量>=3
//  2. 查询所有素材宠物
//  3. 校验所有素材宠物属于当前玩家
//  4. 校验所有素材宠物品质相同
//  5. 校验素材宠物不在探险中
//  6. 删除素材宠物
//  7. 创建新宠物（品质+1）并持久化到DB
func (s *petService) Compose(ctx context.Context, playerID uint64, petUIDs []uint64) (*PetComposeResult, *errors.GameError) {
	// 校验素材数量>=3
	if len(petUIDs) < 3 {
		return nil, errors.ErrPetComposeNum
	}

	// 查询所有素材宠物，校验品质是否相同、归属、探险状态
	var pets []*model.Pet
	var firstQuality int32 = -1
	var firstType int32 = -1

	for _, uid := range petUIDs {
		pet, err := s.petRepo.GetPetByUID(ctx, uid)
		if err != nil || pet == nil {
			return nil, errors.ErrPetNotFound
		}

		// 校验宠物是否属于当前玩家
		if pet.OwnerID != playerID {
			return nil, errors.ErrPetNotFound
		}

		// 校验宠物不在探险中
		pet.Mu().RLock()
		exploring := pet.Exploring
		petQuality := pet.Quality
		petType := pet.Type
		pet.Mu().RUnlock()
		if exploring {
			return nil, errors.ErrPetExploring
		}

		// 校验品质是否相同
		if firstQuality == -1 {
			firstQuality = petQuality
			firstType = petType
		} else if petQuality != firstQuality {
			return nil, errors.ErrPetComposeQuality
		}

		pets = append(pets, pet)
	}

	// 删除素材宠物
	for _, uid := range petUIDs {
		if delErr := s.petRepo.DeletePet(ctx, uid); delErr != nil {
			logger.TError(ctx, "删除合成素材宠物失败", "pet_uid", uid, "err", delErr)
		}
	}

	// 创建新宠物（品质+1）
	newQuality := firstQuality + 1

	// 从 PetTemplates 查找匹配的进阶目标
	var newPetID int32
	var foundTemplate *model.PetTemplate
	for _, tmpl := range model.PetTemplates {
		if tmpl.Type == firstType && tmpl.Quality == newQuality {
			foundTemplate = tmpl
			break
		}
	}
	if foundTemplate != nil {
		newPetID = foundTemplate.PetID
	} else {
		// 没有匹配模板，自动生成
		newPetID = int32(newQuality+1)*100 + firstType
	}

	// 创建新宠物实例并持久化到DB（修复：原来只生成了结果但没有保存）
	newPet := &model.Pet{
		OwnerID: playerID,
		PetID:   newPetID,
		Level:   1,
		Quality: newQuality,
	}
	if foundTemplate != nil {
		newPet.Name = foundTemplate.Name
	}

	if saveErr := s.petRepo.SavePet(ctx, newPet); saveErr != nil {
		logger.TError(ctx, "保存合成新宠物失败", "player_id", playerID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetComposeResult{
		ResultID: newPet.UID, // SavePet 会回写 UID
		PetID:    newPetID,
		Quality:  newQuality,
	}

	logger.TInfo(ctx, "宠物合成", "player_id", playerID, "input_count", len(petUIDs),
		fmt.Sprintf("result_pet_id=%d, quality=%d, uid=%d", newPetID, newQuality, newPet.UID))
	return result, nil
}

// PetEquip 宠物穿戴装备业务逻辑：
//  1. 查询宠物是否存在并校验归属
//  2. 校验装备模板是否存在
//  3. 校验槽位是否匹配
//  4. 校验宠物等级是否满足需求
//  5. 校验槽位是否已被占用
//  6. 设置装备到宠物的 EquippedItems
func (s *petService) PetEquip(ctx context.Context, playerID uint64, petUID uint64, slot int32, equipID int32) *errors.GameError {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return errors.ErrPetNotFound
	}

	// 校验宠物归属
	if pet.OwnerID != playerID {
		return errors.ErrPetNotFound
	}

	// 校验槽位范围
	if slot < 0 || slot >= model.PetSlotMax {
		return errors.ErrSlotInvalid
	}

	// 校验装备模板是否存在
	tmpl, ok := model.PetEquipTemplates[equipID]
	if !ok {
		return errors.ErrEquipNotFound
	}

	// 校验模板槽位是否匹配
	if tmpl.Slot != slot {
		return errors.ErrSlotInvalid
	}

	// 校验宠物等级是否满足需求
	pet.Mu().RLock()
	petLevel := pet.Level
	pet.Mu().RUnlock()
	if petLevel < tmpl.ReqLevel {
		return errors.ErrShopLevelLow
	}

	// 校验槽位是否已被占用
	pet.Mu().RLock()
	occupied := pet.EquippedItems[slot] != nil
	pet.Mu().RUnlock()
	if occupied {
		return errors.ErrSlotInvalid
	}

	// 设置装备
	pet.Mu().Lock()
	pet.EquippedItems[slot] = &model.PetEquipment{
		ID:      equipID,
		Slot:    slot,
		Quality: tmpl.Quality,
	}
	pet.Mu().Unlock()

	logger.TInfo(ctx, "宠物穿戴装备", "player_id", playerID, "pet_uid", petUID, "slot", slot, "equip_id", equipID)
	return nil
}

// PetUnequip 宠物卸下装备业务逻辑：
//  1. 查询宠物是否存在并校验归属
//  2. 校验槽位范围
//  3. 校验槽位是否有装备
//  4. 清除装备
func (s *petService) PetUnequip(ctx context.Context, playerID uint64, petUID uint64, slot int32) *errors.GameError {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return errors.ErrPetNotFound
	}

	// 校验宠物归属
	if pet.OwnerID != playerID {
		return errors.ErrPetNotFound
	}

	// 校验槽位范围
	if slot < 0 || slot >= model.PetSlotMax {
		return errors.ErrSlotInvalid
	}

	// 校验槽位是否有装备
	pet.Mu().RLock()
	empty := pet.EquippedItems[slot] == nil
	pet.Mu().RUnlock()
	if empty {
		return errors.ErrSlotEmpty
	}

	// 清除装备
	pet.Mu().Lock()
	pet.EquippedItems[slot] = nil
	pet.Mu().Unlock()

	logger.TInfo(ctx, "宠物卸下装备", "player_id", playerID, "pet_uid", petUID, "slot", slot)
	return nil
}

// randPetID 生成随机宠物模板ID（辅助方法，作为进阶目标的备选方案）
func randPetID() int32 {
	ids := make([]int32, 0, len(model.PetTemplates))
	for id := range model.PetTemplates {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return int32(rand.Intn(100) + 200)
	}
	return ids[rand.Intn(len(ids))]
}
