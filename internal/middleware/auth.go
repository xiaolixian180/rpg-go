// Package middleware 提供游戏服务器的中间件功能，
// 包括认证检查、限流控制和异常恢复，用于在消息处理前后执行通用逻辑。
package middleware

import (
	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/pkg/logger"
)

// AuthMiddleware 检查连接是否已认证（PlayerID > 0）。
// 未认证的连接会收到 MsgIDKick 踢下线通知后断开连接。
// 返回 true 表示通过认证，false 表示未认证。
func AuthMiddleware(conn *gateway.Conn) bool {
	// PlayerID > 0 说明已完成认证（登录时赋值）
	if conn.PlayerID > 0 {
		return true
	}

	// 未认证连接：发送踢下线消息，告知客户端原因
	logger.Warn("unauthenticated connection rejected", "conn_id", conn.ID)
	conn.Send(protocol.MsgIDKick, &protocol.S2CKick{
		Reason: "未登录，请先完成认证",
	})

	return false
}
