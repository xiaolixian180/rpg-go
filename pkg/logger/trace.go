package logger

import (
	"context"
	"log/slog"

	"github.com/rs/xid"
)

type contextKey struct{}

// TraceID 返回标准 trace_id 日志键名
const TraceIDKey = "trace_id"

// NewTraceID 生成一个新的追踪ID（基于 xid，有序且唯一）
func NewTraceID() string {
	return xid.New().String()
}

// WithTrace 将 traceID 注入到 context 中
func WithTrace(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKey{}, traceID)
}

// GetTrace 从 context 中提取 traceID，没有则返回空字符串
func GetTrace(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}

// traceArgs 从 context 提取 trace_id，追加到日志参数中
func traceArgs(ctx context.Context) []any {
	if id := GetTrace(ctx); id != "" {
		return []any{TraceIDKey, id}
	}
	return nil
}

// TInfo 带追踪的信息级别日志，自动从 context 提取 trace_id
func TInfo(ctx context.Context, msg string, args ...any) {
	all := append(traceArgs(ctx), args...)
	slog.Info(msg, all...)
}

// TDebug 带追踪的调试级别日志
func TDebug(ctx context.Context, msg string, args ...any) {
	all := append(traceArgs(ctx), args...)
	slog.Debug(msg, all...)
}

// TWarn 带追踪的警告级别日志
func TWarn(ctx context.Context, msg string, args ...any) {
	all := append(traceArgs(ctx), args...)
	slog.Warn(msg, all...)
}

// TError 带追踪的错误级别日志
func TError(ctx context.Context, msg string, args ...any) {
	all := append(traceArgs(ctx), args...)
	slog.Error(msg, all...)
}
