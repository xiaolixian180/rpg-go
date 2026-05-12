// Package model 定义游戏核心数据模型。
// 所有模型为纯结构体，不含业务逻辑，供 service/repo/handler 共同引用。
package model

import (
	"sync"
	"time"
)

// ==================== 职业 ====================

// 职业常量
const (
	ClassWarrior  = 0 // 战士
	ClassMage     = 1 // 法师
	ClassArcher   = 2 // 射手
	ClassPriest   = 3 // 牧师
	ClassAssassin = 4 // 刺客
)

// ClassName 职业ID到中文名称的映射表
var ClassName = map[int32]string{
	ClassWarrior:  "战士",
	ClassMage:     "法师",
	ClassArcher:   "射手",
	ClassPriest:   "牧师",
	ClassAssassin: "刺客",
}

// ==================== 区域 ====================

// 区域名称常量，按副本层数划分不同区域
const (
	ZoneForest  = "翠绿森林"  // 1~10层区域
	ZoneSwamp   = "腐蚀沼泽"  // 11~20层区域
	ZoneVolcano = "烈焰火山"  // 21~30层区域
)

// GetZoneByLayer 根据副本层数返回对应的区域名称
func GetZoneByLayer(layer int32) string {
	switch {
	case layer >= 1 && layer <= 10:
		return ZoneForest
	case layer >= 11 && layer <= 20:
		return ZoneSwamp
	case layer >= 21 && layer <= 30:
		return ZoneVolcano
	default:
		return "未知区域"
	}
}

// IsBossLayer 判断指定层数是否为Boss层（每10层出现Boss）
func IsBossLayer(layer int32) bool {
	return layer%10 == 0
}

// ==================== 玩家 ====================

// Player 玩家核心数据结构，保存玩家在游戏中的所有持久化属性和运行时状态。
// 使用读写锁(mu)保证并发安全，所有字段读写前必须加锁。
type Player struct {
	mu             sync.RWMutex // 读写锁，保护Player所有字段的并发访问安全
	ID             uint64       // 玩家唯一ID，对应数据库主键
	Name           string       // 玩家角色名
	Class          int32        // 职业类型（对应ClassXxx常量）
	Level          int32        // 当前等级，影响生命值上限和基础伤害
	Exp            int64        // 当前经验值，达到升级阈值后等级+1
	Gold           int64        // 金币余额，用于购买商品、强化装备等
	Honor          int32        // 荣誉值，通过PvP击杀获得，可用于荣誉商店兑换
	KillValue      int32        // 杀戮值（PK值），每PvP击杀一名玩家+1，达到红名阈值后成为红名
	Str            int32        // 力量属性，影响物理伤害和生命值上限
	Agi            int32        // 敏捷属性，影响闪避率和攻击速度
	Int            int32        // 智力属性，影响魔法伤害和魔法值上限
	Con            int32        // 体质属性，影响生命值上限
	AttrPoints     int32        // 可分配属性点，升级时获得，玩家可自由分配到力量/敏捷/智力/体质
	MaxLayer       int32        // 地下城已通关最高层数，决定玩家可进入的楼层范围
	Hp             int64        // 当前生命值，降为0时角色死亡
	MaxHp          int64        // 生命值上限，由等级、体质、力量共同计算得出
	X              float64      // 角色在当前地图中的X坐标
	Y              float64      // 角色在当前地图中的Y坐标
	Layer          int32        // 当前所在的地下城层数，0表示不在地下城中
	Online         bool         // 是否在线，登录时设为true，登出时设为false
	InvincibleUntil time.Time   // 无敌状态截止时间，PvP被击杀后获得30秒无敌保护
}

// CalcMaxHp 计算玩家的生命值上限。
// 计算公式：基础值(100 + 等级*20) + 体质*10 + 力量*5
func (p *Player) CalcMaxHp() int64 {
	base := int64(100 + p.Level*20)
	return base + int64(p.Con)*10 + int64(p.Str)*5
}

// IsInvincible 判断玩家当前是否处于无敌状态。
// 无敌状态在PvP被击杀后持续30秒，期间无法被其他玩家攻击。
func (p *Player) IsInvincible() bool {
	return time.Now().Before(p.InvincibleUntil)
}

