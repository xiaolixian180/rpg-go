// repo 包 - 缓存数据访问实现
// 实现 CacheRepo 接口，基于 Redis 提供玩家缓存、Boss冷却、排行榜等缓存操作
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hero-quest/internal/cache"

	"github.com/redis/go-redis/v9"
)

// ==================== 缓存数据访问实现 ====================

// cacheRepo 是 CacheRepo 接口的具体实现，使用 cache.Redis 操作 Redis。
type cacheRepo struct {
	rds *cache.Redis
}

// NewCacheRepo 创建 CacheRepo 实例。
func NewCacheRepo(rds *cache.Redis) CacheRepo {
	return &cacheRepo{rds: rds}
}

// SetPlayerCache 将玩家数据序列化后缓存到 Redis。
func (r *cacheRepo) SetPlayerCache(ctx context.Context, playerID uint64, data []byte) error {
	if err := r.rds.SetPlayer(ctx, playerID, data); err != nil {
		return fmt.Errorf("set player cache id=%d: %w", playerID, err)
	}
	return nil
}

// GetPlayerCache 从 Redis 读取玩家缓存数据。
// 缓存未命中时返回 nil, nil。
func (r *cacheRepo) GetPlayerCache(ctx context.Context, playerID uint64) ([]byte, error) {
	data, err := r.rds.GetPlayer(ctx, playerID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// redis.Nil 表示键不存在，视为缓存未命中而非错误
			return nil, nil
		}
		// 其他错误：记录日志并返回
		return nil, fmt.Errorf("get player cache id=%d: %w", playerID, err)
	}
	return data, nil
}

// DelPlayerCache 删除指定玩家的缓存数据。
func (r *cacheRepo) DelPlayerCache(ctx context.Context, playerID uint64) error {
	if err := r.rds.DelPlayer(ctx, playerID); err != nil {
		return fmt.Errorf("del player cache id=%d: %w", playerID, err)
	}
	return nil
}

// SetBossCooldown 设置指定层数 Boss 的冷却标记。
func (r *cacheRepo) SetBossCooldown(ctx context.Context, layer int32, cooldown time.Duration) error {
	if err := r.rds.SetBossCooldown(ctx, layer, cooldown); err != nil {
		return fmt.Errorf("set boss cooldown layer=%d: %w", layer, err)
	}
	return nil
}

// IsBossCooldown 检查指定层数的 Boss 是否处于冷却中。
func (r *cacheRepo) IsBossCooldown(ctx context.Context, layer int32) (bool, error) {
	ok, err := r.rds.IsBossCooldown(ctx, layer)
	if err != nil {
		return false, fmt.Errorf("check boss cooldown layer=%d: %w", layer, err)
	}
	return ok, nil
}

// UpdateRanking 向排行榜中添加或更新成员分数。
func (r *cacheRepo) UpdateRanking(ctx context.Context, key string, member string, score float64) error {
	if err := r.rds.UpdateRanking(ctx, key, member, score); err != nil {
		return fmt.Errorf("update ranking key=%s member=%s: %w", key, member, err)
	}
	return nil
}

// GetRanking 获取排行榜中分数从高到低的一段排名。
func (r *cacheRepo) GetRanking(ctx context.Context, key string, offset, count int64) ([]RankItem, error) {
	result, err := r.rds.GetRanking(ctx, key, offset, count)
	if err != nil {
		return nil, fmt.Errorf("get ranking key=%s: %w", key, err)
	}

	// 将 redis.Z 转换为 RankItem
	items := make([]RankItem, 0, len(result))
	for _, z := range result {
		items = append(items, RankItem{
			Member: fmt.Sprintf("%v", z.Member),
			Score:  z.Score,
		})
	}
	return items, nil
}
