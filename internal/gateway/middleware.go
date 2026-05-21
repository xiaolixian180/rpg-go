package gateway

import (
	"expvar"
	"strconv"
	"time"

	"hero-quest/pkg/logger"
	"runtime/debug"
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

// TraceMiddleware 为每条消息生成唯一 trace_id，写入连接的 traceCtx，
// 后续 handler/service 通过 context 传播实现全链路追踪。
func TraceMiddleware(msgID uint16, conn *Conn, body []byte, next Handler) {
	traceID := logger.NewTraceID()
	conn.traceCtx = logger.WithTrace(conn.traceCtx, traceID)
	next(conn, body)
}

// AccessLogMiddleware 记录每条消息的请求/响应日志（含耗时）。
// 放在 TraceMiddleware 之后，确保有 trace_id。
func AccessLogMiddleware(msgID uint16, conn *Conn, body []byte, next Handler) {
	start := time.Now()
	ctx := conn.Context()

	next(conn, body)

	cost := time.Since(start)
	logger.TInfo(ctx, "请求处理完成",
		"msg_id", msgID, "conn_id", conn.ID, "player_id", conn.PlayerID,
		"cost_ms", cost.Milliseconds())
}

// MetricsMiddleware 记录每条消息的处理延迟和消息计数。
func MetricsMiddleware(msgID uint16, conn *Conn, body []byte, next Handler) {
	start := time.Now()
	defer func() {
		key := strconv.Itoa(int(msgID))
		msgCount.Add(key, 1)
		msgLatency.Add(key, time.Since(start).Nanoseconds())
	}()
	next(conn, body)
}

var (
	msgCount   = expvar.NewMap("gateway_msg_count")
	msgLatency = expvar.NewMap("gateway_msg_latency_ns")
)
