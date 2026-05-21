package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/player"
	"hero-quest/pkg/errors"
)

// AuthHandler 登录/角色模块消息处理器
type AuthHandler struct {
	world     service.World        // 游戏世界（用于 OnLogin 加载到内存）
	playerSvc player.PlayerService // 玩家服务（用于创建角色）
}

// NewAuthHandler 创建登录/角色模块处理器实例
func NewAuthHandler(world service.World, playerSvc player.PlayerService) *AuthHandler {
	return &AuthHandler{world: world, playerSvc: playerSvc}
}

// HandleLogin 处理登录请求
func (h *AuthHandler) HandleLogin(conn *gateway.Conn, body []byte) {
	var req protocol.C2SLogin
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 通过 World.OnLogin 加载玩家到内存
	player, err := h.world.OnLogin(conn.PlayerID)
	if err != nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: errors.ErrInternal.Code})
		return
	}

	conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{
		Code:   errors.ErrSuccess.Code,
		Player: *toPlayerData(player),
	})
}

// HandleCreatePlayer 处理创建角色请求
func (h *AuthHandler) HandleCreatePlayer(conn *gateway.Conn, body []byte) {
	var req protocol.C2SCreatePlayer
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player, ge := h.playerSvc.Create(connCtx(conn), conn.PlayerID, req.Name, req.Class)
	if ge != nil {
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{
		Code:   errors.ErrSuccess.Code,
		Player: *toPlayerData(player),
	})
}
