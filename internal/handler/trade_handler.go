package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/trade"
	"hero-quest/pkg/errors"
)

// TradeHandler 交易模块消息处理器
type TradeHandler struct {
	world    service.World      // 游戏世界（获取在线玩家）
	tradeSvc trade.TradeService // 交易服务接口
}

// NewTradeHandler 创建交易模块处理器实例
func NewTradeHandler(world service.World, tradeSvc trade.TradeService) *TradeHandler {
	return &TradeHandler{world: world, tradeSvc: tradeSvc}
}

// HandleTradeList 处理交易列表查询请求
func (h *TradeHandler) HandleTradeList(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	tr, ge := h.tradeSvc.List(connCtx(conn), req.Category, req.Page)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{
		Code: errors.ErrSuccess.Code, Items: toTradeItemList(tr.Items),
	})
}

// HandleTradePublish 处理发布交易请求
func (h *TradeHandler) HandleTradePublish(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradePublish
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	pr, ge := h.tradeSvc.Publish(connCtx(conn), conn.PlayerID, req.Slot, req.Price)
	if ge != nil {
		conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{
		Code: errors.ErrSuccess.Code, OrderID: pr.OrderID,
	})
}

// HandleTradeBuy 处理购买交易请求
func (h *TradeHandler) HandleTradeBuy(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeBuy
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	buyer := onlinePlayer(h.world, conn)
	if buyer == nil {
		return
	}

	br, ge := h.tradeSvc.Buy(connCtx(conn), buyer, req.OrderID)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{
		Code: errors.ErrSuccess.Code, OrderID: br.OrderID,
	})
}

// HandleTradeCancel 处理取消交易请求
func (h *TradeHandler) HandleTradeCancel(conn *gateway.Conn, body []byte) {
	var req protocol.C2STradeCancel
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	ge := h.tradeSvc.Cancel(connCtx(conn), conn.PlayerID, req.OrderID)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{
		Code: errors.ErrSuccess.Code, OrderID: req.OrderID,
	})
}
