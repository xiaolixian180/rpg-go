package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// TradeHandler 交易模块消息处理器
// 负责处理交易列表查询、发布交易、购买交易和取消交易的网络消息
type TradeHandler struct {
	tradeSvc  service.TradeService  // 交易服务接口
	playerSvc service.PlayerService // 玩家服务接口（用于获取玩家数据）
}

// NewTradeHandler 创建交易模块处理器实例
func NewTradeHandler(tradeSvc service.TradeService, playerSvc service.PlayerService) *TradeHandler {
	return &TradeHandler{tradeSvc: tradeSvc, playerSvc: playerSvc}
}

// HandleTradeList 处理交易列表查询请求
// 流程：
//  1. 反序列化请求体（分类+页码）
//  2. 调用 TradeService.List 执行查询逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送交易列表数据
func (h *TradeHandler) HandleTradeList(conn *gateway.Conn, body []byte) {
	// 反序列化交易列表请求
	var req protocol.C2STradeList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用交易服务查询交易列表
	tr, ge := h.tradeSvc.List(context.Background(), req.Category, req.Page)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{Code: ge.Code})
		return
	}

	// 查询成功，将 TradeListResult 转换为协议层响应并发送
	conn.Send(protocol.MsgIDTradeListResp, &protocol.S2CTradeListResp{
		Code:  errors.ErrSuccess.Code,
		Items: toTradeItemList(tr.Items),
	})
}

// HandleTradePublish 处理发布交易请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（背包槽位+售价）
//  2. 调用 TradeService.Publish 执行发布逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送发布结果（订单ID）
func (h *TradeHandler) HandleTradePublish(conn *gateway.Conn, body []byte) {
	// 反序列化发布交易请求
	var req protocol.C2STradePublish
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用交易服务执行发布逻辑
	pr, ge := h.tradeSvc.Publish(context.Background(), conn.PlayerID, req.Slot, req.Price)
	if ge != nil {
		conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{Code: ge.Code})
		return
	}

	// 发布成功，发送订单ID
	conn.Send(protocol.MsgIDTradePublishResp, &protocol.S2CTradePublishResp{
		Code:    errors.ErrSuccess.Code,
		OrderID: pr.OrderID,
	})
}

// HandleTradeBuy 处理购买交易请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（订单ID）
//  2. 获取买家玩家数据，调用 TradeService.Buy 执行购买逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送购买结果
func (h *TradeHandler) HandleTradeBuy(conn *gateway.Conn, body []byte) {
	// 反序列化购买交易请求
	var req protocol.C2STradeBuy
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取买家玩家数据（购买需扣除买家金币）
	buyer := getPlayer(conn, h.playerSvc)
	if buyer == nil {
		return
	}

	// 调用交易服务执行购买逻辑
	br, ge := h.tradeSvc.Buy(context.Background(), buyer, req.OrderID)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{Code: ge.Code})
		return
	}

	// 购买成功，发送购买结果
	conn.Send(protocol.MsgIDTradeBuyResp, &protocol.S2CTradeBuyResp{
		Code:    errors.ErrSuccess.Code,
		OrderID: br.OrderID,
	})
}

// HandleTradeCancel 处理取消交易请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（订单ID）
//  2. 调用 TradeService.Cancel 执行取消逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送取消结果
func (h *TradeHandler) HandleTradeCancel(conn *gateway.Conn, body []byte) {
	// 反序列化取消交易请求
	var req protocol.C2STradeCancel
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用交易服务执行取消逻辑
	ge := h.tradeSvc.Cancel(context.Background(), conn.PlayerID, req.OrderID)
	if ge != nil {
		conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{Code: ge.Code})
		return
	}

	// 取消成功
	conn.Send(protocol.MsgIDTradeCancelResp, &protocol.S2CTradeCancelResp{
		Code:    errors.ErrSuccess.Code,
		OrderID: req.OrderID,
	})
}