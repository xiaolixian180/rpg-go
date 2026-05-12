// Package service - 商店服务
// 提供商品列表查询、购买等商店相关业务逻辑
package service

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
	// Buy 购买商店商品，校验金币/荣誉、库存、等级等条件
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
	shopRepo repo.ShopRepo // 商店数据访问接口
}

// NewShopService 创建商店服务实例
func NewShopService(shopRepo repo.ShopRepo) ShopService {
	return &shopService{
		shopRepo: shopRepo,
	}
}

// List 查询商店商品列表：
//  根据商店类型（普通/荣誉/公会）查询对应的商品列表
func (s *shopService) List(ctx context.Context, shopType int32) ([]*model.ShopItem, *errors.GameError) {
	items, err := s.shopRepo.ListShopItems(ctx, shopType)
	if err != nil {
		logger.Error("查询商店商品列表失败", "shop_type", shopType, "err", err)
		return nil, errors.ErrInternal
	}
	return items, nil
}

// Buy 贜买商店商品业务逻辑：
//  1. 查询商品信息
//  2. 校验商品是否存在
//  3. 校验库存是否充足（-1表示无限库存）
//  4. 校验玩家等级是否满足购买条件
//  5. 根据货币类型校验金币/荣誉是否充足
//  6. 扣减货币和库存
func (s *shopService) Buy(ctx context.Context, player *model.Player, itemID uint64, count int32, currencyType int32) (*ShopBuyResult, *errors.GameError) {
	// 查询商品信息
	item, err := s.shopRepo.GetShopItem(ctx, itemID)
	if err != nil || item == nil {
		return nil, errors.ErrShopNotFound
	}

	// 校验库存是否充足（-1表示无限库存）
	if item.Stock != -1 && item.Stock < count {
		return nil, errors.ErrShopStockOut
	}

	// 校验玩家等级是否满足购买条件
	if player.Level < item.RequireLevel {
		return nil, errors.ErrShopLevelLow
	}

	// 根据货币类型校验金币/荣誉是否充足，并计算总价
	totalPrice := item.Price * int64(count)

	switch currencyType {
	case 0: // 金币购买
		if player.Gold < totalPrice {
			return nil, errors.ErrGoldNotEnough
		}
		player.Gold -= totalPrice
	case 1: // 荣誉购买
		if int64(player.Honor) < totalPrice {
			return nil, errors.ErrHonorNotEnough
		}
		player.Honor -= int32(totalPrice)
	default:
		return nil, errors.ErrParamInvalid
	}

	// 扣减库存（非无限库存时）
	if item.Stock != -1 {
		success, stockErr := s.shopRepo.DecrementStock(ctx, itemID, count)
		if stockErr != nil || !success {
			logger.Error("扣减商品库存失败", "item_id", itemID, "err", stockErr)
			return nil, errors.ErrShopStockOut
		}
	}

	result := &ShopBuyResult{
		ItemID: itemID,
		Count:  count,
	}

	logger.Info("购买商店商品", "player_id", player.ID, "item_id", itemID, "count", count, "cost", totalPrice)
	return result, nil
}