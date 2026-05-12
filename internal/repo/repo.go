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

// PlayerRepo 玩家数据访问接口，提供玩家基础数据的CRUD操作
type PlayerRepo interface {
	// GetPlayerByID 根据玩家ID查询玩家基础属性
	GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error)
	// GetMaxLayer 查询玩家地下城通关最高层数
	GetMaxLayer(ctx context.Context, playerID uint64) (int32, error)
	// SavePlayer 保存玩家基础属性到数据库
	SavePlayer(ctx context.Context, p *model.Player) error
	// SaveMaxLayer 保存玩家地下城通关进度
	SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error
	// CreatePlayer 创建新玩家记录，返回新玩家的ID
	CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error)
}

// ==================== 装备数据访问接口 ====================

// EquipRepo 装备数据访问接口，提供装备穿戴、强化、附魔等数据操作
type EquipRepo interface {
	// GetEquipBySlot 查询玩家指定槽位的装备
	GetEquipBySlot(ctx context.Context, playerID uint64, slot int32) (*model.Equipment, error)
	// SaveEquip 保存装备数据（新增或更新）
	SaveEquip(ctx context.Context, equip *model.Equipment) error
	// DeleteEquip 删除指定槽位的装备记录
	DeleteEquip(ctx context.Context, playerID uint64, slot int32) error
	// GetEquipTemplates 批量查询装备模板
	GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error)
	// GetForgeRecipe 查询锻造配方
	GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error)
}

// ==================== 技能数据访问接口 ====================

// SkillRepo 技能数据访问接口，提供技能等级的查询和更新
type SkillRepo interface {
	// GetSkillLevel 查询玩家指定技能的当前等级，0表示未学习
	GetSkillLevel(ctx context.Context, playerID uint64, skillID int32) (int32, error)
	// SetSkillLevel 设置玩家指定技能的等级
	SetSkillLevel(ctx context.Context, playerID uint64, skillID int32, level int32) error
	// GetAllSkills 查询玩家所有已学技能及等级，返回 map[skillID]level
	GetAllSkills(ctx context.Context, playerID uint64) (map[int32]int32, error)
	// DeleteAllSkills 删除玩家所有技能记录（重置时使用）
	DeleteAllSkills(ctx context.Context, playerID uint64) error
}

// ==================== 宠物数据访问接口 ====================

// PetRepo 宠物数据访问接口，提供宠物的CRUD操作
type PetRepo interface {
	// GetPetByUID 根据宠物实例ID查询宠物
	GetPetByUID(ctx context.Context, uid uint64) (*model.Pet, error)
	// GetPetsByOwner 查询玩家拥有的所有宠物
	GetPetsByOwner(ctx context.Context, ownerID uint64) ([]*model.Pet, error)
	// SavePet 保存宠物数据
	SavePet(ctx context.Context, pet *model.Pet) error
	// DeletePet 删除宠物实例
	DeletePet(ctx context.Context, uid uint64) error
}

// ==================== 商店数据访问接口 ====================

// ShopRepo 商店数据访问接口，提供商品查询和库存扣减
type ShopRepo interface {
	// ListShopItems 根据商店类型查询商品列表
	ListShopItems(ctx context.Context, shopType int32) ([]*model.ShopItem, error)
	// GetShopItem 根据商品ID查询单个商品
	GetShopItem(ctx context.Context, itemID uint64) (*model.ShopItem, error)
	// DecrementStock 扣减商品库存，返回是否成功
	DecrementStock(ctx context.Context, itemID uint64, count int32) (bool, error)
}

// ==================== 交易行数据访问接口 ====================

// TradeRepo 交易行数据访问接口，提供订单的查询、创建、购买、取消等操作
type TradeRepo interface {
	// ListOrders 按分类分页查询在售订单
	ListOrders(ctx context.Context, category int32, page, pageSize int32) ([]*model.TradeOrder, error)
	// CountOrders 统计指定分类的在售订单总数
	CountOrders(ctx context.Context, category int32) (int32, error)
	// GetOrder 根据订单ID查询订单
	GetOrder(ctx context.Context, orderID uint64) (*model.TradeOrder, error)
	// CreateOrder 创建交易订单
	CreateOrder(ctx context.Context, order *model.TradeOrder) (uint64, error)
	// BuyOrder 购买订单（将状态改为已售出），返回是否成功
	BuyOrder(ctx context.Context, orderID uint64, buyerID uint64) (bool, error)
	// CancelOrder 取消订单（将状态改为已下架），返回是否成功
	CancelOrder(ctx context.Context, orderID uint64, sellerID uint64) (bool, error)
}

// ==================== 排行榜数据访问接口 ====================

// RankRepo 排行榜数据访问接口，使用 Redis ZSet 实现排行榜功能。
type RankRepo interface {
	// Update 更新排行榜中指定成员的分数，若成员不存在则新增
	Update(ctx context.Context, key string, member string, score float64) error
	// GetTopN 获取排行榜前 N 名，按分数降序排列
	GetTopN(ctx context.Context, key string, n int64) ([]RankItem, error)
}

// ==================== 缓存访问接口 ====================

// CacheRepo 缓存访问接口，提供玩家缓存、Boss冷却、排行榜等缓存操作
type CacheRepo interface {
	// SetPlayerCache 将玩家数据序列化后缓存到Redis
	SetPlayerCache(ctx context.Context, playerID uint64, data []byte) error
	// GetPlayerCache 从Redis读取玩家缓存数据
	GetPlayerCache(ctx context.Context, playerID uint64) ([]byte, error)
	// DelPlayerCache 删除指定玩家的缓存数据
	DelPlayerCache(ctx context.Context, playerID uint64) error
	// SetBossCooldown 设置指定层数Boss的冷却标记
	SetBossCooldown(ctx context.Context, layer int32, cooldown time.Duration) error
	// IsBossCooldown 检查指定层数的Boss是否处于冷却中
	IsBossCooldown(ctx context.Context, layer int32) (bool, error)
	// UpdateRanking 向排行榜中添加或更新成员分数
	UpdateRanking(ctx context.Context, key string, member string, score float64) error
	// GetRanking 获取排行榜中分数从高到低的一段排名
	GetRanking(ctx context.Context, key string, offset, count int64) ([]RankItem, error)
}

// RankItem 排行榜条目，用于缓存层返回排行数据
type RankItem struct {
	Member string  // 成员标识（玩家ID字符串）
	Score  float64 // 分数
}
