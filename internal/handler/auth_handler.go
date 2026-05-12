package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// AuthHandler 登录/角色模块消息处理器
// 负责处理玩家登录和创建角色的网络消息
type AuthHandler struct {
	playerSvc service.PlayerService // 玩家服务接口
}

// NewAuthHandler 创建登录/角色模块处理器实例
func NewAuthHandler(playerSvc service.PlayerService) *AuthHandler {
	return &AuthHandler{playerSvc: playerSvc}
}

// HandleLogin 处理登录请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体
//  2. 调用 PlayerService.Login 执行登录逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送登录响应
func (h *AuthHandler) HandleLogin(conn *gateway.Conn, body []byte) {
	// 反序列化登录请求
	var req protocol.C2SLogin
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，返回参数无效错误
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用玩家服务执行登录
	player, ge := h.playerSvc.Login(context.Background(), conn.PlayerID)
	if ge != nil {
		// 登录失败，将 GameError 的错误码写入响应
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: ge.Code})
		return
	}

	// 登录成功，将 model.Player 转换为协议层 PlayerData 并发送
	conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{
		Code:   errors.ErrSuccess.Code,
		Player: *toPlayerData(player),
	})
}

// HandleCreatePlayer 处理创建角色请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（角色名+职业）
//  2. 调用 PlayerService.Create 执行角色创建逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送创建角色响应
func (h *AuthHandler) HandleCreatePlayer(conn *gateway.Conn, body []byte) {
	// 反序列化创建角色请求
	var req protocol.C2SCreatePlayer
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，返回参数无效错误
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用玩家服务执行角色创建
	player, ge := h.playerSvc.Create(context.Background(), conn.PlayerID, req.Name, req.Class)
	if ge != nil {
		// 创建失败，将 GameError 的错误码写入响应
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{Code: ge.Code})
		return
	}

	// 创建成功，将 model.Player 转换为协议层 PlayerData 并发送
	conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{
		Code:   errors.ErrSuccess.Code,
		Player: *toPlayerData(player),
	})
}

// getPlayer 是通用辅助方法，通过玩家ID获取 *model.Player 对象。
// 如果玩家不在线或查询失败，发送未登录错误并返回 nil。
func getPlayer(conn *gateway.Conn, playerSvc service.PlayerService) *model.Player {
	player, ge := playerSvc.GetPlayer(context.Background(), conn.PlayerID)
	if ge != nil || player == nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: errors.ErrNotLogin.Code})
		return nil
	}
	return player
}