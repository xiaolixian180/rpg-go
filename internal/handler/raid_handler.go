package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/raid"
	"hero-quest/pkg/errors"
)

// RaidHandler 战局模块消息处理器
type RaidHandler struct {
	world   service.World    // 游戏世界（获取在线玩家）
	raidSvc raid.RaidService // 战局服务接口
}

// NewRaidHandler 创建战局模块处理器实例
func NewRaidHandler(world service.World, raidSvc raid.RaidService) *RaidHandler {
	return &RaidHandler{world: world, raidSvc: raidSvc}
}

// HandleEnterRaid 处理进入战局请求
func (h *RaidHandler) HandleEnterRaid(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidEnter
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidEnterResp, &protocol.S2CRaidEnterResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ri, ge := h.raidSvc.StartRaid(connCtx(conn), player, req.MapID)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidEnterResp, &protocol.S2CRaidEnterResp{Code: ge.Code})
		return
	}

	// 转换怪物列表为协议格式
	ri.Mu().RLock()
	monsters := make([]protocol.MonsterData, 0, len(ri.Monsters))
	for _, m := range ri.Monsters {
		m.Mu().RLock()
		monsters = append(monsters, protocol.MonsterData{
			ID:    m.ID,
			Name:  m.Name,
			Hp:    m.Hp,
			MaxHp: m.MaxHp,
			X:     m.X,
			Y:     m.Y,
			Elite: m.Elite,
		})
		m.Mu().RUnlock()
	}

	// 转换区域列表
	zones := make([]protocol.RaidZoneData, len(ri.Zones))
	for i, z := range ri.Zones {
		zones[i] = protocol.RaidZoneData{
			ID:         z.ID,
			Name:       z.Name,
			X1:         z.X1,
			Y1:         z.Y1,
			X2:         z.X2,
			Y2:         z.Y2,
			PvPEnabled: z.PvPEnabled,
			LootTier:   z.LootTier,
		}
	}

	// 转换撤离点列表
	extractPoints := make([]protocol.ExtractionPointData, len(ri.ExtractionPoints))
	for i, ep := range ri.ExtractionPoints {
		extractPoints[i] = protocol.ExtractionPointData{
			ID:              ep.ID,
			X:               ep.X,
			Y:               ep.Y,
			Radius:          ep.Radius,
			ExtractDuration: ep.ExtractDuration,
		}
	}

	// 转换战利品容器列表
	lootContainers := make([]protocol.LootContainerData, 0, len(ri.LootContainers))
	for _, c := range ri.LootContainers {
		c.Mu().Lock()
		lootContainers = append(lootContainers, protocol.LootContainerData{
			ID:     c.ID,
			X:      c.X,
			Y:      c.Y,
			Opened: c.Opened,
		})
		c.Mu().Unlock()
	}
	ri.Mu().RUnlock()

	resp := &protocol.S2CRaidEnterResp{
		Code:             errors.ErrSuccess.Code,
		MapID:            ri.ID,
		MapName:          ri.Name,
		Duration:         int64(ri.Duration.Seconds()),
		Monsters:         monsters,
		Zones:            zones,
		ExtractionPoints: extractPoints,
		LootContainers:   lootContainers,
	}
	conn.Send(protocol.MsgIDRaidEnterResp, resp)
}

// HandleLeaveRaid 处理离开战局请求
func (h *RaidHandler) HandleLeaveRaid(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidLeave
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidLeaveResp, &protocol.S2CRaidLeaveResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ge := h.raidSvc.LeaveRaid(connCtx(conn), player)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidLeaveResp, &protocol.S2CRaidLeaveResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRaidLeaveResp, &protocol.S2CRaidLeaveResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleOpenLoot 处理打开战利品容器请求
func (h *RaidHandler) HandleOpenLoot(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidLootOpen
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidLootOpenResp, &protocol.S2CRaidLootOpenResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	items, ge := h.raidSvc.OpenLootContainer(connCtx(conn), player, req.ContainerID)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidLootOpenResp, &protocol.S2CRaidLootOpenResp{Code: ge.Code})
		return
	}

	// 转换物品列表为协议格式
	lootData := make([]protocol.RaidLootData, len(items))
	for i, item := range items {
		lootData[i] = protocol.RaidLootData{
			Index:   int32(i),
			ItemID:  item.ItemID,
			Count:   item.Count,
			Quality: item.Quality,
			Name:    item.Name,
		}
	}

	conn.Send(protocol.MsgIDRaidLootOpenResp, &protocol.S2CRaidLootOpenResp{
		Code:  errors.ErrSuccess.Code,
		Items: lootData,
	})
}

