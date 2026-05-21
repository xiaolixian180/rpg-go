// Package rank - 排行榜服务
// 提供排行榜查询和更新等排行榜相关业务逻辑
package rank

import (
	"context"
	"fmt"
	"strconv"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 排行榜服务接口 ====================

// RankService 排行榜服务接口，定义排行榜查询和更新操作
type RankService interface {
	// GetRanking 查询排行榜（等级/战力/荣誉），从Redis member字符串解析playerID，查询玩家名称
	GetRanking(ctx context.Context, rankType int32) ([]*model.RankingItem, *errors.GameError)
	// UpdateRanking 更新排行榜数据（玩家分数变更时调用）
	UpdateRanking(ctx context.Context, playerID uint64, rankType int32, value int64) *errors.GameError
}

// ==================== 排行榜类型常量 ====================

// 排行榜类型到Redis键名的映射
var rankTypeToKey = map[int32]string{
	0: "rank:level", // 等级排行榜
	1: "rank:power", // 战力排行榜
	2: "rank:honor", // 荣誉排行榜
}

// ==================== 排行榜服务实现 ====================

// rankService 排行榜服务实现
type rankService struct {
	cacheRepo  repo.CacheRepo  // 缓存访问接口（排行榜使用Redis有序集合）
	playerRepo repo.PlayerRepo // 玩家数据访问接口（查询玩家名称）
}

// NewRankService 创建排行榜服务实例（需要 PlayerRepo 依赖用于查询玩家名称）
func NewRankService(cacheRepo repo.CacheRepo, playerRepo repo.PlayerRepo) RankService {
	return &rankService{
		cacheRepo:  cacheRepo,
		playerRepo: playerRepo,
	}
}

// GetRanking 查询排行榜业务逻辑（修复：从member字符串解析playerID，查询玩家名称）：
//  1. 根据排行榜类型获取对应的Redis键名
//  2. 从Redis有序集合中按分数降序获取前50名
//  3. 解析member字符串中的playerID
//  4. 查询玩家名称用于展示
//  5. 转换为排行榜数据格式
func (s *rankService) GetRanking(ctx context.Context, rankType int32) ([]*model.RankingItem, *errors.GameError) {
	// 获取排行榜对应的Redis键名
	key, ok := rankTypeToKey[rankType]
	if !ok {
		return nil, errors.ErrParamInvalid
	}

	// 从Redis有序集合中按分数降序获取前50名
	items, err := s.cacheRepo.GetRanking(ctx, key, 0, 50)
	if err != nil {
		logger.TError(ctx, "查询排行榜失败", "rank_type", rankType, "err", err)
		return nil, errors.ErrInternal
	}

	// 转换为排行榜数据格式，解析playerID并查询玩家名称
	var rankings []*model.RankingItem
	for i, item := range items {
		// 从member字符串解析玩家ID（member格式为 "playerID"）
		playerID, parseErr := strconv.ParseUint(item.Member, 10, 64)
		if parseErr != nil {
			logger.TError(ctx, "解析排行榜member失败", "member", item.Member, "err", parseErr)
			playerID = 0
		}

		// 查询玩家名称
		name := item.Member // 默认使用member字符串作为名称
		if playerID > 0 {
			if p, playerErr := s.playerRepo.GetPlayerByID(ctx, playerID); playerErr == nil && p != nil {
				name = p.Name
			}
		}

		rankings = append(rankings, &model.RankingItem{
			Rank:     int32(i + 1),
			PlayerID: playerID,
			Name:     name,
			Value:    int64(item.Score),
		})
	}

	return rankings, nil
}

// UpdateRanking 更新排行榜数据业务逻辑：
//  1. 根据排行榜类型获取对应的Redis键名
//  2. 将玩家分数更新到Redis有序集合
func (s *rankService) UpdateRanking(ctx context.Context, playerID uint64, rankType int32, value int64) *errors.GameError {
	// 获取排行榜对应的Redis键名
	key, ok := rankTypeToKey[rankType]
	if !ok {
		return errors.ErrParamInvalid
	}

	// 将玩家分数更新到Redis有序集合
	member := fmt.Sprintf("%d", playerID)
	if err := s.cacheRepo.UpdateRanking(ctx, key, member, float64(value)); err != nil {
		logger.TError(ctx, "更新排行榜数据失败", "player_id", playerID, "rank_type", rankType, "err", err)
		return errors.ErrInternal
	}

	return nil
}
