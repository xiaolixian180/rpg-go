package handler

import (
	"encoding/json"

	"hero-quest/internal/eventbus"
	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/player"
	"hero-quest/pkg/auth"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// AuthHandler 登录/角色模块消息处理器
type AuthHandler struct {
	world     service.World        // 游戏世界（用于 OnLogin 加载到内存）
	playerSvc player.PlayerService // 玩家服务（用于创建角色）
	jwtMgr    *auth.JWTManager     // JWT 管理器，用于协议层登录鉴权
	bus       *eventbus.Bus        // 事件总线
}

// NewAuthHandler 创建登录/角色模块处理器实例
func NewAuthHandler(world service.World, playerSvc player.PlayerService, jwtMgr *auth.JWTManager, bus *eventbus.Bus) *AuthHandler {
	return &AuthHandler{world: world, playerSvc: playerSvc, jwtMgr: jwtMgr, bus: bus}
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

	playerID, ok := h.parsePlayerID(req.Token)
	if !ok {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: errors.ErrTokenInvalid.Code})
		return
	}
	conn.BindPlayer(playerID)

	// 通过 World.OnLogin 加载玩家到内存
	player, err := h.world.OnLogin(playerID)
	if err != nil {
		conn.Send(protocol.MsgIDLoginResp, &protocol.S2CLoginResp{Code: errors.ErrInternal.Code})
		return
	}

	// 发布登录事件（用于更新排行榜等）
	player.Mu().RLock()
	playerLevel := player.Level
	playerName := player.Name
	player.Mu().RUnlock()
	h.bus.Publish(eventbus.TopicPlayerLogin, &eventbus.PlayerLoginEvent{
		PlayerID: playerID,
		Name:     playerName,
		Level:    playerLevel,
	})

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

	playerID, ok := h.parsePlayerID(req.Token)
	if !ok {
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{Code: errors.ErrTokenInvalid.Code})
		return
	}
	conn.BindPlayer(playerID)

	player, ge := h.playerSvc.Create(connCtx(conn), playerID, req.Name, req.Class)
	if ge != nil {
		conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{Code: ge.Code})
		return
	}

	// 创建后立即加载到内存（与登录路径一致），保证后续业务能通过 GetOnlinePlayer 找到该玩家
	if _, err := h.world.OnLogin(playerID); err != nil {
		logger.TWarn(connCtx(conn), "创建角色后加载到内存失败", "player_id", playerID, "err", err)
	}

	conn.Send(protocol.MsgIDCreatePlayerResp, &protocol.S2CCreatePlayerResp{
		Code:   errors.ErrSuccess.Code,
		Player: *toPlayerData(player),
	})
}

func (h *AuthHandler) parsePlayerID(token string) (uint64, bool) {
	if token == "" {
		return 0, false
	}
	claims, err := h.jwtMgr.ParseToken(token)
	if err != nil {
		logger.Warn("JWT解析失败", "err", err)
		return 0, false
	}
	return claims.PlayerID, true
}
