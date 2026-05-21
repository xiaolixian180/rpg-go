// Package shop - 商店服务
// 提供商品列表查询、购买等商店相关业务逻辑
package shop

import (
	"context"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 商店服务接口 ====================

// ShopService 商店服务接口，定义商品列表查询和购买操作
type ShopService interface {
	// List 查询商店商品列表（按商店类型）
	List(ctx context.Context, shopType int32) ([]*model.ShopItem, *errors.GameError)
	// Buy 购买商店商品，校验金币/荣誉、库存、等级等条件，并通过InventoryRepo发放物品
	Buy(ctx context.Context, player *model.Player, itemID uint64, count int32, currencyType int32) (*ShopBuyResult, *errors.GameError)
}

// ==================== 商店购买结果结构体 ====================

// ShopBuyResult 商店购买结果
type ShopBuyResult struct {
	ItemID uint64 // 商品ID
	Count  int32  // 购买数量
}

// ==================== 商店服务实现 ====================

// shopService 商店服务实现
type shopService struct {
	shopRepo      repo.ShopRepo      // 商店数据访问接口
	inventoryRepo repo.InventoryRepo // 背包数据访问接口（发放物品）
}

// NewShopService 创建商店服务实例（需要 InventoryRepo 依赖用于发放物品）
func NewShopService(shopRepo repo.ShopRepo, inventoryRepo repo.InventoryRepo) ShopService {
	return &shopService{
		shopRepo:      shopRepo,
		inventoryRepo: inventoryRepo,
	}
}

// List 查询商店商品列表：
//
//	根据商店类型（普通/荣誉/公会）查询对应的商品列表
func (s *shopService) List(ctx context.Context, shopType int32) ([]*model.ShopItem, *errors.GameError) {
	items, err := s.shopRepo.ListShopItems(ctx, shopType)
	if err != nil {
		logger.TError(ctx, "查询商店商品列表失败", "shop_type", shopType, "err", err)
		return nil, errors.ErrInternal
	}
	return items, nil
}

// Buy 购买商店商品业务逻辑（修复：校验货币类型与商品匹配，扣减库存后通过InventoryRepo发放物品）：
//  1. 查询商品信息
//  2. 校验商品是否存在
//  3. 校验货币类型是否与商品一致
//  4. 校验库存是否充足（-1表示无限库存）
//  5. 校验玩家等级是否满足购买条件
//  6. 根据货币类型校验金币/荣誉是否充足
//  7. 扣减货币
//  8. 扣减库存
//  9. 通过InventoryRepo发放物品到玩家背包
func (s *shopService) Buy(ctx context.Context, player *model.Player, itemID uint64, count int32, currencyType int32) (*ShopBuyResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	// 查询商品信息
	item, err := s.shopRepo.GetShopItem(ctx, itemID)
	if err != nil || item == nil {
		return nil, errors.ErrShopNotFound
	}

	// 校验货币类型是否与商品一致
	if item.CurrencyType != currencyType {
		return nil, errors.ErrParamInvalid
	}

	// 校验库存是否充足（-1表示无限库存）
	if item.Stock != -1 && item.Stock < count {
		return nil, errors.ErrShopStockOut
	}

	// 校验玩家等级是否满足购买条件
	player.Mu().RLock()
	playerLevel := player.Level
	player.Mu().RUnlock()
	if playerLevel < item.RequireLevel {
		return nil, errors.ErrShopLevelLow
	}

	// 根据货币类型校验金币/荣誉是否充足，并计算总价
	totalPrice := item.Price * int64(count)

	player.Mu().Lock()
	switch currencyType {
	case model.CurrencyGold: // 金币购买
		if player.Gold < totalPrice {
			player.Mu().Unlock()
			return nil, errors.ErrGoldNotEnough
		}
		player.Gold -= totalPrice
	case model.CurrencyHonor: // 荣誉购买
		if int64(player.Honor) < totalPrice {
			player.Mu().Unlock()
			return nil, errors.ErrHonorNotEnough
		}
		player.Honor -= int32(totalPrice)
	default:
		player.Mu().Unlock()
		return nil, errors.ErrParamInvalid
	}
	playerID := player.ID
	player.Mu().Unlock()

	// 扣减库存（非无限库存时）
	if item.Stock != -1 {
		success, stockErr := s.shopRepo.DecrementStock(ctx, itemID, count)
		if stockErr != nil || !success {
			logger.TError(ctx, "扣减商品库存失败", "item_id", itemID, "err", stockErr)
			// 库存扣减失败时退还货币
			player.Mu().Lock()
			switch currencyType {
			case model.CurrencyGold:
				player.Gold += totalPrice
			case model.CurrencyHonor:
				player.Honor += int32(totalPrice)
			}
			player.Mu().Unlock()
			return nil, errors.ErrShopStockOut
		}
	}

	// 通过 InventoryRepo 发放物品到玩家背包
	deliverCount := item.ItemCount * count
	if deliverCount > 0 {
		if addErr := s.inventoryRepo.AddItem(ctx, playerID, item.ItemID, deliverCount); addErr != nil {
			logger.TError(ctx, "发放商店物品到背包失败", "player_id", playerID, "item_id", item.ItemID, "err", addErr)
			// 物品发放失败不影响交易完成，只记录日志
		}
	}

	result := &ShopBuyResult{
		ItemID: itemID,
		Count:  count,
	}

	logger.TInfo(ctx, "购买商店商品", "player_id", playerID, "item_id", itemID,
		"count", count, "cost", totalPrice, "currency_type", currencyType,
		"deliver_item_id", item.ItemID, "deliver_count", deliverCount)
	return result, nil
}
