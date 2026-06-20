package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// UseItemHandler 消耗品使用模块处理器
type UseItemHandler struct {
	world service.World
}

// NewUseItemHandler 创建消耗品使用处理器实例
func NewUseItemHandler(world service.World) *UseItemHandler {
	return &UseItemHandler{world: world}
}

// HandleUseItem 处理使用消耗品请求
func (h *UseItemHandler) HandleUseItem(conn *gateway.Conn, body []byte) {
	var req protocol.C2SUseItem
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDUseItemResp, &protocol.S2CUseItemResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	p := onlinePlayer(h.world, conn)
	if p == nil {
		return
	}

	p.Mu().Lock()
	defer p.Mu().Unlock()

	// 验证物品模板存在
	tmpl, ok := model.ItemTemplates[req.ItemId]
	if !ok {
		conn.Send(protocol.MsgIDUseItemResp, &protocol.S2CUseItemResp{
			Code:   errors.ErrItemInvalid.Code,
			ItemId: req.ItemId,
		})
		return
	}

	// 验证库存
	count := p.Items[req.ItemId]
	if count <= 0 {
		conn.Send(protocol.MsgIDUseItemResp, &protocol.S2CUseItemResp{
			Code:   errors.ErrItemNoStock.Code,
			ItemId: req.ItemId,
		})
		return
	}

	// 扣除物品
	p.Items[req.ItemId] = count - 1

	// 应用效果
	if tmpl.HPVal > 0 {
		p.Hp += tmpl.HPVal
		if p.Hp > p.MaxHp {
			p.Hp = p.MaxHp
		}
	}
	if tmpl.MPVal > 0 {
		p.Mp += tmpl.MPVal
		if p.Mp > p.MaxMp {
			p.Mp = p.MaxMp
		}
	}

	conn.Send(protocol.MsgIDUseItemResp, &protocol.S2CUseItemResp{
		Code:   errors.ErrSuccess.Code,
		ItemId: req.ItemId,
		Count:  p.Items[req.ItemId],
		Hp:     p.Hp,
		MaxHp:  p.MaxHp,
		Mp:     p.Mp,
		MaxMp:  p.MaxMp,
	})
}
