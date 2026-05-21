package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/equip"
	"hero-quest/pkg/errors"
)

// EquipHandler 装备模块消息处理器
// 负责处理装备强化、附魔、穿戴、卸下和锻造合成的网络消息
type EquipHandler struct {
	world    service.World      // 游戏世界（获取在线玩家）
	equipSvc equip.EquipService // 装备服务接口
}

// NewEquipHandler 创建装备模块处理器实例
func NewEquipHandler(world service.World, equipSvc equip.EquipService) *EquipHandler {
	return &EquipHandler{world: world, equipSvc: equipSvc}
}

// HandleStrengthen 处理装备强化请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（槽位）
//  2. 调用 EquipService.Strengthen 执行强化逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送强化结果（新等级、消耗金币、是否成功）
func (h *EquipHandler) HandleStrengthen(conn *gateway.Conn, body []byte) {
	// 反序列化装备强化请求
	var req protocol.C2SEquipStrengthen
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取在线玩家实例
	p := h.world.GetOnlinePlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{Code: errors.ErrNotLogin.Code})
		return
	}

	// 调用装备服务执行强化逻辑
	sr, ge := h.equipSvc.Strengthen(connCtx(conn), p, req.Slot)
	if ge != nil {
		conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{Code: ge.Code})
		return
	}

	// 强化完成，发送强化结果
	conn.Send(protocol.MsgIDEquipStrengthenResp, &protocol.S2CEquipStrengthenResp{
		Code:      errors.ErrSuccess.Code,
		Slot:      sr.Slot,
		NewLevel:  sr.NewLevel,
		CostGold:  sr.CostGold,
		IsSuccess: sr.IsSuccess,
	})
}

// HandleEnchant 处理装备附魔请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（槽位+材料ID）
//  2. 调用 EquipService.Enchant 执行附魔逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送附魔结果（附魔属性名和值）
func (h *EquipHandler) HandleEnchant(conn *gateway.Conn, body []byte) {
	// 反序列化装备附魔请求
	var req protocol.C2SEquipEnchant
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEquipEnchantResp, &protocol.S2CEquipEnchantResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用装备服务执行附魔逻辑
	er, ge := h.equipSvc.Enchant(connCtx(conn), conn.PlayerID, req.Slot, req.MaterialID)
	if ge != nil {
		conn.Send(protocol.MsgIDEquipEnchantResp, &protocol.S2CEquipEnchantResp{Code: ge.Code})
		return
	}

	// 附魔成功，发送附魔结果
	conn.Send(protocol.MsgIDEquipEnchantResp, &protocol.S2CEquipEnchantResp{
		Code:     errors.ErrSuccess.Code,
		Slot:     er.Slot,
		AttrName: er.AttrName,
		AttrVal:  er.AttrVal,
	})
}

// HandleWear 处理穿戴装备请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（槽位+装备ID）
//  2. 调用 EquipService.Wear 执行穿戴逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送穿戴结果
func (h *EquipHandler) HandleWear(conn *gateway.Conn, body []byte) {
	// 反序列化穿戴装备请求
	var req protocol.C2SEquipWear
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取在线玩家实例
	p := h.world.GetOnlinePlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{Code: errors.ErrNotLogin.Code})
		return
	}

	// 调用装备服务执行穿戴逻辑
	ge := h.equipSvc.Wear(connCtx(conn), p, req.Slot, req.EquipID)
	if ge != nil {
		conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{Code: ge.Code})
		return
	}

	// 穿戴成功
	conn.Send(protocol.MsgIDEquipWearResp, &protocol.S2CEquipWearResp{
		Code: errors.ErrSuccess.Code,
		Slot: req.Slot,
	})
}

// HandleUnload 处理卸下装备请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（槽位）
//  2. 调用 EquipService.Unload 执行卸下逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送卸下结果
func (h *EquipHandler) HandleUnload(conn *gateway.Conn, body []byte) {
	// 反序列化卸下装备请求
	var req protocol.C2SEquipUnload
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDEquipUnloadResp, &protocol.S2CEquipUnloadResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用装备服务执行卸下逻辑
	ge := h.equipSvc.Unload(connCtx(conn), conn.PlayerID, req.Slot)
	if ge != nil {
		conn.Send(protocol.MsgIDEquipUnloadResp, &protocol.S2CEquipUnloadResp{Code: ge.Code})
		return
	}

	// 卸下成功
	conn.Send(protocol.MsgIDEquipUnloadResp, &protocol.S2CEquipUnloadResp{
		Code: errors.ErrSuccess.Code,
		Slot: req.Slot,
	})
}

// HandleForge 处理锻造合成请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（图纸ID+材料ID列表）
//  2. 调用 EquipService.Forge 执行锻造逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送锻造结果（产出装备ID、名称、品质）
func (h *EquipHandler) HandleForge(conn *gateway.Conn, body []byte) {
	// 反序列化锻造合成请求
	var req protocol.C2SForge
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDForgeResp, &protocol.S2CForgeResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取在线玩家实例
	p := h.world.GetOnlinePlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDForgeResp, &protocol.S2CForgeResp{Code: errors.ErrNotLogin.Code})
		return
	}

	// 调用装备服务执行锻造逻辑
	fr, ge := h.equipSvc.Forge(connCtx(conn), p, req.RecipeID, req.Materials)
	if ge != nil {
		conn.Send(protocol.MsgIDForgeResp, &protocol.S2CForgeResp{Code: ge.Code})
		return
	}

	// 锻造成功，发送锻造结果
	conn.Send(protocol.MsgIDForgeResp, &protocol.S2CForgeResp{
		Code:       errors.ErrSuccess.Code,
		ResultID:   fr.ResultID,
		ResultName: fr.ResultName,
		Quality:    fr.Quality,
	})
}
