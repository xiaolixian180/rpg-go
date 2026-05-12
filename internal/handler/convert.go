package handler

import (
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
)

// toPlayerData 将 model.Player 转换为协议层 protocol.PlayerData，
// 用于网络传输。只传输持久化属性，不传输坐标、在线状态等运行时字段。
func toPlayerData(p *model.Player) *protocol.PlayerData {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return &protocol.PlayerData{
		ID:         p.ID,
		Name:       p.Name,
		Class:      p.Class,
		Level:      p.Level,
		Exp:        p.Exp,
		Gold:       p.Gold,
		Honor:      p.Honor,
		KillValue:  p.KillValue,
		Str:        p.Str,
		Agi:        p.Agi,
		Int:        p.Int,
		Con:        p.Con,
		AttrPoints: p.AttrPoints,
		MaxLayer:   p.MaxLayer,
	}
}

// toPetData 将 model.Pet 转换为协议层 protocol.PetData
func toPetData(p *model.Pet) *protocol.PetData {
	p.Mu().RLock()
	defer p.Mu().RUnlock()
	return &protocol.PetData{
		UID:     p.UID,
		PetID:   p.PetID,
		Name:    p.Name,
		Level:   p.Level,
		Quality: p.Quality,
		Type:    p.Type,
		Skills:  p.Skills,
	}
}

// toDropItemList 将 model.DropItem 列表转换为协议层 protocol.DropItem 列表
func toDropItemList(src []model.DropItem) []protocol.DropItem {
	result := make([]protocol.DropItem, len(src))
	for i, d := range src {
		result[i] = protocol.DropItem{
			ItemID:  d.ItemID,
			Name:    d.Name,
			Quality: d.Quality,
			Count:   d.Count,
		}
	}
	return result
}

// toTradeItemList 将 model.TradeOrder 列表转换为协议层 protocol.TradeItem 列表
func toTradeItemList(src []*model.TradeOrder) []protocol.TradeItem {
	result := make([]protocol.TradeItem, len(src))
	for i, o := range src {
		result[i] = protocol.TradeItem{
			OrderID:         o.ID,
			SellerID:        o.SellerID,
			EquipID:         o.EquipID,
			Quality:         o.Quality,
			StrengthenLevel: o.StrengthenLevel,
			Price:           o.Price,
		}
	}
	return result
}

// toShopItemList 将 model.ShopItem 列表转换为协议层 protocol.ShopItem 列表
func toShopItemList(src []*model.ShopItem) []protocol.ShopItem {
	result := make([]protocol.ShopItem, len(src))
	for i, s := range src {
		result[i] = protocol.ShopItem{
			ID:            s.ID,
			Name:          s.Name,
			Price:         s.Price,
			CurrencyType:  0, // model 中无此字段，默认金币
			Stock:         s.Stock,
			RequireLevel:  s.RequireLevel,
		}
	}
	return result
}

// toRankingItemList 将 model.RankingItem 列表转换为协议层 protocol.RankingItem 列表
func toRankingItemList(src []*model.RankingItem) []protocol.RankingItem {
	result := make([]protocol.RankingItem, len(src))
	for i, r := range src {
		result[i] = protocol.RankingItem{
			Rank:     r.Rank,
			PlayerID: r.PlayerID,
			Name:     r.Name,
			Value:    r.Value,
		}
	}
	return result
}