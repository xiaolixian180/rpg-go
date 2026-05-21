// repo 包 - 商店数据访问实现
package repo

import (
	"context"
	"fmt"
	"sync"

	"hero-quest/internal/database"
	"hero-quest/internal/model"
)

// ==================== 商店数据访问实现 ====================

// shopRepo 是 ShopRepo 接口的具体实现。
type shopRepo struct {
	stock map[uint64]int32 // 有限库存商品的当前库存
	mu    sync.Mutex       // 保护 stock 的并发访问
}

// NewShopRepo 创建 ShopRepo 实例。
func NewShopRepo(_ *database.DB) ShopRepo {
	stock := make(map[uint64]int32)
	for _, item := range model.StaticShopItems {
		if item.Stock > 0 {
			stock[item.ID] = item.Stock
		}
	}
	return &shopRepo{stock: stock}
}

// ListShopItems 根据商店类型查询商品列表。
func (r *shopRepo) ListShopItems(ctx context.Context, shopType int32) ([]*model.ShopItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*model.ShopItem, 0)
	for _, item := range model.StaticShopItems {
		if shopType == 0 || item.CurrencyType == shopType {
			// 返回带有当前库存的商品
			copy := *item
			if curStock, ok := r.stock[item.ID]; ok {
				copy.Stock = curStock
			}
			items = append(items, &copy)
		}
	}
	return items, nil
}

// GetShopItem 根据商品ID查询单个商品。
func (r *shopRepo) GetShopItem(ctx context.Context, itemID uint64) (*model.ShopItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range model.StaticShopItems {
		if item.ID == itemID {
			copy := *item
			if curStock, ok := r.stock[item.ID]; ok {
				copy.Stock = curStock
			}
			return &copy, nil
		}
	}
	return nil, nil
}

// DecrementStock 扣减商品库存，返回是否成功。
func (r *shopRepo) DecrementStock(ctx context.Context, itemID uint64, count int32) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	curStock, hasStock := r.stock[itemID]
	if !hasStock {
		// 无限库存商品（Stock=-1）
		for _, item := range model.StaticShopItems {
			if item.ID == itemID && item.Stock < 0 {
				return true, nil
			}
		}
		return false, fmt.Errorf("item %d not found", itemID)
	}
	if curStock < count {
		return false, nil
	}
	r.stock[itemID] = curStock - count
	return true, nil
}
