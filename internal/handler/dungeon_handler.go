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

	// Boss层处理：检查是否需要生成Boss，并通知客户端
	if model.IsBossLayer(req.Layer) {
		h.handleBossLayer(conn, req.Layer)
	}
}

// handleBossLayer 处理进入Boss层：检查并生成Boss，广播Boss出现
func (h *DungeonHandler) handleBossLayer(conn *gateway.Conn, layer int32) {
	// 尝试重新生成Boss（如果冷却已过）
	newBoss := h.world.SpawnBossIfNeeded(layer)

	// 获取当前层的Boss（可能是刚生成的，也可能是已存在的）
	boss := h.world.GetLayerBoss(layer)
	if boss == nil {
		return
	}

	// 向当前层所有玩家广播Boss出现
	bossData := &protocol.S2CBossSpawn{
		BossID: boss.ID,
		Name:   boss.Name,
		Hp:     boss.Hp,
		MaxHp:  boss.MaxHp,
		Layer:  boss.Layer,
		X:      boss.X,
		Y:      boss.Y,
	}
	for _, s := range boss.Skills {
		bossData.Skills = append(bossData.Skills, protocol.BossSkillData{
			SkillID: s.SkillID, Name: s.Name, CD: s.CD, Range: s.Range,
		})
	}

	if newBoss != nil {
		// 新生成的Boss，广播给当前层所有玩家
		h.world.Hub().BroadcastToPlayers(h.world.LayerPlayerIDs(layer), protocol.MsgIDBossSpawn, bossData)
	} else {
		// Boss已存在，只发给刚进入的玩家
		conn.Send(protocol.MsgIDBossSpawn, bossData)
	}
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
	if layer <= 0 {
		return
	}
	h.world.Hub().BroadcastToPlayers(h.world.LayerPlayerIDs(layer), protocol.MsgIDPlayerMove, &protocol.S2CPlayerMove{
		PlayerID: playerID, X: req.X, Y: req.Y,
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
		m.Mu().RLock()
		dead := m.Dead
		m.Mu().RUnlock()
		if dead {
			continue // 跳过已死亡的怪物
		}
		resp.Monsters = append(resp.Monsters, protocol.MonsterData{
			ID: m.ID, Name: m.Name, Hp: m.Hp, MaxHp: m.MaxHp, X: m.X, Y: m.Y, Elite: m.Elite,
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
