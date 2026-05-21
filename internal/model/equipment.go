package model

import "sync"

// Equipment 玩家已装备的装备实例
type Equipment struct {
	ID              uint64 // 装备实例唯一标识
	PlayerID        uint64 // 所属玩家ID
	Slot            int32  // 装备槽位（对应SlotXxx常量）
	EquipID         int32  // 装备模板ID（对应EquipTemplate.ID）
	StrengthenLevel int32  // 强化等级
	EnchantAttr     string // 附魔属性描述
	Quality         int32  // 品质
}

// 装备槽位常量
const (
	SlotWeapon   = 0 // 武器槽位
	SlotHelmet   = 1 // 头盔槽位
	SlotArmor    = 2 // 铠甲槽位
	SlotGloves   = 3 // 手套槽位
	SlotBoots    = 4 // 靴子槽位
	SlotNecklace = 5 // 项链槽位
	SlotRing1    = 6 // 戒指1槽位
	SlotRing2    = 7 // 戒指2槽位
	SlotMax      = 8 // 槽位总数（用于数组边界）
)

// 装备品质常量，品质越高属性越好
const (
	QualityWhite  = 0 // 白色（普通）
	QualityGreen  = 1 // 绿色（优秀）
	QualityBlue   = 2 // 蓝色（精良）
	QualityPurple = 3 // 紫色（史诗）
	QualityOrange = 4 // 橙色（传说）
	QualityRed    = 5 // 红色（神话）
)

// QualityName 品质名称映射
var QualityName = map[int32]string{
	QualityWhite:  "普通",
	QualityGreen:  "优秀",
	QualityBlue:   "精良",
	QualityPurple: "史诗",
	QualityOrange: "传说",
	QualityRed:    "神话",
}

// EquipTemplate 装备模板，定义装备的基础属性
type EquipTemplate struct {
	ID           int32  // 模板ID
	Name         string // 装备名称
	Slot         int32  // 适配槽位
	Quality      int32  // 品质等级（对应QualityXxx常量）
	BaseAtk      int64  // 基础攻击力
	BaseDef      int64  // 基础防御力
	BaseHp       int64  // 基础生命值加成
	SetID        int32  // 所属套装ID（0表示不属于任何套装）
	RequireLevel int32  // 装备需求等级
}

