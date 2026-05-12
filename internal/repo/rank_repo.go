package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/cache"
)

// ==================== 排行榜数据访问实现 ====================

// rankRepo 是 RankRepo 接口的具体实现，使用 cache.Redis 的 ZSet 操作。
// 排行榜数据存储在 Redis 有序集合中，利用其天然排序特性实现高效排名查询。
type rankRepo struct {
	rds *cache.Redis
}

// NewRankRepo 创建 RankRepo 实例。
func NewRankRepo(rds *cache.Redis) RankRepo {
	return &rankRepo{rds: rds}
}

// Update 更新排行榜中指定成员的分数。
// 底层调用 Redis ZADD 命令，若成员已存在则覆盖分数，不存在则新增。
func (r *rankRepo) Update(ctx context.Context, key string, member string, score float64) error {
	if err := r.rds.UpdateRanking(ctx, key, member, score); err != nil {
		return fmt.Errorf("update rank key=%s member=%s: %w", key, member, err)
	}
	return nil
}

// GetTopN 获取排行榜前 N 名，按分数从高到低排列。
// 底层调用 Redis ZREVRANGEWITHSCORES 命令，
// 将 redis.Z 转换为 RankItem 返回，供 service 层使用。
func (r *rankRepo) GetTopN(ctx context.Context, key string, n int64) ([]RankItem, error) {
	result, err := r.rds.GetRanking(ctx, key, 0, n)
	if err != nil {
		return nil, fmt.Errorf("get top %d rank key=%s: %w", n, key, err)
	}

	// 将 redis.Z 转换为 RankItem，避免 service 层依赖 redis 包
	items := make([]RankItem, 0, len(result))
	for _, z := range result {
		items = append(items, RankItem{
			Member: fmt.Sprintf("%v", z.Member),
			Score:  z.Score,
		})
	}
	return items, nil
}
