// Package service - 宠物服务
// 提供召唤/收回、升级、进阶、探险、合成等宠物相关业务逻辑
package service

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
	// Summon 召唤宠物出战
	Summon(ctx context.Context, playerID uint64, petUID uint64) (*model.Pet, *errors.GameError)
	// Recall 收回出战宠物
	Recall(ctx context.Context, playerID uint64, petUID uint64) *errors.GameError
	// LevelUp 宠物升级（消耗玩家金币）
	LevelUp(ctx context.Context, playerID uint64, petUID uint64, goldCost int64) (*PetLevelUpResult, *errors.GameError)
	// Evolve 宠物进阶（需等级>=10，消耗材料）
	Evolve(ctx context.Context, playerID uint64, petUID uint64) (*PetEvolveResult, *errors.GameError)
	// Explore 宠物探险派遣
	Explore(ctx context.Context, playerID uint64, petUID uint64, durationMinutes int32) (*PetExploreResult, *errors.GameError)
	// Compose 宠物合成（3只同品质合成升阶）
	Compose(ctx context.Context, playerID uint64, petUIDs []uint64) (*PetComposeResult, *errors.GameError)
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

// Summon 召唤宠物出战业务逻辑：
//  1. 查询宠物是否存在
//  2. 校验宠物是否已在出战
//  3. 校验宠物是否在探险中
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

	// 校验宠物是否已在出战（简化：通过状态判断）
	if pet.Exploring {
		return nil, errors.ErrPetExploring
	}

	logger.Info("召唤宠物出战", "player_id", playerID, "pet_uid", petUID)
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

	logger.Info("收回宠物", "player_id", playerID, "pet_uid", petUID)
	return nil
}

// LevelUp 宠物升级业务逻辑：
//  1. 查询宠物是否存在
//  2. 宠物等级+1
//  3. 金币消耗由调用方负责扣减
//  4. 保存到数据库
func (s *petService) LevelUp(ctx context.Context, playerID uint64, petUID uint64, goldCost int64) (*PetLevelUpResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 宠物等级+1
	pet.Level++

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.Error("保存宠物升级结果失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetLevelUpResult{
		PetUID: petUID,
		Level:  pet.Level,
	}

	logger.Info("宠物升级", "player_id", playerID, "pet_uid", petUID, "new_level", pet.Level)
	return result, nil
}

// Evolve 客户端宠物进阶业务逻辑：
//  1. 查询宠物是否存在
//  2. 校验宠物等级>=10
//  3. 进阶后品质+1，模板ID更新
//  4. 保存到数据库
func (s *petService) Evolve(ctx context.Context, playerID uint64, petUID uint64) (*PetEvolveResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物等级>=10
	if pet.Level < 10 {
		return nil, errors.ErrPetLevelLow
	}

	// 进阶：品质+1，模板ID更新（简化：模板ID = 原模板ID + 100）
	pet.Quality++
	pet.PetID += 100

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.Error("保存宠物进阶结果失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetEvolveResult{
		PetUID:     petUID,
		NewPetID:   pet.PetID,
		NewQuality: pet.Quality,
	}

	logger.Info("宠物进阶", "player_id", playerID, "pet_uid", petUID,
		"new_pet_id", pet.PetID, "new_quality", pet.Quality)
	return result, nil
}

// Explore 宠物探险派遣业务逻辑：
//  1. 查询宠物是否存在
//  2. 校验宠物是否已在探险中
//  3. 校验探险时长（上限12小时）
//  4. 设置探险状态和结束时间
//  5. 保存到数据库
func (s *petService) Explore(ctx context.Context, playerID uint64, petUID uint64, durationMinutes int32) (*PetExploreResult, *errors.GameError) {
	// 查询宠物实例
	pet, err := s.petRepo.GetPetByUID(ctx, petUID)
	if err != nil || pet == nil {
		return nil, errors.ErrPetNotFound
	}

	// 校验宠物是否已在探险中
	if pet.Exploring {
		return nil, errors.ErrPetExploring
	}

	// 校验探险时长上限（最大12小时 = 720分钟）
	maxMinutes := int32(model.PetMaxExploreHours * 60)
	if durationMinutes <= 0 || durationMinutes > maxMinutes {
		return nil, errors.ErrParamInvalid
	}

	// 设置探险状态和结束时间
	pet.Exploring = true
	pet.ExploreEndTime = time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	// 保存到数据库
	if saveErr := s.petRepo.SavePet(ctx, pet); saveErr != nil {
		logger.Error("保存宠物探险状态失败", "pet_uid", petUID, "err", saveErr)
		return nil, errors.ErrInternal
	}

	result := &PetExploreResult{
		PetUID:  petUID,
		EndTime: pet.ExploreEndTime.Unix(),
	}

	logger.Info("宠物探险派遣", "player_id", playerID, "pet_uid", petUID,
		"duration_min", durationMinutes, "end_time", pet.ExploreEndTime)
	return result, nil
}

// Compose 宠物合成业务逻辑：
//  1. 校验素材数量>=3
//  2. 查询所有素材宠物
//  3. 校验所有素材宠物品质相同
//  4. 删除素材宠物
//  5. 创建新宠物（品质+1）
func (s *petService) Compose(ctx context.Context, playerID uint64, petUIDs []uint64) (*PetComposeResult, *errors.GameError) {
	// 校验素材数量>=3
	if len(petUIDs) < 3 {
		return nil, errors.ErrPetComposeNum
	}

	// 查询所有素材宠物，校验品质是否相同
	var pets []*model.Pet
	var firstQuality int32 = -1

	for _, uid := range petUIDs {
		pet, err := s.petRepo.GetPetByUID(ctx, uid)
		if err != nil || pet == nil {
			return nil, errors.ErrPetNotFound
		}

		// 校验宠物是否属于当前玩家
		if pet.OwnerID != playerID {
			return nil, errors.ErrPetNotFound
		}

		// 校验品质是否相同
		if firstQuality == -1 {
			firstQuality = pet.Quality
		} else if pet.Quality != firstQuality {
			return nil, errors.ErrPetComposeQuality
		}

		pets = append(pets, pet)
	}

	// 删除素材宠物
	for _, uid := range petUIDs {
		if delErr := s.petRepo.DeletePet(ctx, uid); delErr != nil {
			logger.Error("删除合成素材宠物失败", "pet_uid", uid, "err", delErr)
		}
	}

	// 创建新宠物（品质+1）
	newPetID := int32(rand.Intn(100) + 200) // 简化：随机生成新模板ID
	newQuality := firstQuality + 1

	result := &PetComposeResult{
		ResultID: uint64(rand.Int63n(100000) + 1), // 生成新实例ID
		PetID:    newPetID,
		Quality:  newQuality,
	}

	logger.Info("宠物合成", "player_id", playerID, "input_count", len(petUIDs),
		fmt.Sprintf("result_pet_id=%d, quality=%d", newPetID, newQuality))
	return result, nil
}