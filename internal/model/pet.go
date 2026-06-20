package model

import (
	"math/rand"
	"sync"
	"time"
)

// PetTemplate 宠物模板定义
type PetTemplate struct {
	PetID   int32  // 宠物模板ID
	Name    string // 宠物名称
	Type    int32  // 宠物类型
	Quality int32  // 基础品质
}

// PetTemplates 宠物模板静态数据
var PetTemplates = map[int32]*PetTemplate{
	100: {PetID: 100, Name: "小火龙", Type: PetTypeAttack, Quality: QualityWhite},
	101: {PetID: 101, Name: "铁甲龟", Type: PetTypeDefense, Quality: QualityWhite},
	102: {PetID: 102, Name: "治疗精灵", Type: PetTypeSupport, Quality: QualityWhite},
	103: {PetID: 103, Name: "掠夺者", Type: PetTypePlunder, Quality: QualityWhite},
	200: {PetID: 200, Name: "烈焰龙", Type: PetTypeAttack, Quality: QualityGreen},
	201: {PetID: 201, Name: "玄铁龟", Type: PetTypeDefense, Quality: QualityGreen},
	202: {PetID: 202, Name: "圣光精灵", Type: PetTypeSupport, Quality: QualityGreen},
	300: {PetID: 300, Name: "远古火龙", Type: PetTypeAttack, Quality: QualityBlue},
	301: {PetID: 301, Name: "龙龟", Type: PetTypeDefense, Quality: QualityBlue},
	400: {PetID: 400, Name: "凤凰", Type: PetTypeAttack, Quality: QualityPurple},
	500: {PetID: 500, Name: "神圣巨龙", Type: PetTypeAttack, Quality: QualityOrange},
}

// petEquipBonus 遍历宠物已装备物品，累加属性加成
func (p *Pet) petEquipBonus() (atkBonus, defBonus, hpBonus int64) {
	for _, eq := range p.EquippedItems {
		if eq == nil {
			continue
		}
		tmpl, ok := PetEquipTemplates[eq.ID]
		if !ok {
			continue
		}
		atkBonus += tmpl.AtkBonus
		defBonus += tmpl.DefBonus
		hpBonus += tmpl.HpBonus
	}
	return
}

// CalcPetStats 根据宠物类型、品质、等级计算战斗属性并写入 Pet 实例。
// 攻击型：高攻低防；防御型：低攻高防；辅助型：均衡；掠夺型：偏攻。
func (p *Pet) CalcPetStats() {
	qualityMul := float64(p.Quality+1) * 0.5 // 品质系数：白0.5 绿1.0 蓝1.5 紫2.0 橙2.5
	levelMul := float64(p.Level)

	switch p.Type {
	case PetTypeAttack:
		p.Attack = int64((20 + levelMul*5) * qualityMul)
		p.Defense = int64((5 + levelMul*1) * qualityMul)
		p.MaxHP = int64((100 + levelMul*15) * qualityMul)
	case PetTypeDefense:
		p.Attack = int64((8 + levelMul*2) * qualityMul)
		p.Defense = int64((15 + levelMul*4) * qualityMul)
		p.MaxHP = int64((200 + levelMul*30) * qualityMul)
	case PetTypeSupport:
		p.Attack = int64((12 + levelMul*3) * qualityMul)
		p.Defense = int64((10 + levelMul*2) * qualityMul)
		p.MaxHP = int64((150 + levelMul*20) * qualityMul)
	case PetTypePlunder:
		p.Attack = int64((18 + levelMul*4) * qualityMul)
		p.Defense = int64((6 + levelMul*1) * qualityMul)
		p.MaxHP = int64((80 + levelMul*12) * qualityMul)
	default:
		p.Attack = int64((10 + levelMul*3) * qualityMul)
		p.Defense = int64((8 + levelMul*2) * qualityMul)
		p.MaxHP = int64((100 + levelMul*15) * qualityMul)
	}

	// 叠加装备加成
	equipAtk, equipDef, equipHp := p.petEquipBonus()
	p.Attack += equipAtk
	p.Defense += equipDef
	p.MaxHP += equipHp

	p.HP = p.MaxHP
}

// PetExploreReward 宠物探险奖励
type PetExploreReward struct {
	Exp        int64 // 获得经验
	Gold       int64 // 获得金币
	DropItemID int32 // 掉落物品ID（0表示无掉落）
	DropCount  int32 // 掉落数量
}

// Pet 宠物实例，玩家可携带宠物进行探索或战斗
type Pet struct {
	mu               sync.RWMutex              // 读写锁，保护并发访问
	UID              uint64                    // 宠物实例唯一标识
	OwnerID          uint64                    // 所属玩家ID
	PetID            int32                     // 宠物模板ID
	Name             string                    // 宠物名称
	Level            int32                     // 宠物等级
	Quality          int32                     // 宠物品质（对应品质常量）
	Type             int32                     // 宠物类型（对应PetTypeXxx常量）
	Skills           string                    // 宠物技能列表（序列化字符串）
	Exploring        bool                      // 是否正在探索中
	ExploreStartTime time.Time                 // 探索开始时间
	ExploreEndTime   time.Time                 // 探索结束时间
	EquippedItems    [PetSlotMax]*PetEquipment // 已装备的宠物装备（按槽位索引）
	// 战斗属性（由 CalcPetStats 初始化，不持久化）
	Attack  int64 // 宠物攻击力
	Defense int64 // 宠物防御力
	HP      int64 // 宠物当前生命值
	MaxHP   int64 // 宠物最大生命值
}

