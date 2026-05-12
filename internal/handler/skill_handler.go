package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// SkillHandler 技能模块消息处理器
// 负责处理技能升级和技能重置的网络消息
type SkillHandler struct {
	skillSvc service.SkillService // 技能服务接口
}

// NewSkillHandler 创建技能模块处理器实例
func NewSkillHandler(skillSvc service.SkillService) *SkillHandler {
	return &SkillHandler{skillSvc: skillSvc}
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
		conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2SSkillLevelUpResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用技能服务执行升级逻辑（每次消耗1点技能点）
	lr, ge := h.skillSvc.LevelUp(context.Background(), conn.PlayerID, req.SkillID, 1)
	if ge != nil {
		conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2SSkillLevelUpResp{Code: ge.Code})
		return
	}

	// 升级成功，发送新等级
	conn.Send(protocol.MsgIDSkillLevelUpResp, &protocol.S2SSkillLevelUpResp{
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
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用技能服务执行重置逻辑（重置费用为1000金币，由调用方决定）
	rr, ge := h.skillSvc.Reset(context.Background(), conn.PlayerID, 1000)
	if ge != nil {
		conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{Code: ge.Code})
		return
	}

	// 重置成功，发送返还的技能点数
	conn.Send(protocol.MsgIDSkillResetResp, &protocol.S2SSkillResetResp{
		Code:         errors.ErrSuccess.Code,
		RefundPoints: rr.RefundPoints,
	})
}