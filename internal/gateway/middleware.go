package gateway

import (
	"runtime/debug"

	"hero-quest/pkg/logger"
)

// Middleware 消息处理中间件。
// 接收消息ID、连接、消息体和下一层 handler，由中间件决定是否调用 next。
type Middleware func(msgID uint16, conn *Conn, body []byte, next Handler)

// RecoveryMiddleware 捕获 handler 中的 panic，防止单个消息异常导致连接崩溃。
func RecoveryMiddleware(msgID uint16, conn *Conn, body []byte, next Handler) {
	defer func() {
		if r := recover(); r != nil {
			ctx := conn.Context()
			logger.TError(ctx, "消息处理panic",
				"msg_id", msgID, "conn_id", conn.ID, "player_id", conn.PlayerID,
				"err", r, "stack", string(debug.Stack()))
		}
	}()
	next(conn, body)
}

// RateLimitMiddleware 基于连接的令牌桶限流器，超出频率的消息直接丢弃。
func RateLimitMiddleware(msgID uint16, conn *Conn, body []byte, next Handler) {
	if !conn.Limiter().Allow() {
		ctx := conn.Context()
		logger.TWarn(ctx, "请求频率超限",
			"conn_id", conn.ID, "player_id", conn.PlayerID, "msg_id", msgID)
		return
	}
	next(conn, body)
}

// MaxBodySizeMiddleware 返回一个限制消息体大小的中间件。
func MaxBodySizeMiddleware(maxSize int) Middleware {
	return func(msgID uint16, conn *Conn, body []byte, next Handler) {
		if len(body) > maxSize {
			ctx := conn.Context()
			logger.TWarn(ctx, "消息体过大",
				"msg_id", msgID, "conn_id", conn.ID, "size", len(body), "max", maxSize)
			return
		}
		next(conn, body)
	}
}

// AuthGuardMiddleware 返回一个认证守卫中间件。
func AuthGuardMiddleware(whitelist ...uint16) Middleware {
	allowed := make(map[uint16]struct{}, len(whitelist))
	for _, id := range whitelist {
		allowed[id] = struct{}{}
	}
	return func(msgID uint16, conn *Conn, body []byte, next Handler) {
		if _, ok := allowed[msgID]; !ok && conn.PlayerID == 0 {
			ctx := conn.Context()
			logger.TWarn(ctx, "未认证连接请求",
				"msg_id", msgID, "conn_id", conn.ID)
			return
		}
		next(conn, body)
	}
}