// Mu 返回 Pet 的读写锁指针，供外部按需加锁保护并发操作
func (p *Pet) Mu() *sync.RWMutex {
	return &p.mu
}

// CalcExploreReward 根据品质和探索时长计算奖励
func (p *Pet) CalcExploreReward(duration time.Duration) PetExploreReward {
	hours := duration.Hours()
	qualityMul := float64(p.Quality+1) * 1.0
	reward := PetExploreReward{
		Exp:  int64(float64(hours) * 50 * qualityMul),
		Gold: int64(float64(hours) * 30 * qualityMul),
	}
	// 30% 概率掉落物品
	if rand.Float64() < 0.3 {
		reward.DropItemID = int32(2001 + rand.Intn(4))
		reward.DropCount = int32(rand.Intn(3) + 1)
	}
	return reward
}

// 宠物类型常量
const (
	PetTypeAttack  = 0 // 攻击型宠物
	PetTypeDefense = 1 // 防御型宠物
	PetTypeSupport = 2 // 辅助型宠物
	PetTypePlunder = 3 // 掠夺型宠物
)

// 宠物装备槽位常量
const (
	PetSlotCollar    = 0 // 项圈槽位（攻击加成）
	PetSlotArmor     = 1 // 护甲槽位（防御加成）
	PetSlotAccessory = 2 // 饰品槽位（生命加成）
	PetSlotMax       = 3 // 宠物装备槽位总数
)

// PetEquipTemplate 宠物装备模板
type PetEquipTemplate struct {
	ID       int32  // 模板ID
	Name     string // 装备名称
	Slot     int32  // 适配槽位
	Quality  int32  // 品质
	AtkBonus int64  // 攻击加成
	DefBonus int64  // 防御加成
	HpBonus  int64  // 生命加成
	ReqLevel int32  // 需求等级
}

// PetEquipTemplates 宠物装备模板静态数据
var PetEquipTemplates = map[int32]*PetEquipTemplate{
	// 项圈 (Slot 0)
	9001: {ID: 9001, Name: "皮项圈", Slot: PetSlotCollar, Quality: QualityWhite, AtkBonus: 5, DefBonus: 0, HpBonus: 0, ReqLevel: 1},
	9002: {ID: 9002, Name: "铁项圈", Slot: PetSlotCollar, Quality: QualityGreen, AtkBonus: 15, DefBonus: 0, HpBonus: 0, ReqLevel: 10},
	9003: {ID: 9003, Name: "秘银项圈", Slot: PetSlotCollar, Quality: QualityBlue, AtkBonus: 30, DefBonus: 5, HpBonus: 0, ReqLevel: 20},
	9004: {ID: 9004, Name: "龙骨项圈", Slot: PetSlotCollar, Quality: QualityPurple, AtkBonus: 60, DefBonus: 10, HpBonus: 20, ReqLevel: 30},
	// 护甲 (Slot 1)
	9101: {ID: 9101, Name: "皮甲", Slot: PetSlotArmor, Quality: QualityWhite, AtkBonus: 0, DefBonus: 5, HpBonus: 20, ReqLevel: 1},
	9102: {ID: 9102, Name: "铁甲", Slot: PetSlotArmor, Quality: QualityGreen, AtkBonus: 0, DefBonus: 15, HpBonus: 50, ReqLevel: 10},
	9103: {ID: 9103, Name: "秘银甲", Slot: PetSlotArmor, Quality: QualityBlue, AtkBonus: 0, DefBonus: 30, HpBonus: 100, ReqLevel: 20},
	9104: {ID: 9104, Name: "龙鳞甲", Slot: PetSlotArmor, Quality: QualityPurple, AtkBonus: 5, DefBonus: 60, HpBonus: 200, ReqLevel: 30},
	// 饰品 (Slot 2)
	9201: {ID: 9201, Name: "铜铃铛", Slot: PetSlotAccessory, Quality: QualityWhite, AtkBonus: 2, DefBonus: 2, HpBonus: 10, ReqLevel: 1},
	9202: {ID: 9202, Name: "银铃铛", Slot: PetSlotAccessory, Quality: QualityGreen, AtkBonus: 5, DefBonus: 5, HpBonus: 30, ReqLevel: 10},
	9203: {ID: 9203, Name: "金铃铛", Slot: PetSlotAccessory, Quality: QualityBlue, AtkBonus: 10, DefBonus: 10, HpBonus: 60, ReqLevel: 20},
	9204: {ID: 9204, Name: "龙魂铃铛", Slot: PetSlotAccessory, Quality: QualityPurple, AtkBonus: 20, DefBonus: 20, HpBonus: 120, ReqLevel: 30},
}

// PetEquipment 宠物装备实例
type PetEquipment struct {
	ID      int32 // 装备模板ID
	Slot    int32 // 装备槽位
	Quality int32 // 品质
}

// PetMaxExploreHours 宠物单次探索最大时长（小时）
const PetMaxExploreHours = 12

// PlayerPetORM 玩家宠物表持久化模型，对应 player_pet 表。
type PlayerPetORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID uint64 `gorm:"not null;index:idx_player_id" json:"player_id"`
	PetID    int32  `gorm:"not null" json:"pet_id"`
	Level    int32  `gorm:"not null;default:1" json:"level"`
	Quality  int32  `gorm:"not null;default:0" json:"quality"`
	Skills   string `gorm:"type:varchar(128);not null;default:''" json:"skills"`
}

// TableName 指定 PlayerPetORM 对应的数据库表名
func (PlayerPetORM) TableName() string { return "player_pet" }