// Mu 返回 Player 的读写锁指针，供外部按需加锁保护并发操作
func (p *Player) Mu() *sync.RWMutex {
	return &p.mu
}

// ==================== 地下城 ====================

// DungeonLayer 副本单层结构，包含该层的怪物、玩家和资源
type DungeonLayer struct {
	Layer     int32               // 层数编号
	IsBoss    bool                // 是否为Boss层（每10层为Boss层）
	Monsters  map[uint64]*Monster // 该层所有怪物，key为怪物ID
	Players   map[uint64]*Player  // 该层所有玩家，key为玩家ID
	Resources map[uint64]*Resource // 该层所有可采集资源，key为资源ID
	mu        sync.RWMutex        // 读写锁，保护并发访问
}

// Mu 返回 DungeonLayer 的读写锁指针，供外部按需加锁保护并发操作
func (d *DungeonLayer) Mu() *sync.RWMutex {
	return &d.mu
}

// Monster 副本中的普通怪物
type Monster struct {
	ID         uint64  // 怪物唯一标识
	Name       string  // 怪物名称
	Hp         int64   // 当前生命值
	MaxHp      int64   // 最大生命值
	ExpReward  int64   // 击杀后奖励的经验值
	GoldReward int64   // 击杀后奖励的金币数
	X          float64 // 地图中的X坐标
	Y          float64 // 地图中的Y坐标
}

// Resource 副本中可采集的资源点
type Resource struct {
	ID        uint64  // 资源唯一标识
	Type      int32   // 资源类型（0=矿石 1=草药 2=木材）
	Name      string  // 资源名称
	X         float64 // 地图中的X坐标
	Y         float64 // 地图中的Y坐标
	Harvested bool    // 是否已被采集
}

// ==================== Boss ====================

// Boss 副本中的Boss怪物，每10层出现一次
type Boss struct {
	ID       uint64      // Boss唯一标识
	Name     string      // Boss名称
	Hp       int64       // 当前生命值
	MaxHp    int64       // 最大生命值
	Layer    int32       // 所在副本层数
	X        float64     // 地图中的X坐标
	Y        float64     // 地图中的Y坐标
	Cooldown time.Duration // 技能公共冷却时间
	Skills   []BossSkill // Boss拥有的技能列表
	mu       sync.RWMutex // 读写锁，保护并发访问
}

// Mu 返回 Boss 的读写锁指针，供外部按需加锁保护并发操作
func (b *Boss) Mu() *sync.RWMutex {
	return &b.mu
}

// BossSkill Boss技能定义
type BossSkill struct {
	SkillID int32   // 技能ID
	Name    string  // 技能名称
	CD      float64 // 技能冷却时间（秒）
	Range   float64 // 技能施放范围
	Damage  int64   // 技能造成的伤害值
}

// ==================== 装备 ====================

