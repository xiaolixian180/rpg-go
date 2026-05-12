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

// DungeonHandler 地下城模块消息处理器
// 负责处理进入地下城、离开地下城和层间传送的网络消息
type DungeonHandler struct {
	dungeonSvc service.DungeonService // 地下城服务接口
	playerSvc  service.PlayerService  // 玩家服务接口（用于获取玩家数据）
}

// NewDungeonHandler 创建地下城模块处理器实例
func NewDungeonHandler(dungeonSvc service.DungeonService, playerSvc service.PlayerService) *DungeonHandler {
	return &DungeonHandler{dungeonSvc: dungeonSvc, playerSvc: playerSvc}
}

// HandleEnterDungeon 处理进入地下城请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（目标层数）
//  2. 获取玩家数据，调用 DungeonService.Enter 执行进入逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送进入地下城响应（含场景数据）
func (h *DungeonHandler) HandleEnterDungeon(conn *gateway.Conn, body []byte) {
	// 反序列化进入地下城请求
	var req protocol.C2SEnterDungeon
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用地下城服务执行进入逻辑
	dl, ge := h.dungeonSvc.Enter(context.Background(), conn.PlayerID, req.Layer, player.MaxLayer, 30)
	if ge != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: ge.Code})
		return
	}

	// 将 DungeonLayer 转换为协议响应并发送
	conn.Send(protocol.MsgIDEnterDungeonResp, toDungeonResp(dl, req.Layer, errors.ErrSuccess.Code))
}

// HandleLeaveDungeon 处理离开地下城请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体
//  2. 获取玩家数据，调用 DungeonService.Leave 执行离开逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送离开地下城响应
func (h *DungeonHandler) HandleLeaveDungeon(conn *gateway.Conn, body []byte) {
	// 反序列化离开地下城请求（无额外参数，仅校验格式）
	var req protocol.C2SLeaveDungeon
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用地下城服务执行离开逻辑
	ge := h.dungeonSvc.Leave(context.Background(), player)
	if ge != nil {
		conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{Code: ge.Code})
		return
	}

	// 离开成功
	conn.Send(protocol.MsgIDLeaveDungeonResp, &protocol.S2CLeaveDungeonResp{
		Code: errors.ErrSuccess.Code,
	})
}

// HandleLayerTeleport 处理层间传送请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（目标层数）
//  2. 获取玩家数据，调用 DungeonService.Teleport 执行传送逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送传送结果（含目标层场景数据）
func (h *DungeonHandler) HandleLayerTeleport(conn *gateway.Conn, body []byte) {
	// 反序列化层间传送请求
	var req protocol.C2SLayerTeleport
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用地下城服务执行传送逻辑
	dl, ge := h.dungeonSvc.Teleport(context.Background(), conn.PlayerID, req.TargetLayer, player.MaxLayer, 30)
	if ge != nil {
		conn.Send(protocol.MsgIDEnterDungeonResp, &protocol.S2CEnterDungeonResp{Code: ge.Code})
		return
	}

	// 将 DungeonLayer 转换为协议响应并发送
	conn.Send(protocol.MsgIDEnterDungeonResp, toDungeonResp(dl, req.TargetLayer, errors.ErrSuccess.Code))
}

// toDungeonResp 将 model.DungeonLayer 转换为协议层 S2CEnterDungeonResp
// 遍历场景中的怪物、玩家、资源，提取网络传输所需的字段
func toDungeonResp(dl *model.DungeonLayer, layer int32, code uint32) *protocol.S2CEnterDungeonResp {
	resp := &protocol.S2CEnterDungeonResp{
		Code:  code,
		Layer: layer,
		Zone:  model.GetZoneByLayer(layer),
	}
	if dl == nil {
		return resp
	}
	// 收集怪物数据（Monster 无读写锁，直接读取字段）
	for _, m := range dl.Monsters {
		resp.Monsters = append(resp.Monsters, protocol.MonsterData{
			ID: m.ID, Name: m.Name, Hp: m.Hp, MaxHp: m.MaxHp, X: m.X, Y: m.Y,
		})
	}
	// 收集玩家数据（Player 有读写锁，需加读锁保护）
	for _, pl := range dl.Players {
		pl.Mu().RLock()
		resp.Players = append(resp.Players, protocol.PlayerBrief{
			ID: pl.ID, Name: pl.Name, Class: pl.Class, Level: pl.Level,
			Hp: pl.Hp, MaxHp: pl.MaxHp, X: pl.X, Y: pl.Y,
		})
		pl.Mu().RUnlock()
	}
	// 收集资源数据（Resource 无读写锁，直接读取字段）
	for _, r := range dl.Resources {
		resp.Resources = append(resp.Resources, protocol.ResourceData{
			ID: r.ID, Type: r.Type, Name: r.Name, X: r.X, Y: r.Y, Harvested: r.Harvested,
		})
	}
	return resp
}
