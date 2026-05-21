package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/shop"
	"hero-quest/pkg/errors"
)

// ShopHandler 商店模块消息处理器
type ShopHandler struct {
	world   service.World    // 游戏世界（获取在线玩家）
	shopSvc shop.ShopService // 商店服务接口
}

// NewShopHandler 创建商店模块处理器实例
func NewShopHandler(world service.World, shopSvc shop.ShopService) *ShopHandler {
	return &ShopHandler{world: world, shopSvc: shopSvc}
}

// HandleShopList 处理商店列表查询请求
func (h *ShopHandler) HandleShopList(conn *gateway.Conn, body []byte) {
	var req protocol.C2SShopList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDShopListResp, &protocol.S2CShopListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	items, ge := h.shopSvc.List(connCtx(conn), req.Type)
	if ge != nil {
		conn.Send(protocol.MsgIDShopListResp, &protocol.S2CShopListResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDShopListResp, &protocol.S2CShopListResp{
		Code: errors.ErrSuccess.Code, Items: toShopItemList(items),
	})
}

// HandleShopBuy 处理商店商品购买请求
func (h *ShopHandler) HandleShopBuy(conn *gateway.Conn, body []byte) {
	var req protocol.C2SShopBuy
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2CShopBuyResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	br, ge := h.shopSvc.Buy(connCtx(conn), player, req.ItemID, req.Count, req.CurrencyType)
	if ge != nil {
		conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2CShopBuyResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDShopBuyResp, &protocol.S2CShopBuyResp{
		Code: errors.ErrSuccess.Code, ItemID: br.ItemID, Count: br.Count,
	})
}