// Equipment 玩家已装备的装备实例
type Equipment struct {
	ID              uint64 // 装备实例唯一标识
	PlayerID        uint64 // 所属玩家ID
	Slot            int32  // 装备槽位（对应SlotXxx常量）
	EquipID         int32  // 装备模板ID（对应EquipTemplate.ID）
	StrengthenLevel int32  // 强化等级
	EnchantAttr     string // 附魔属性描述
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

// EquipTemplate 装备模板，定义装备的基础属性
type EquipTemplate struct {
	ID           int32  // 模板ID
	Name         string // 装备名称
	Quality      int32  // 品质等级（对应QualityXxx常量）
	BaseAtk      int64  // 基础攻击力
	BaseDef      int64  // 基础防御力
	BaseHp       int64  // 基础生命值加成
	SetID        int32  // 所属套装ID（0表示不属于任何套装）
	RequireLevel int32  // 装备需求等级
}

// EquipSet 套装定义，集齐一定件数可激活套装效果
type EquipSet struct {
	ID    int32            // 套装ID
	Name  string           // 套装名称
	Bonus map[int32]string // 套装件数→效果描述，如2件套、4件套效果
}

// ==================== 宠物 ====================

// Pet 宠物实例，玩家可携带宠物进行探索或战斗
type Pet struct {
	mu             sync.RWMutex  // 读写锁，保护并发访问
	UID            uint64        // 宠物实例唯一标识
	OwnerID        uint64        // 所属玩家ID
	PetID          int32         // 宠物模板ID
	Name           string        // 宠物名称
	Level          int32         // 宠物等级
	Quality        int32         // 宠物品质（对应品质常量）
	Type           int32         // 宠物类型（对应PetTypeXxx常量）
	Skills         string        // 宠物技能列表（序列化字符串）
	Exploring      bool          // 是否正在探索中
	ExploreEndTime time.Time     // 探索结束时间
}

// Mu 返回 Pet 的读写锁指针，供外部按需加锁保护并发操作
func (p *Pet) Mu() *sync.RWMutex {
	return &p.mu
}

// 宠物类型常量
const (
	PetTypeAttack  = 0 // 攻击型宠物
	PetTypeDefense = 1 // 防御型宠物
	PetTypeSupport = 2 // 辅助型宠物
	PetTypePlunder = 3 // 掠夺型宠物
)

// PetMaxExploreHours 宠物单次探索最大时长（小时）
const PetMaxExploreHours = 12

// ==================== 商店/锻造 ====================

// ShopItem 商店售卖的商品
type ShopItem struct {
	ID            uint64 // 商品唯一标识
	Name          string // 商品名称
	Price         int64  // 售价（金币）
	Stock         int32  // 库存数量（-1表示无限）
	RequireLevel int32  // 购买所需最低等级
}

// ForgeRecipe 锻造配方，定义合成装备所需的材料和产出
type ForgeRecipe struct {
	ID            uint64            // 配方唯一标识
	Name          string            // 配方名称
	Materials     map[uint64]int32  // 所需材料，key为材料ID，value为数量
	ResultID      int32             // 锻造产出的装备模板ID
	ResultQuality int32             // 锻造产出的品质等级
}

// ==================== 技能 ====================

// SkillDef 技能定义，描述一个技能的基本属性
type SkillDef struct {
	ID       int32  // 技能ID
	Name     string // 技能名称
	Class    int32  // 所属职业（对应ClassXxx常量）
	Type     int32  // 技能类型：0=主动 1=被动 2=终极
	MaxLevel int32  // 技能最大可升级等级
}

// SkillDefs 全局技能定义表，key为技能ID
// 初始化后只读，并发安全
var SkillDefs = map[int32]SkillDef{
	// 战士
	1:  {ID: 1, Name: "旋风斩", Class: ClassWarrior, Type: 0, MaxLevel: 10},
	2:  {ID: 2, Name: "嘲讽盾", Class: ClassWarrior, Type: 0, MaxLevel: 10},
	3:  {ID: 3, Name: "冲锋", Class: ClassWarrior, Type: 0, MaxLevel: 10},
	4:  {ID: 4, Name: "钢铁意志", Class: ClassWarrior, Type: 1, MaxLevel: 10},
	5:  {ID: 5, Name: "战神降临", Class: ClassWarrior, Type: 2, MaxLevel: 10},
	// 法师
	6:  {ID: 6, Name: "陨石术", Class: ClassMage, Type: 0, MaxLevel: 10},
	7:  {ID: 7, Name: "暴风雪", Class: ClassMage, Type: 0, MaxLevel: 10},
	8:  {ID: 8, Name: "闪现", Class: ClassMage, Type: 0, MaxLevel: 10},
	9:  {ID: 9, Name: "元素亲和", Class: ClassMage, Type: 1, MaxLevel: 10},
	10: {ID: 10, Name: "元素风暴", Class: ClassMage, Type: 2, MaxLevel: 10},
	// 射手
	11: {ID: 11, Name: "穿云箭", Class: ClassArcher, Type: 0, MaxLevel: 10},
	12: {ID: 12, Name: "冰冻陷阱", Class: ClassArcher, Type: 0, MaxLevel: 10},
	13: {ID: 13, Name: "翻滚", Class: ClassArcher, Type: 0, MaxLevel: 10},
	14: {ID: 14, Name: "鹰眼", Class: ClassArcher, Type: 1, MaxLevel: 10},
	15: {ID: 15, Name: "万箭齐发", Class: ClassArcher, Type: 2, MaxLevel: 10},
	// 牧师
	16: {ID: 16, Name: "圣光术", Class: ClassPriest, Type: 0, MaxLevel: 10},
	17: {ID: 17, Name: "暗影鞭笞", Class: ClassPriest, Type: 0, MaxLevel: 10},
	18: {ID: 18, Name: "治愈光环", Class: ClassPriest, Type: 0, MaxLevel: 10},
	19: {ID: 19, Name: "信仰之力", Class: ClassPriest, Type: 1, MaxLevel: 10},
	20: {ID: 20, Name: "天使降临", Class: ClassPriest, Type: 2, MaxLevel: 10},
	// 刺客
	21: {ID: 21, Name: "背刺", Class: ClassAssassin, Type: 0, MaxLevel: 10},
	22: {ID: 22, Name: "影分身", Class: ClassAssassin, Type: 0, MaxLevel: 10},
	23: {ID: 23, Name: "隐身", Class: ClassAssassin, Type: 0, MaxLevel: 10},
	24: {ID: 24, Name: "致命一击", Class: ClassAssassin, Type: 1, MaxLevel: 10},
	25: {ID: 25, Name: "暗影绝杀", Class: ClassAssassin, Type: 2, MaxLevel: 10},
}

// ==================== 交易行 ====================

// TradeOrder 交易行订单，玩家上架出售的装备
type TradeOrder struct {
	ID              uint64    // 订单唯一标识
	SellerID        uint64    // 卖家玩家ID
	EquipID         int32     // 装备模板ID
	Quality         int32     // 装备品质等级
	StrengthenLevel int32     // 装备强化等级
	Price           int64     // 挂单价格（金币）
	Status          int32     // 订单状态：0=在售 1=已售出 2=已下架
	CreatedAt       time.Time // 订单创建时间
}

// 交易订单状态常量
const (
	TradeOrderOnSale   = 0 // 在售
	TradeOrderSold     = 1 // 已售出
	TradeOrderCancelled = 2 // 已下架
)

// ==================== 掉落物品 ====================

// DropItem 掉落物品，怪物/Boss被击杀后掉落的物品
type DropItem struct {
	ItemID  uint64 // 物品模板ID
	Name    string // 物品名称
	Quality int32  // 品质等级：1=白 2=绿 3=蓝 4=紫 5=橙
	Count   int32  // 物品数量
}

// ==================== 排行榜 ====================

// RankingItem 排行榜条目
type RankingItem struct {
	Rank     int32  // 排名
	PlayerID uint64 // 玩家ID
	Name     string // 玩家名称
	Value    int64  // 分数值（等级/战力/荣誉）
}

// ==================== GORM 持久化模型 ====================
// 以下结构体用于 GORM AutoMigrate 和数据库读写，
// 与上面的运行时模型分离，避免运行时字段（如 sync.RWMutex）参与序列化。

// PlayerORM 玩家表持久化模型，对应 player 表。
// 注意：int 字段因是 MySQL 保留字，需用 gorm tag 指定列名为 `int`。
type PlayerORM struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`                              // 玩家唯一ID，自增主键
	Name       string `gorm:"type:varchar(32);uniqueIndex;not null" json:"name"`               // 玩家角色名，全局唯一
	Class      int32  `gorm:"not null;default:0" json:"class"`                                 // 职业类型，0 表示未选择
	Level      int32  `gorm:"not null;default:1" json:"level"`                                 // 玩家等级，初始为 1
	Exp        int64  `gorm:"not null;default:0" json:"exp"`                                   // 当前累计经验值
	Gold       int64  `gorm:"not null;default:0" json:"gold"`                                  // 金币数量
	Honor      int32  `gorm:"not null;default:0" json:"honor"`                                 // 荣誉值
	KillValue  int32  `gorm:"column:kill_value;not null;default:0" json:"kill_value"`          // 击杀值
	Str        int32  `gorm:"not null;default:0" json:"str"`                                   // 力量属性
	Agi        int32  `gorm:"not null;default:0" json:"agi"`                                   // 敏捷属性
	Int        int32  `gorm:"column:int;not null;default:0" json:"int"`                        // 智力属性（列名 int 为 MySQL 保留字）
	Con        int32  `gorm:"not null;default:0" json:"con"`                                   // 体质属性
	AttrPoints int32  `gorm:"not null;default:0" json:"attr_points"`                           // 可分配属性点
	CreatedAt  int64  `gorm:"autoCreateTime" json:"created_at"`                                // 创建时间
	UpdatedAt  int64  `gorm:"autoUpdateTime" json:"updated_at"`                                // 更新时间
}

// TableName 指定 PlayerORM 对应的数据库表名
func (PlayerORM) TableName() string { return "player" }

// PlayerSkillORM 玩家技能表持久化模型，对应 player_skill 表。
type PlayerSkillORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`                      // 技能记录ID
	PlayerID uint64 `gorm:"not null;index:idx_player_id" json:"player_id"`            // 所属玩家ID
	SkillID  int32  `gorm:"not null;uniqueIndex:uk_player_skill" json:"skill_id"`     // 技能配置ID
	Level    int32  `gorm:"not null;default:0" json:"level"`                          // 技能等级
}

