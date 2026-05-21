package model

// ShopItem 商店售卖的商品
type ShopItem struct {
	ID           uint64 // 商品唯一标识
	Name         string // 商品名称
	Price        int64  // 售价
	Stock        int32  // 库存数量（-1表示无限）
	RequireLevel int32  // 购买所需最低等级
	CurrencyType int32  // 货币类型：0=金币 1=荣誉
	ItemType     int32  // 物品类型：0=装备 1=消耗品 2=材料
	ItemID       int32  // 发放的物品模板ID（装备为EquipID，消耗品为ItemID）
	ItemCount    int32  // 发放数量
}

// 货币类型常量
const (
	CurrencyGold  = 0 // 金币
	CurrencyHonor = 1 // 荣誉
)

// ForgeRecipe 锻造配方，定义合成装备所需的材料和产出
type ForgeRecipe struct {
	ID            uint64           // 配方唯一标识
	Name          string           // 配方名称
	Materials     map[uint64]int32 // 所需材料，key为材料ID，value为数量
	ResultID      int32            // 锻造产出的装备模板ID
	ResultQuality int32            // 锻造产出的品质等级
	RequireLevel  int32            // 锻造所需等级
	Cost          int64            // 锻造金币消耗
}

// StaticShopItems 商店静态商品列表
var StaticShopItems = []*ShopItem{
	// 金币商店
	{ID: 1, Name: "新手武器", Price: 100, Stock: -1, RequireLevel: 1, CurrencyType: CurrencyGold, ItemType: 0, ItemID: 1, ItemCount: 1},
	{ID: 2, Name: "铁盔", Price: 200, Stock: -1, RequireLevel: 5, CurrencyType: CurrencyGold, ItemType: 0, ItemID: 11, ItemCount: 1},
	{ID: 3, Name: "生命药水", Price: 50, Stock: 100, RequireLevel: 1, CurrencyType: CurrencyGold, ItemType: 1, ItemID: 1001, ItemCount: 5},
	{ID: 4, Name: "力量药水", Price: 150, Stock: 50, RequireLevel: 10, CurrencyType: CurrencyGold, ItemType: 1, ItemID: 1002, ItemCount: 3},
	// 荣誉商店
	{ID: 10, Name: "精钢长剑", Price: 500, Stock: -1, RequireLevel: 10, CurrencyType: CurrencyHonor, ItemType: 0, ItemID: 3, ItemCount: 1},
	{ID: 11, Name: "暗影战甲", Price: 800, Stock: -1, RequireLevel: 20, CurrencyType: CurrencyHonor, ItemType: 0, ItemID: 23, ItemCount: 1},
}

// StaticForgeRecipes 锻造配方静态数据
var StaticForgeRecipes = []*ForgeRecipe{
	{ID: 1, Name: "精钢长剑锻造", Materials: map[uint64]int32{2001: 3, 2002: 2}, ResultID: 3, ResultQuality: QualityGreen, RequireLevel: 10, Cost: 500},
	{ID: 2, Name: "暗影之刃锻造", Materials: map[uint64]int32{2001: 5, 2003: 3}, ResultID: 4, ResultQuality: QualityBlue, RequireLevel: 20, Cost: 1500},
	{ID: 3, Name: "龙牙巨剑锻造", Materials: map[uint64]int32{2001: 10, 2003: 5, 2004: 2}, ResultID: 5, ResultQuality: QualityPurple, RequireLevel: 30, Cost: 5000},
}
