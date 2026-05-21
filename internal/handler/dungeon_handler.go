package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/dungeon"
	"hero-quest/pkg/errors"
)

// DungeonHandler 地下城模块消息处理器
type DungeonHandler struct {
	world      service.World          // 游戏世界（获取在线玩家）
	dungeonSvc dungeon.DungeonService // 地下城服务接口
}

// NewDungeonHandler 创建地下城模块处理器实例
func NewDungeonHandler(world service.World, dungeonSvc dungeon.DungeonService) *DungeonHandler {
	return &DungeonHandler{world: world, dungeonSvc: dungeonSvc}
}

// HandleEnterDungeon 处理进入地下城请求
func (h *DungeonHandler) HandleEnterDungeon(conn *gateway.Conn, body []byte) {
	var req protocol.C2SEnterDungeon
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	dl, ge := h.dungeonSvc.Enter(connCtx(conn), player, req.Layer)
	if ge != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDEnterDungeonResp, toDungeonResp(dl, req.Layer, errors.ErrSuccess.Code))
}

// HandleLeaveDungeon 处理离开地下城请求
func (h *DungeonHandler) HandleLeaveDungeon(conn *gateway.Conn, body []byte) {
	var req protocol.C2SLeaveDungeon
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ge := h.dungeonSvc.Leave(connCtx(conn), player)
	if ge != nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleLayerTeleport 处理层间传送请求
func (h *DungeonHandler) HandleLayerTeleport(conn *gateway.Conn, body []byte) {
	var req protocol.C2SLayerTeleport
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDLayerTeleportResp, &protocol.S2CLayerTeleportResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	dl, ge := h.dungeonSvc.Teleport(connCtx(conn), player, req.TargetLayer)
	if ge != nil {
		conn.Send(protocol.MsgIDLayerTeleportResp, &protocol.S2CLayerTeleportResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDLayerTeleportResp, toDungeonResp(dl, req.TargetLayer, errors.ErrSuccess.Code))
}

// HandleMove 处理玩家移动请求，更新玩家坐标
func (h *DungeonHandler) HandleMove(conn *gateway.Conn, body []byte) {
	var req protocol.C2SMove
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	player.Mu().Lock()
	player.X = req.X
	player.Y = req.Y
	playerID := player.ID
	layer := player.Layer
	player.Mu().Unlock()
	h.world.Hub().BroadcastToLayer(layer, protocol.MsgIDPlayerMove, &protocol.S2CPlayerMove{
		PlayerID: playerID, X: req.X, Y: req.Y,
	}, func(pid uint64) int32 {
		p := h.world.GetOnlinePlayer(pid)
		if p == nil {
			return 0
		}
		p.Mu().RLock()
		l := p.Layer
		p.Mu().RUnlock()
		return l
	})
}

// toDungeonResp 将 model.DungeonLayer 转换为协议层 S2CEnterDungeonResp
func toDungeonResp(dl *model.DungeonLayer, layer int32, code uint32) *protocol.S2CEnterDungeonResp {
	resp := &protocol.S2CEnterDungeonResp{
		Code:  code,
		Layer: layer,
		Zone:  model.GetZoneByLayer(layer),
	}
	if dl == nil {
		return resp
	}
	for _, m := range dl.Monsters {
		resp.Monsters = append(resp.Monsters, protocol.MonsterData{
			ID: m.ID, Name: m.Name, Hp: m.Hp, MaxHp: m.MaxHp, X: m.X, Y: m.Y,
		})
	}
	for _, pl := range dl.Players {
		pl.Mu().RLock()
		resp.Players = append(resp.Players, protocol.PlayerBrief{
			ID: pl.ID, Name: pl.Name, Class: pl.Class, Level: pl.Level,
			Hp: pl.Hp, MaxHp: pl.MaxHp, X: pl.X, Y: pl.Y,
		})
		pl.Mu().RUnlock()
	}
	for _, r := range dl.Resources {
		resp.Resources = append(resp.Resources, protocol.ResourceData{
			ID: r.ID, Type: r.Type, Name: r.Name, X: r.X, Y: r.Y, Harvested: r.Harvested,
		})
	}
	return resp
}
