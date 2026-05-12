// Package logger 提供统一的结构化日志功能。
// 基于 Go 标准库 slog 实现，输出 JSON 格式日志到标准输出，
// 支持通过配置动态调整日志级别（debug/info/warn/error），
// 便于在生产环境中进行日志采集和检索分析。
package logger

import (
	"log/slog"
	"os"
)

// Init 初始化全局日志器。根据传入的级别字符串设置日志输出级别，
// 并将默认日志器替换为 JSON 格式的 handler。
// 仅识别 "debug"/"info"/"warn"/"error" 四种级别，
// 传入无法识别的字符串时默认使用 info 级别，避免静默丢失重要日志。
func Init(level string) {
	// 将字符串级别的日志级别映射为 slog.Level 常量
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		// 未知级别降级为 info，确保日志不会过于冗余也不会完全静默
		lvl = slog.LevelInfo
	}

	// 创建 JSON 格式的 handler，输出到标准输出，便于日志采集系统收集
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	// 设为全局默认日志器，后续所有 slog 调用均使用此 handler
	slog.SetDefault(slog.New(handler))
}

// Debug 输出调试级别日志，仅在 debug 级别启用时才会输出，
// 用于开发阶段打印详细运行时信息（如请求参数、中间状态等）。
func Debug(msg string, args ...any) { slog.Debug(msg, args...) }

// Info 输出信息级别日志，记录常规运行事件（如服务启动、请求完成等）。
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Warn 输出警告级别日志，记录非致命异常（如配置缺失使用默认值、重试成功等）。
func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

// Error 输出错误级别日志，记录严重错误（如数据库连接失败、外部服务不可用等）。
func Error(msg string, args ...any) { slog.Error(msg, args...) }
