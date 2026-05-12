// Package middleware 提供游戏服务器的中间件功能，
// 包括认证检查、限流控制和异常恢复，用于在消息处理前后执行通用逻辑。
package middleware

import (
	"fmt"

	"hero-quest/internal/gateway"
	"hero-quest/pkg/logger"
)

// RecoveryMiddleware 捕获 handler 中的 panic，
// 记录错误日志，防止单个请求的 panic 导致整个服务崩溃。
// 返回一个安全的 handler 包装函数，原 handler 发生 panic 时会被恢复并记录日志，
// 不会影响其他连接和消息的正常处理。
func RecoveryMiddleware(h gateway.Handler) gateway.Handler {
	return func(conn *gateway.Conn, body []byte) {
		// 使用 defer + recover 捕获 panic
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 的详细错误信息，包括连接ID和玩家ID，
				// 便于排查问题而不会导致服务整体崩溃
				logger.Error("handler panic recovered",
					"conn_id", conn.ID,
					"player_id", conn.PlayerID,
					"error", fmt.Sprintf("%v", r),
				)
			}
		}()

		// 执行原始的 handler 逻辑
		h(conn, body)
	}
}