// EquipTemplates 装备模板静态数据表
var EquipTemplates = map[int32]*EquipTemplate{
	// 武器 (Slot 0)
	1: {ID: 1, Name: "新手木剑", Slot: SlotWeapon, Quality: QualityWhite, BaseAtk: 5, BaseDef: 0, BaseHp: 0, RequireLevel: 1},
	2: {ID: 2, Name: "铁剑", Slot: SlotWeapon, Quality: QualityWhite, BaseAtk: 12, BaseDef: 0, BaseHp: 0, RequireLevel: 5},
	3: {ID: 3, Name: "精钢长剑", Slot: SlotWeapon, Quality: QualityGreen, BaseAtk: 25, BaseDef: 2, BaseHp: 0, RequireLevel: 10},
	4: {ID: 4, Name: "暗影之刃", Slot: SlotWeapon, Quality: QualityBlue, BaseAtk: 45, BaseDef: 5, BaseHp: 0, RequireLevel: 20},
	5: {ID: 5, Name: "龙牙巨剑", Slot: SlotWeapon, Quality: QualityPurple, BaseAtk: 80, BaseDef: 10, BaseHp: 50, RequireLevel: 30},
	6: {ID: 6, Name: "天罚圣剑", Slot: SlotWeapon, Quality: QualityOrange, BaseAtk: 130, BaseDef: 15, BaseHp: 100, RequireLevel: 45},
	// 头盔 (Slot 1)
	10: {ID: 10, Name: "布帽", Slot: SlotHelmet, Quality: QualityWhite, BaseAtk: 0, BaseDef: 3, BaseHp: 10, RequireLevel: 1},
	11: {ID: 11, Name: "铁盔", Slot: SlotHelmet, Quality: QualityWhite, BaseAtk: 0, BaseDef: 8, BaseHp: 30, RequireLevel: 5},
	12: {ID: 12, Name: "秘银头盔", Slot: SlotHelmet, Quality: QualityGreen, BaseAtk: 0, BaseDef: 18, BaseHp: 60, RequireLevel: 10},
	13: {ID: 13, Name: "暗夜兜帽", Slot: SlotHelmet, Quality: QualityBlue, BaseAtk: 5, BaseDef: 30, BaseHp: 100, RequireLevel: 20},
	14: {ID: 14, Name: "战神之冠", Slot: SlotHelmet, Quality: QualityPurple, BaseAtk: 10, BaseDef: 50, BaseHp: 180, RequireLevel: 30},
	// 铠甲 (Slot 2)
	20: {ID: 20, Name: "布衣", Slot: SlotArmor, Quality: QualityWhite, BaseAtk: 0, BaseDef: 5, BaseHp: 20, RequireLevel: 1},
	21: {ID: 21, Name: "铁甲", Slot: SlotArmor, Quality: QualityWhite, BaseAtk: 0, BaseDef: 15, BaseHp: 50, RequireLevel: 5},
	22: {ID: 22, Name: "精钢战甲", Slot: SlotArmor, Quality: QualityGreen, BaseAtk: 0, BaseDef: 30, BaseHp: 100, RequireLevel: 10},
	23: {ID: 23, Name: "暗影战甲", Slot: SlotArmor, Quality: QualityBlue, BaseAtk: 5, BaseDef: 55, BaseHp: 180, RequireLevel: 20},
	24: {ID: 24, Name: "龙鳞铠甲", Slot: SlotArmor, Quality: QualityPurple, BaseAtk: 10, BaseDef: 90, BaseHp: 300, RequireLevel: 30},
	// 手套 (Slot 3)
	30: {ID: 30, Name: "布手套", Slot: SlotGloves, Quality: QualityWhite, BaseAtk: 2, BaseDef: 2, BaseHp: 0, RequireLevel: 1},
	31: {ID: 31, Name: "铁手套", Slot: SlotGloves, Quality: QualityWhite, BaseAtk: 5, BaseDef: 5, BaseHp: 0, RequireLevel: 5},
	32: {ID: 32, Name: "精钢护手", Slot: SlotGloves, Quality: QualityGreen, BaseAtk: 12, BaseDef: 12, BaseHp: 20, RequireLevel: 10},
	// 靴子 (Slot 4)
	40: {ID: 40, Name: "草鞋", Slot: SlotBoots, Quality: QualityWhite, BaseAtk: 0, BaseDef: 3, BaseHp: 5, RequireLevel: 1},
	41: {ID: 41, Name: "铁靴", Slot: SlotBoots, Quality: QualityWhite, BaseAtk: 0, BaseDef: 8, BaseHp: 15, RequireLevel: 5},
	42: {ID: 42, Name: "疾风之靴", Slot: SlotBoots, Quality: QualityGreen, BaseAtk: 0, BaseDef: 18, BaseHp: 30, RequireLevel: 10},
	// 项链 (Slot 5)
	50: {ID: 50, Name: "铜项链", Slot: SlotNecklace, Quality: QualityWhite, BaseAtk: 3, BaseDef: 0, BaseHp: 10, RequireLevel: 1},
	51: {ID: 51, Name: "银项链", Slot: SlotNecklace, Quality: QualityGreen, BaseAtk: 8, BaseDef: 0, BaseHp: 30, RequireLevel: 10},
	// 戒指 (Slot 6/7)
	60: {ID: 60, Name: "铜戒指", Slot: SlotRing1, Quality: QualityWhite, BaseAtk: 2, BaseDef: 0, BaseHp: 5, RequireLevel: 1},
	61: {ID: 61, Name: "银戒指", Slot: SlotRing1, Quality: QualityGreen, BaseAtk: 6, BaseDef: 0, BaseHp: 20, RequireLevel: 10},
}

// EquipSet 套装定义，集齐一定件数可激活套装效果
type EquipSet struct {
	ID    int32            // 套装ID
	Name  string           // 套装名称
	Bonus map[int32]string // 套装件数→效果描述，如2件套、4件套效果
}

// PlayerEquipORM 玩家装备表持久化模型，对应 player_equip 表。
type PlayerEquipORM struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID        uint64 `gorm:"not null;uniqueIndex:uk_player_slot;index:idx_player_id" json:"player_id"`
	Slot            int32  `gorm:"not null;uniqueIndex:uk_player_slot" json:"slot"`
	EquipID         int32  `gorm:"not null" json:"equip_id"`
	StrengthenLevel int32  `gorm:"not null;default:0" json:"strengthen_level"`
	EnchantAttr     string `gorm:"type:varchar(64);not null;default:''" json:"enchant_attr"`
	Quality         int32  `gorm:"not null;default:0" json:"quality"`
}

// TableName 指定 PlayerEquipORM 对应的数据库表名
func (PlayerEquipORM) TableName() string { return "player_equip" }

// InventoryItem 背包物品
type InventoryItem struct {
	mu     sync.RWMutex
	ID     uint64 // 物品实例ID
	Owner  uint64 // 所属玩家ID
	ItemID int32  // 物品模板ID
	Count  int32  // 数量
}

// Mu 返回 InventoryItem 的读写锁
func (i *InventoryItem) Mu() *sync.RWMutex { return &i.mu }

// PlayerInventoryORM 背包表持久化模型
type PlayerInventoryORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID uint64 `gorm:"not null;index:idx_player_item;index:idx_player_id" json:"player_id"`
	ItemID   int32  `gorm:"not null;index:idx_player_item" json:"item_id"`
	Count    int32  `gorm:"not null;default:1" json:"count"`
}

// TableName 指定 PlayerInventoryORM 对应的数据库表名
func (PlayerInventoryORM) TableName() string { return "player_inventory" }
