// Package service - Boss服务
// 提供Boss生成检查、Boss死亡处理（掉落、通关、冷却、播报）等业务逻辑
package service

import (
	"context"
	"fmt"
	"math/rand"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== Boss服务接口 ====================

// BossService Boss服务接口，定义Boss生成检查和死亡处理等业务操作
type BossService interface {
	// CheckSpawn 检查指定层是否需要生成Boss，以及Boss是否在冷却中
	CheckSpawn(ctx context.Context, layer int32) (shouldSpawn bool, gameErr *errors.GameError)
	// OnDie Boss死亡处理，包括掉落计算、通关进度更新、冷却设置、全服播报
	OnDie(ctx context.Context, killer *model.Player, boss *model.Boss) (*BossDieResult, *errors.GameError)
}

// ==================== Boss死亡结果结构体 ====================

// BossDieResult Boss死亡处理结果
type BossDieResult struct {
	Drops   []model.DropItem // Boss掉落物品列表
	Message string           // 全服播报内容（稀有掉落时）
}

// ==================== Boss服务实现 ====================

// bossService Boss服务实现
type bossService struct {
	cacheRepo  repo.CacheRepo  // 缓存访问接口（Boss冷却、通关进度）
	playerRepo repo.PlayerRepo // 玩家数据访问接口（通关进度持久化）
}

// NewBossService 创建Boss服务实例
func NewBossService(cacheRepo repo.CacheRepo, playerRepo repo.PlayerRepo) BossService {
	return &bossService{
		cacheRepo:  cacheRepo,
		playerRepo: playerRepo,
	}
}

// CheckSpawn 检查指定层是否需要生成Boss：
//  1. 仅Boss层（每10层）才需要生成Boss
//  2. 检查Boss是否在冷却中（冷却期间不生成）
func (s *bossService) CheckSpawn(ctx context.Context, layer int32) (bool, *errors.GameError) {
	// 非Boss层不需要生成
	if !model.IsBossLayer(layer) {
		return false, nil
	}

	// 检查Boss是否在冷却中
	inCooldown, err := s.cacheRepo.IsBossCooldown(ctx, layer)
	if err != nil {
		logger.Error("检查Boss冷却状态失败", "layer", layer, "err", err)
		return false, errors.ErrInternal
	}

	// 冷却中不生成新Boss
	if inCooldown {
		return false, nil
	}

	return true, nil
}

// OnDie Boss死亡处理业务逻辑：
//  1. 计算Boss掉落物品（品质随机：白/绿/蓝/紫/橙）
//  2. 更新击杀者的地下城通关进度（若该层超过历史最高层则更新）
//  3. 将通关进度持久化到数据库
//  4. 设置Boss冷却时间到Redis
//  5. 品质>=紫色的掉落物生成全服播报内容
func (s *bossService) OnDie(ctx context.Context, killer *model.Player, boss *model.Boss) (*BossDieResult, *errors.GameError) {
	result := &BossDieResult{}

	// 1. 计算Boss掉落物品
	result.Drops = s.calcBossDrops(boss)

	// 2. 更新击杀者的通关进度
	if boss.Layer > killer.MaxLayer {
		killer.MaxLayer = boss.Layer

		// 3. 持久化通关进度到数据库
		if err := s.playerRepo.SaveMaxLayer(ctx, killer.ID, boss.Layer); err != nil {
			logger.Error("保存通关进度失败", "player_id", killer.ID, "layer", boss.Layer, "err", err)
		}
	}

	// 4. 设置Boss冷却时间到Redis
	if err := s.cacheRepo.SetBossCooldown(ctx, boss.Layer, boss.Cooldown); err != nil {
		logger.Error("设置Boss冷却时间失败", "layer", boss.Layer, "err", err)
	}

	// 5. 稀有掉落（紫色及以上品质）生成全服播报内容
	for _, drop := range result.Drops {
		if drop.Quality >= 4 {
			result.Message = fmt.Sprintf("%s 击败了 %s，获得了 %s！", killer.Name, boss.Name, drop.Name)
			break
		}
	}

	logger.Info("Boss被击杀", "boss_id", boss.ID, "killer_id", killer.ID, "drops", len(result.Drops))
	return result, nil
}

// calcBossDrops 计算Boss掉落物品，使用简化的随机掉落算法
// 掉落品质概率：
//   - 品质1~3（白/绿/蓝）：默认品质，quality = 1 + rand(0~2)
//   - 品质4（紫色）：10%概率提升到紫色
//   - 品质5（橙色）：2%概率提升到橙色（覆盖紫色判定）
// 每次Boss死亡固定掉落1件装备
func (s *bossService) calcBossDrops(boss *model.Boss) []model.DropItem {
	var drops []model.DropItem

	// 默认品质：白/绿/蓝随机
	quality := int32(1 + rand.Intn(3))

	// 10%概率提升为紫色（品质4）
	if rand.Float64() < 0.1 {
		quality = 4
	}

	// 2%概率提升为橙色（品质5），橙色判定覆盖紫色
	if rand.Float64() < 0.02 {
		quality = 5
	}

	drops = append(drops, model.DropItem{
		ItemID:  uint64(rand.Intn(1000) + 1),
		Name:    fmt.Sprintf("Boss装备_%d", boss.Layer),
		Quality: quality,
		Count:   1,
	})

	return drops
}
