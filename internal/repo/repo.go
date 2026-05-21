// Package repo 定义数据访问层接口。
// Service 层通过这些接口访问数据库和缓存，不直接依赖 database/cache 包，
// 实现业务逻辑与数据存储的解耦，便于单元测试时 mock 替换。
package repo

import (
	"context"
	"time"

	"hero-quest/internal/model"
)

// ==================== 玩家数据访问接口 ====================

type PlayerRepo interface {
	GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error)
	GetMaxLayer(ctx context.Context, playerID uint64) (int32, error)
	SavePlayer(ctx context.Context, p *model.Player) error
	SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error
	CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error)
}

// ==================== 装备数据访问接口 ====================

type EquipRepo interface {
	GetEquipBySlot(ctx context.Context, playerID uint64, slot int32) (*model.Equipment, error)
	GetAllEquips(ctx context.Context, playerID uint64) ([]*model.Equipment, error)
	SaveEquip(ctx context.Context, equip *model.Equipment) error
	DeleteEquip(ctx context.Context, playerID uint64, slot int32) error
	GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error)
	GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error)
}

// ==================== 技能数据访问接口 ====================

type SkillRepo interface {
	GetSkillLevel(ctx context.Context, playerID uint64, skillID int32) (int32, error)
	SetSkillLevel(ctx context.Context, playerID uint64, skillID int32, level int32) error
	GetAllSkills(ctx context.Context, playerID uint64) (map[int32]int32, error)
	DeleteAllSkills(ctx context.Context, playerID uint64) error
}

// ==================== 宠物数据访问接口 ====================

type PetRepo interface {
	GetPetByUID(ctx context.Context, uid uint64) (*model.Pet, error)
	GetPetsByOwner(ctx context.Context, ownerID uint64) ([]*model.Pet, error)
	SavePet(ctx context.Context, pet *model.Pet) error
	DeletePet(ctx context.Context, uid uint64) error
}

// ==================== 商店数据访问接口 ====================

type ShopRepo interface {
	ListShopItems(ctx context.Context, shopType int32) ([]*model.ShopItem, error)
	GetShopItem(ctx context.Context, itemID uint64) (*model.ShopItem, error)
	DecrementStock(ctx context.Context, itemID uint64, count int32) (bool, error)
}

// ==================== 交易行数据访问接口 ====================

type TradeRepo interface {
	ListOrders(ctx context.Context, category int32, page, pageSize int32) ([]*model.TradeOrder, error)
	CountOrders(ctx context.Context, category int32) (int32, error)
	GetOrder(ctx context.Context, orderID uint64) (*model.TradeOrder, error)
	CreateOrder(ctx context.Context, order *model.TradeOrder) (uint64, error)
	BuyOrder(ctx context.Context, orderID uint64, buyerID uint64) (bool, error)
	CancelOrder(ctx context.Context, orderID uint64, sellerID uint64) (bool, error)
}

// ==================== 排行榜数据访问接口 ====================

type RankRepo interface {
	Update(ctx context.Context, key string, member string, score float64) error
	GetTopN(ctx context.Context, key string, n int64) ([]RankItem, error)
}

// ==================== 背包数据访问接口 ====================

type InventoryRepo interface {
	GetItemCount(ctx context.Context, playerID uint64, itemID int32) (int32, error)
	AddItem(ctx context.Context, playerID uint64, itemID int32, count int32) error
	RemoveItem(ctx context.Context, playerID uint64, itemID int32, count int32) (bool, error)
	ListItems(ctx context.Context, playerID uint64) ([]*model.PlayerInventoryORM, error)
}

// ==================== 缓存访问接口 ====================

type CacheRepo interface {
	SetPlayerCache(ctx context.Context, playerID uint64, data []byte) error
	GetPlayerCache(ctx context.Context, playerID uint64) ([]byte, error)
	DelPlayerCache(ctx context.Context, playerID uint64) error
	SetBossCooldown(ctx context.Context, layer int32, cooldown time.Duration) error
	IsBossCooldown(ctx context.Context, layer int32) (bool, error)
	UpdateRanking(ctx context.Context, key string, member string, score float64) error
	GetRanking(ctx context.Context, key string, offset, count int64) ([]RankItem, error)
}

// RankItem 排行榜条目
type RankItem struct {
	Member string
	Score  float64
}
