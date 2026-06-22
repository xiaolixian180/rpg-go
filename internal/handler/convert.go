package handler

import (
	"fmt"

	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
)

// toPlayerData 将 model.Player 转换为协议层 protocol.PlayerData，
// 用于网络传输。只传输持久化属性，不传输坐标、在线状态等运行时字段。
func toPlayerData(p *model.Player) *protocol.PlayerData {
	p.Mu().RLock()
	defer p.Mu().RUnlock()

	// 序列化已装备的装备列表，附带模板基础属性供客户端展示
	var equipped []protocol.EquipmentData
	for _, eq := range p.EquippedItems {
		if eq == nil {
			continue
		}
		ed := protocol.EquipmentData{
			Slot:            eq.Slot,
			EquipID:         eq.EquipID,
			Quality:         eq.Quality,
			StrengthenLevel: eq.StrengthenLevel,
			EnchantAttr:     eq.EnchantAttr,
		}
		if tmpl, ok := model.EquipTemplates[eq.EquipID]; ok {
			ed.Name = tmpl.Name
			ed.BaseAtk = tmpl.BaseAtk
			ed.BaseDef = tmpl.BaseDef
			ed.BaseHp = tmpl.BaseHp
			ed.RequireLevel = tmpl.RequireLevel
			// 序列化技能特效
			if len(tmpl.SkillEffects) > 0 {
				ed.SkillEffects = make([]protocol.SkillEffectData, 0, len(tmpl.SkillEffects))
				for _, eff := range tmpl.SkillEffects {
					desc := formatSkillEffectDesc(eff)
					ed.SkillEffects = append(ed.SkillEffects, protocol.SkillEffectData{
						SkillID:    eff.SkillID,
						EffectType: eff.EffectType,
						Value:      eff.Value,
						Desc:       desc,
					})
				}
			}
		}
		equipped = append(equipped, ed)
	}

	return &protocol.PlayerData{
		ID:            p.ID,
		Name:          p.Name,
		Class:         p.Class,
		Level:         p.Level,
		Exp:           p.Exp,
		Gold:          p.Gold,
		Honor:         p.Honor,
		KillValue:     p.KillValue,
		Str:           p.Str,
		Agi:           p.Agi,
		Int:           p.Int,
		Con:           p.Con,
		Def:           p.Def,
		AttrPoints:    p.AttrPoints,
		MaxLayer:      p.MaxLayer,
		Hp:            p.Hp,
		MaxHp:         p.MaxHp,
		Mp:            p.Mp,
		MaxMp:         p.MaxMp,
		Items:         p.Items,
		EquippedItems: equipped,
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
			ID:           s.ID,
			Name:         s.Name,
			Price:        s.Price,
			CurrencyType: s.CurrencyType,
			Stock:        s.Stock,
			RequireLevel: s.RequireLevel,
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

// formatSkillEffectDesc 生成技能特效的中文描述
func formatSkillEffectDesc(eff model.SkillEffect) string {
	pct := int(eff.Value * 100)
	typeName := model.EffectTypeName[eff.EffectType]
	if typeName == "" {
		typeName = "未知"
	}
	if eff.SkillID == 0 {
		return fmt.Sprintf("所有技能%s+%d%%", typeName, pct)
	}
	if skillDef, ok := model.SkillDefs[eff.SkillID]; ok {
		return fmt.Sprintf("%s%s+%d%%", skillDef.Name, typeName, pct)
	}
	return fmt.Sprintf("技能%d%s+%d%%", eff.SkillID, typeName, pct)
}