// TableName 指定 PlayerSkillORM 对应的数据库表名
func (PlayerSkillORM) TableName() string { return "player_skill" }

// PlayerEquipORM 玩家装备表持久化模型，对应 player_equip 表。
type PlayerEquipORM struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement" json:"id"`                       // 装备记录ID
	PlayerID        uint64 `gorm:"not null;uniqueIndex:uk_player_slot;index:idx_player_id" json:"player_id"` // 所属玩家ID
	Slot            int32  `gorm:"not null;uniqueIndex:uk_player_slot" json:"slot"`          // 装备槽位编号
	EquipID         int32  `gorm:"not null" json:"equip_id"`                                 // 装备配置ID
	StrengthenLevel int32  `gorm:"not null;default:0" json:"strengthen_level"`               // 强化等级
	EnchantAttr     string `gorm:"type:varchar(64);not null;default:''" json:"enchant_attr"` // 附魔属性
}

// TableName 指定 PlayerEquipORM 对应的数据库表名
func (PlayerEquipORM) TableName() string { return "player_equip" }

// PlayerPetORM 玩家宠物表持久化模型，对应 player_pet 表。
type PlayerPetORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`              // 宠物记录ID
	PlayerID uint64 `gorm:"not null;index:idx_player_id" json:"player_id"`   // 所属玩家ID
	PetID    int32  `gorm:"not null" json:"pet_id"`                          // 宠物配置ID
	Level    int32  `gorm:"not null;default:1" json:"level"`                  // 宠物等级
	Quality  int32  `gorm:"not null;default:0" json:"quality"`                // 宠物品质
	Skills   string `gorm:"type:varchar(128);not null;default:''" json:"skills"` // 宠物技能列表
}

// TableName 指定 PlayerPetORM 对应的数据库表名
func (PlayerPetORM) TableName() string { return "player_pet" }

// DungeonProgressORM 副本进度表持久化模型，对应 dungeon_progress 表。
type DungeonProgressORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`                // 进度记录ID
	PlayerID uint64 `gorm:"not null;uniqueIndex;index:idx_player_id" json:"player_id"` // 所属玩家ID（唯一）
	MaxLayer int32  `gorm:"not null;default:0" json:"max_layer"`               // 最高通关层数
}

// TableName 指定 DungeonProgressORM 对应的数据库表名
func (DungeonProgressORM) TableName() string { return "dungeon_progress" }

// TradeOrderORM 交易订单表持久化模型，对应 trade_order 表。
type TradeOrderORM struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement" json:"id"`                      // 订单ID
	SellerID        uint64 `gorm:"not null;index:idx_seller_id" json:"seller_id"`            // 卖家玩家ID
	EquipID         int32  `gorm:"not null" json:"equip_id"`                                 // 装备配置ID
	Quality         int32  `gorm:"not null" json:"quality"`                                  // 装备品质
	StrengthenLevel int32  `gorm:"not null" json:"strengthen_level"`                         // 强化等级
	Price           int64  `gorm:"not null" json:"price"`                                    // 挂单价格
	Status          int32  `gorm:"not null;default:0;index:idx_status" json:"status"`        // 订单状态：0=在售 1=已售出 2=已下架
	CreatedAt       int64  `gorm:"autoCreateTime" json:"created_at"`                         // 订单创建时间
}

// TableName 指定 TradeOrderORM 对应的数据库表名
func (TradeOrderORM) TableName() string { return "trade_order" }