// HandlePickupLoot 处理拾取战利品请求
func (h *RaidHandler) HandlePickupLoot(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidLootPickup
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidLootPickupResp, &protocol.S2CRaidLootPickupResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ge := h.raidSvc.PickUpLoot(connCtx(conn), player, req.ItemIndex)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidLootPickupResp, &protocol.S2CRaidLootPickupResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRaidLootPickupResp, &protocol.S2CRaidLootPickupResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleDiscardLoot 处理丢弃战利品请求
func (h *RaidHandler) HandleDiscardLoot(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidLootDiscard
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidLootDiscardResp, &protocol.S2CRaidLootDiscardResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ge := h.raidSvc.DiscardLoot(connCtx(conn), player, req.ItemIndex)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidLootDiscardResp, &protocol.S2CRaidLootDiscardResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRaidLootDiscardResp, &protocol.S2CRaidLootDiscardResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleExtract 处理开始撤离请求
func (h *RaidHandler) HandleExtract(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidExtract
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	timer, ge := h.raidSvc.AttemptExtraction(connCtx(conn), player, req.PointID)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{
		Code:  errors.ErrSuccess.Code,
		Timer: timer,
	})
}

// HandleCancelExtract 处理取消撤离请求
func (h *RaidHandler) HandleCancelExtract(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidExtract
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	ge := h.raidSvc.CancelExtraction(connCtx(conn), player)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRaidExtractResp, &protocol.S2CRaidExtractResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleRaidPvpAttack 处理战局内PvP攻击请求
func (h *RaidHandler) HandleRaidPvpAttack(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidPvpAttack
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidPvpResult, &protocol.S2CRaidPvpResult{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	result, ge := h.raidSvc.RaidPvpAttack(connCtx(conn), player, req.TargetID)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidPvpResult, &protocol.S2CRaidPvpResult{Code: ge.Code})
		return
	}

	// 转换为协议格式
	conn.Send(protocol.MsgIDRaidPvpResult, &protocol.S2CRaidPvpResult{
		Code:       errors.ErrSuccess.Code,
		AttackerID: player.ID,
		TargetID:   result.TargetID,
		Damage:     result.Damage,
		CurrHp:     result.CurrHp,
		IsDead:     result.IsDead,
	})
}

// HandleRaidMapList 处理查询可用战局地图列表请求
func (h *RaidHandler) HandleRaidMapList(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRaidMapList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRaidMapListResp, &protocol.S2CRaidMapListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	templates, ge := h.raidSvc.ListAvailableMaps(connCtx(conn), player)
	if ge != nil {
		conn.Send(protocol.MsgIDRaidMapListResp, &protocol.S2CRaidMapListResp{Code: ge.Code})
		return
	}

	// 转换模板为协议格式
	maps := make([]protocol.RaidMapInfo, len(templates))
	for i, t := range templates {
		maps[i] = protocol.RaidMapInfo{
			TemplateID: t.TemplateID,
			Name:       t.Name,
			Duration:   int64(t.Duration.Seconds()),
			ZoneCount:  int32(len(t.Zones)),
			LootTier:   t.MaxLootTier,
		}
	}

	conn.Send(protocol.MsgIDRaidMapListResp, &protocol.S2CRaidMapListResp{
		Code: errors.ErrSuccess.Code,
		Maps: maps,
	})
}
