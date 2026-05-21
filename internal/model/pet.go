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

// PetExploreReward 宠物探险奖励
type PetExploreReward struct {
	Exp        int64 // 获得经验
	Gold       int64 // 获得金币
	DropItemID int32 // 掉落物品ID（0表示无掉落）
	DropCount  int32 // 掉落数量
}

// Pet 宠物实例，玩家可携带宠物进行探索或战斗
type Pet struct {
	mu               sync.RWMutex // 读写锁，保护并发访问
	UID              uint64       // 宠物实例唯一标识
	OwnerID          uint64       // 所属玩家ID
	PetID            int32        // 宠物模板ID
	Name             string       // 宠物名称
	Level            int32        // 宠物等级
	Quality          int32        // 宠物品质（对应品质常量）
	Type             int32        // 宠物类型（对应PetTypeXxx常量）
	Skills           string       // 宠物技能列表（序列化字符串）
	Exploring        bool         // 是否正在探索中
	ExploreStartTime time.Time    // 探索开始时间
	ExploreEndTime   time.Time    // 探索结束时间
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
