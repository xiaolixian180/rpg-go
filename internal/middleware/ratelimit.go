// Package middleware 提供游戏服务器的中间件功能，
// 包括认证检查、限流控制和异常恢复，用于在消息处理前后执行通用逻辑。
package middleware

import (
	"hero-quest/internal/gateway"
	"hero-quest/pkg/logger"
)

// RateLimitMiddleware 检查连接是否超过消息频率限制。
// 使用 conn 内置的令牌桶限流器（golang.org/x/time/rate.Limiter），
// 限流参数在连接创建时由网关的 rateLimit 配置决定。
// 返回 true 表示允许通过，false 表示被限流（消息应被丢弃）。
func RateLimitMiddleware(conn *gateway.Conn) bool {
	// 调用连接内置的令牌桶限流器，尝试消耗一个令牌
	if conn.Limiter().Allow() {
		return true
	}

	// 令牌不足，当前消息频率超出限制，记录警告日志
	logger.Warn("rate limit exceeded", "conn_id", conn.ID, "player_id", conn.PlayerID)
	return false
}
