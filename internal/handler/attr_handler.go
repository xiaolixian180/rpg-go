package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// AttrHandler 属性分配模块消息处理器
// 负责处理属性点分配的网络消息
type AttrHandler struct {
	playerSvc service.PlayerService // 玩家服务接口（提供属性分配功能）
}

// NewAttrHandler 创建属性分配模块处理器实例
func NewAttrHandler(playerSvc service.PlayerService) *AttrHandler {
	return &AttrHandler{playerSvc: playerSvc}
}

// HandleAttrAssign 处理属性点分配请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（属性名+分配点数）
//  2. 调用 PlayerService.AssignAttr 执行属性分配逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送分配结果（剩余属性点）
func (h *AttrHandler) HandleAttrAssign(conn *gateway.Conn, body []byte) {
	// 反序列化属性分配请求
	var req protocol.C2SAttrAssign
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用玩家服务执行属性分配逻辑
	ge := h.playerSvc.AssignAttr(context.Background(), conn.PlayerID, req.Attr, req.Val)
	if ge != nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{Code: ge.Code})
		return
	}

	// 分配成功，需要重新获取玩家数据以返回剩余属性点
	player, ge := h.playerSvc.GetPlayer(context.Background(), conn.PlayerID)
	if ge != nil || player == nil {
		// 分配成功但查询剩余属性点失败，仍返回成功（不回滚分配）
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
			Code: errors.ErrSuccess.Code,
			Attr: req.Attr,
			Val:  req.Val,
		})
		return
	}

	// 返回完整结果，包含剩余属性点
	conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
		Code:       errors.ErrSuccess.Code,
		Attr:       req.Attr,
		Val:        req.Val,
		AttrPoints: player.AttrPoints,
	})
}