package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// ShopHandler 商店模块消息处理器
// 负责处理商店列表查询和商品购买的网络消息
type ShopHandler struct {
	shopSvc   service.ShopService   // 商店服务接口
	playerSvc service.PlayerService // 玩家服务接口（用于获取玩家数据）
}

// NewShopHandler 创建商店模块处理器实例
func NewShopHandler(shopSvc service.ShopService, playerSvc service.PlayerService) *ShopHandler {
	return &ShopHandler{shopSvc: shopSvc, playerSvc: playerSvc}
}

// HandleShopList 处理商店列表查询请求
// 流程：
//  1. 反序列化请求体（商店类型）
//  2. 调用 ShopService.List 执行查询逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送商店列表数据
func (h *ShopHandler) HandleShopList(conn *gateway.Conn, body []byte) {
	// 反序列化商店列表请求
	var req protocol.C2SShopList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDShopListResp, &protocol.S2SShopListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用商店服务查询商品列表
	items, ge := h.shopSvc.List(context.Background(), req.Type)
	if ge != nil {
		conn.Send(protocol.MsgIDShopListResp, &protocol.S2SShopListResp{Code: ge.Code})
		return
	}

	// 查询成功，将 model.ShopItem 列表转换为协议层 ShopItem 列表并发送
	conn.Send(protocol.MsgIDShopListResp, &protocol.S2SShopListResp{
		Code:  errors.ErrSuccess.Code,
		Items: toShopItemList(items),
	})
}

// HandleShopBuy 处理商店商品购买请求
// 流程：
//  1. 反序列化请求体（商品ID+购买数量）
//  2. 获取玩家数据，调用 ShopService.Buy 执行购买逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送购买结果
func (h *ShopHandler) HandleShopBuy(conn *gateway.Conn, body []byte) {
	// 反序列化商店购买请求
	var req protocol.C2SShopBuy
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2SShopBuyResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取玩家数据（购买需要扣除玩家金币）
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用商店服务执行购买逻辑（货币类型默认为金币，通过商店类型推断）
	br, ge := h.shopSvc.Buy(context.Background(), player, req.ItemID, req.Count, 0)
	if ge != nil {
		conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2SShopBuyResp{Code: ge.Code})
		return
	}

	// 购买成功，发送购买结果
	conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2SShopBuyResp{
		Code:   errors.ErrSuccess.Code,
		ItemID: br.ItemID,
		Count:  br.Count,
	})
}