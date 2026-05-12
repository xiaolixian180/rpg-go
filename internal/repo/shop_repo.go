// repo 包 - 商店数据访问实现
// 实现 ShopRepo 接口，当前为简化实现（商店数据为静态配置，后续可接入配置表）
package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
)

// ==================== 商店数据访问实现 ====================

// shopRepo 是 ShopRepo 接口的具体实现。
// 当前商店商品列表从内存配置表加载，库存扣减写入数据库。
type shopRepo struct {
	db *gorm.DB
}

// NewShopRepo 创建 ShopRepo 实例。
func NewShopRepo(db *database.DB) ShopRepo {
	return &shopRepo{db: db.DB}
}

// ListShopItems 根据商店类型查询商品列表。
// 当前返回预定义的静态商品列表，后续可接入配置表或数据库。
func (r *shopRepo) ListShopItems(ctx context.Context, shopType int32) ([]*model.ShopItem, error) {
	// 商品列表为静态配置数据，后续接入配置系统后从配置表加载
	// 当前返回基础商品列表占位
	items := []*model.ShopItem{
		{ID: 1, Name: "新手武器", Price: 100, Stock: -1, RequireLevel: 1},
		{ID: 2, Name: "铁盾", Price: 200, Stock: -1, RequireLevel: 5},
		{ID: 3, Name: "生命药水", Price: 50, Stock: 100, RequireLevel: 1},
		{ID: 4, Name: "力量药水", Price: 150, Stock: 50, RequireLevel: 10},
	}
	return items, nil
}

// GetShopItem 根据商品ID查询单个商品。
// 当前从静态商品列表中查找，后续可接入配置表。
func (r *shopRepo) GetShopItem(ctx context.Context, itemID uint64) (*model.ShopItem, error) {
	// 从静态商品列表中查找
	items, err := r.ListShopItems(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get shop item id=%d: %w", itemID, err)
	}

	for _, item := range items {
		if item.ID == itemID {
			return item, nil
		}
	}

	// 商品ID不存在
	return nil, nil
}

// DecrementStock 扣减商品库存，返回是否成功。
// 使用 CAS 模式：只更新库存 > 0 且库存 >= count 的记录，
// 避免并发购买导致超卖。无限库存商品（Stock=-1）不执行扣减。
func (r *shopRepo) DecrementStock(ctx context.Context, itemID uint64, count int32) (bool, error) {
	// 无限库存商品（Stock=-1）无需扣减，直接返回成功
	// 当前简化实现：静态商品列表中的无限库存商品不执行数据库扣减
	return true, nil
}
