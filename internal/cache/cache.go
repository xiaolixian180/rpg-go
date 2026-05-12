// cache 包封装了 Redis 缓存的连接与常用操作，
// 提供玩家数据缓存、Boss 冷却计时、排行榜等功能。
package cache

import (
	"context"
	"fmt"
	"time"

	"hero-quest/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Redis 是对 go-redis 客户端的封装，
// 在底层客户端之上提供业务语义化的缓存方法。
type Redis struct {
	client *redis.Client // 底层 go-redis 客户端实例
}

// Config 包含 Redis 连接所需的所有配置项，
// 通常从配置文件或环境变量中读取后传入 New 函数。
type Config struct {
	Host     string // Redis 主机地址，如 "127.0.0.1"
	Port     int    // Redis 端口号，如 6379
	Password string // Redis 认证密码，无密码时为空字符串
	DB       int    // Redis 数据库编号（0-15），用于隔离不同环境的数据
}

// New 根据配置创建并初始化 Redis 连接。
// 关键逻辑：
//  1. 拼接地址字符串，创建 go-redis 客户端实例。
//  2. 使用 5 秒超时的 Ping 命令验证连接可用性，避免长时间阻塞。
//  3. 连接成功后返回封装后的 Redis 实例。
func New(cfg Config) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	logger.Info("redis connected", "host", cfg.Host)
	return &Redis{client: client}, nil
}

// Client 返回底层 go-redis 客户端实例，
// 供需要使用未封装的 Redis 命令的场景调用。
func (r *Redis) Client() *redis.Client {
	return r.client
}

// SetPlayer 将玩家数据序列化后缓存到 Redis。
// 键格式为 "player:{playerID}"，过期时间 30 分钟，
// 避免缓存数据长期不一致。调用方需在数据变更时主动刷新或删除缓存。
func (r *Redis) SetPlayer(ctx context.Context, playerID uint64, data []byte) error {
	return r.client.Set(ctx, fmt.Sprintf("player:%d", playerID), data, 30*time.Minute).Err()
}

// GetPlayer 从 Redis 读取玩家缓存数据。
// 键格式为 "player:{playerID}"。
// 若键不存在，redis 会返回 nil 和 redis.Nil 错误，调用方需据此区分"缓存未命中"和"真正的错误"。
func (r *Redis) GetPlayer(ctx context.Context, playerID uint64) ([]byte, error) {
	return r.client.Get(ctx, fmt.Sprintf("player:%d", playerID)).Bytes()
}

// DelPlayer 删除指定玩家的缓存数据。
// 典型场景：玩家数据变更后主动淘汰旧缓存，保证下次读取时从数据库重新加载。
func (r *Redis) DelPlayer(ctx context.Context, playerID uint64) error {
	return r.client.Del(ctx, fmt.Sprintf("player:%d", playerID)).Err()
}

// SetBossCooldown 设置指定层数 Boss 的冷却标记。
// 键格式为 "boss:cooldown:{layer}"，值为占位字符串 "1"，
// 过期时间由 cooldown 参数决定，到期后 Redis 自动删除该键，冷却即结束。
// 利用 Redis 的 TTL 机制实现倒计时，无需后台轮询。
func (r *Redis) SetBossCooldown(ctx context.Context, layer int32, cooldown time.Duration) error {
	return r.client.Set(ctx, fmt.Sprintf("boss:cooldown:%d", layer), "1", cooldown).Err()
}

// IsBossCooldown 检查指定层数的 Boss 是否处于冷却中。
// 通过 EXISTS 命令判断冷却键是否还存在：
//   - 存在（返回值 > 0）：Boss 仍在冷却中
//   - 不存在（返回值 = 0）：冷却已结束，玩家可再次挑战
func (r *Redis) IsBossCooldown(ctx context.Context, layer int32) (bool, error) {
	val, err := r.client.Exists(ctx, fmt.Sprintf("boss:cooldown:%d", layer)).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// UpdateRanking 向排行榜中添加或更新成员分数。
// 使用 Redis 有序集合（ZSet），key 为排行榜名称，member 为成员标识，score 为分数。
// 若 member 已存在则更新其分数，不存在则新增。
func (r *Redis) UpdateRanking(ctx context.Context, key string, member string, score float64) error {
	return r.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// GetRanking 获取排行榜中分数从高到低的一段排名。
// 使用 ZRevRangeWithScores 按分数降序返回，offset 为起始偏移量，count 为获取条数。
// 例如 offset=0, count=10 返回前 10 名。
func (r *Redis) GetRanking(ctx context.Context, key string, offset, count int64) ([]redis.Z, error) {
	return r.client.ZRevRangeWithScores(ctx, key, offset, offset+count-1).Result()
}

// Close 关闭 Redis 连接，释放资源。
// 通常在应用关闭阶段调用。
func (r *Redis) Close() error {
	return r.client.Close()
}
