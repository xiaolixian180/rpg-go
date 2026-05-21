package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/player"
	"hero-quest/pkg/errors"
)

// AttrHandler 属性分配模块消息处理器
type AttrHandler struct {
	world     service.World        // 游戏世界（获取在线玩家）
	playerSvc player.PlayerService // 玩家服务（属性分配需要持久化）
}

// NewAttrHandler 创建属性分配模块处理器实例
func NewAttrHandler(world service.World, playerSvc player.PlayerService) *AttrHandler {
	return &AttrHandler{world: world, playerSvc: playerSvc}
}

// HandleAttrAssign 处理属性点分配请求
func (h *AttrHandler) HandleAttrAssign(conn *gateway.Conn, body []byte) {
	var req protocol.C2SAttrAssign
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 从内存获取在线玩家实例（AssignAttr 直接操作内存对象，避免缓存快照脏写）
	p := h.world.GetOnlinePlayer(conn.PlayerID)
	if p == nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{Code: errors.ErrNotLogin.Code})
		return
	}

	ge := h.playerSvc.AssignAttr(connCtx(conn), p, req.Attr, req.Val)
	if ge != nil {
		conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{Code: ge.Code})
		return
	}

	// 从内存获取最新的属性点
	attrPoints := int32(0)
	p.Mu().RLock()
	attrPoints = p.AttrPoints
	p.Mu().RUnlock()

	conn.Send(protocol.MsgIDAttrAssignResp, &protocol.S2CAttrAssignResp{
		Code:       errors.ErrSuccess.Code,
		Attr:       req.Attr,
		Val:        req.Val,
		AttrPoints: attrPoints,
	})
}
