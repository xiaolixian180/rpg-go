package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/skill"
	"hero-quest/pkg/errors"
)

// SkillHandler 技能模块消息处理器
type SkillHandler struct {
	world    service.World
	skillSvc skill.SkillService
}

// NewSkillHandler 创建技能模块处理器实例
func NewSkillHandler(world service.World, skillSvc skill.SkillService) *SkillHandler {
	return &SkillHandler{world: world, skillSvc: skillSvc}
}

// HandleSkillLevelUp 处理技能升级请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（技能ID）
//  2. 调用 SkillService.LevelUp 执行技能升级逻辑（每次消耗1点技能点）
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送升级结果（新等级）
func (h *SkillHandler) HandleSkillLevelUp(conn *gateway.Conn, body []byte) {
	// 反序列化技能升级请求
	var req protocol.C2SSkillLevelUp
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2CSkillLevelUpResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取在线玩家
	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	// 调用技能服务执行升级逻辑（每次消耗1点技能点）
	lr, ge := h.skillSvc.LevelUp(connCtx(conn), player, req.SkillID)
	if ge != nil {
		conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2CSkillLevelUpResp{Code: ge.Code})
		return
	}

	// 升级成功，发送新等级
	conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2CSkillLevelUpResp{
		Code:     errors.ErrSuccess.Code,
		SkillID:  lr.SkillID,
		NewLevel: lr.NewLevel,
	})
}

// HandleSkillReset 处理技能重置请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（无额外参数）
//  2. 调用 SkillService.Reset 执行技能重置逻辑（消耗金币，返还技能点）
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送重置结果（返还的技能点数）
func (h *SkillHandler) HandleSkillReset(conn *gateway.Conn, body []byte) {
	// 反序列化技能重置请求（无额外参数，仅校验格式）
	var req protocol.C2SSkillReset
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2CSkillResetResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取在线玩家
	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	// 调用技能服务执行重置逻辑
	rr, ge := h.skillSvc.Reset(connCtx(conn), player, h.world.Config().SkillResetCost)
	if ge != nil {
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2CSkillResetResp{Code: ge.Code})
		return
	}

	// 重置成功，发送返还的技能点数
	conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2CSkillResetResp{
		Code:         errors.ErrSuccess.Code,
		RefundPoints: rr.RefundPoints,
	})
}
