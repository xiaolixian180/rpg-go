package model

import (
	"sync"
	"time"
)

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

// Player 玩家核心数据结构，保存玩家在游戏中的所有持久化属性和运行时状态。
// 使用读写锁(mu)保证并发安全，所有字段读写前必须加锁。
type Player struct {
	mu              sync.RWMutex        // 读写锁，保护Player所有字段的并发访问安全
	ID              uint64              // 玩家唯一ID，对应数据库主键
	Name            string              // 玩家角色名
	Class           int32               // 职业类型（对应ClassXxx常量）
	Level           int32               // 当前等级，影响生命值上限和基础伤害
	Exp             int64               // 当前经验值，达到升级阈值后等级+1
	Gold            int64               // 金币余额，用于购买商品、强化装备等
	Honor           int32               // 荣誉值，通过PvP击杀获得，可用于荣誉商店兑换
	KillValue       int32               // 杀戮值（PK值），每PvP击杀一名玩家+1，达到红名阈值后成为红名
	Str             int32               // 力量属性，影响物理伤害和生命值上限
	Agi             int32               // 敏捷属性，影响闪避率和暴击率
	Int             int32               // 智力属性，影响魔法伤害和魔法值上限
	Con             int32               // 体质属性，影响生命值上限
	Def             int32               // 防御属性，影响物理减伤
	AttrPoints      int32               // 可分配属性点，升级时获得，玩家可自由分配到力量/敏捷/智力/体质
	MaxLayer        int32               // 地下城已通关最高层数，决定玩家可进入的楼层范围
	Hp              int64               // 当前生命值，降为0时角色死亡
	MaxHp           int64               // 生命值上限，由等级、体质、力量共同计算得出
	X               float64             // 角色在当前地图中的X坐标
	Y               float64             // 角色在当前地图中的Y坐标
	Layer           int32               // 当前所在的地下城层数，0表示不在地下城中
	Online          bool                // 是否在线，登录时设为true，登出时设为false
	InvincibleUntil time.Time           // 无敌状态截止时间，PvP被击杀后获得30秒无敌保护
	ActivePet       *Pet                // 当前出战宠物，nil表示无宠物出战
	AutoBattle      bool                // 是否开启自动战斗
	EquippedItems   [SlotMax]*Equipment // 已装备的装备（按槽位索引），登录时从DB加载
	Mp              int64               // 当前魔法值
	MaxMp           int64               // 魔法值上限
	Items           map[uint32]int32    // 背包物品（item_id -> 数量），登录时初始化默认物品
	Dirty           bool                // 是否有未持久化的变更（定时存档标记）
}

// ExpTable 升级所需经验表，索引为当前等级，值为升到下一级所需经验
var ExpTable = [...]int64{
	0, 100, 250, 450, 700, 1000, 1400, 1900, 2500, 3200, // 1~10
	4000, 4900, 5900, 7000, 8200, 9500, 11000, 12600, 14300, 16100, // 11~20
	18000, 20000, 22100, 24300, 26600, 29000, 31500, 34100, 36800, 39600, // 21~30
	42500, 45500, 48600, 51800, 55100, 58500, 62000, 65600, 69300, 73100, // 31~40
	77000, 81000, 85100, 89300, 93600, 98000, 102500, 107100, 111800, 116600, // 41~50
	121500, 126500, 131600, 136800, 142100, 147500, 153000, 158600, 164300, 170000, // 51~60
}

// equipBonus 遍历已装备物品，累加基础属性和强化加成
func (p *Player) equipBonus() (atkBonus, defBonus, hpBonus int64) {
	for _, eq := range p.EquippedItems {
		if eq == nil {
			continue
		}
		tmpl, ok := EquipTemplates[eq.EquipID]
		if !ok {
			continue
		}
		// 强化加成：每级增加基础属性的10%
		mul := 1.0 + float64(eq.StrengthenLevel)*0.1
		atkBonus += int64(float64(tmpl.BaseAtk) * mul)
		defBonus += int64(float64(tmpl.BaseDef) * mul)
		hpBonus += int64(float64(tmpl.BaseHp) * mul)
	}
	return
}

// CalcMaxHp 计算玩家的生命值上限。
// 计算公式：基础值(100 + 等级*20) + 体质*10 + 力量*5 + 防御*3 + 装备生命值加成
func (p *Player) CalcMaxHp() int64 {
	base := int64(100 + p.Level*20)
	_, _, equipHp := p.equipBonus()
	return base + int64(p.Con)*10 + int64(p.Str)*5 + int64(p.Def)*3 + equipHp
}

// CalcMaxMp 计算玩家的魔法值上限。
// 公式：基础值(50 + 等级*10) + 智力*15
func (p *Player) CalcMaxMp() int64 {
	return int64(50+p.Level*10) + int64(p.Int)*15
}

// CalcAttack 计算玩家攻击力
// 公式：力量*2 + 等级*5 + 敏捷*0.5 + 装备攻击加成
func (p *Player) CalcAttack() int64 {
	equipAtk, _, _ := p.equipBonus()
	return int64(p.Str)*2 + int64(p.Level)*5 + int64(float64(p.Agi)*0.5) + equipAtk
}

// CalcDefense 计算玩家防御力
// 公式：防御属性*3 + 体质*1 + 等级*2 + 装备防御加成
func (p *Player) CalcDefense() int64 {
	_, equipDef, _ := p.equipBonus()
	return int64(p.Def)*3 + int64(p.Con) + int64(p.Level)*2 + equipDef
}

// CalcDodgeRate 计算闪避率（0~0.3）
func (p *Player) CalcDodgeRate() float64 {
	rate := float64(p.Agi) * 0.005
	if rate > 0.3 {
		rate = 0.3
	}
	return rate
}

// CalcCritRate 计算暴击率（0~0.5）
func (p *Player) CalcCritRate() float64 {
	rate := float64(p.Agi)*0.003 + float64(p.Str)*0.001
	if rate > 0.5 {
		rate = 0.5
	}
	return rate
}

// CalcCritDamage 计算暴击伤害倍率（基础1.5）
func (p *Player) CalcCritDamage() float64 {
	return 1.5 + float64(p.Str)*0.01
}

// CalcSkillEffectBonus 计算装备技能特效增伤总和
// skillID=0 表示所有技能，否则只统计匹配指定技能的增伤
func (p *Player) CalcSkillEffectBonus(skillID int32) float64 {
	var total float64
	for _, eq := range p.EquippedItems {
		if eq == nil {
			continue
		}
		tmpl, ok := EquipTemplates[eq.EquipID]
		if !ok {
			continue
		}
		for _, eff := range tmpl.SkillEffects {
			if eff.EffectType == 1 { // 技能增伤
				if eff.SkillID == 0 || eff.SkillID == skillID {
					total += eff.Value
				}
			}
		}
	}
	return total
}

// IsInvincible 判断玩家当前是否处于无敌状态。
// 无敌状态在PvP被击杀后持续30秒，期间无法被其他玩家攻击。
func (p *Player) IsInvincible() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Now().Before(p.InvincibleUntil)
}

// Mu 返回 Player 的读写锁指针，供外部按需加锁保护并发操作
func (p *Player) Mu() *sync.RWMutex {
	return &p.mu
}

// PlayerORM 玩家表持久化模型，对应 player 表。
type PlayerORM struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string `gorm:"type:varchar(32);uniqueIndex;not null" json:"name"`
	Class      int32  `gorm:"not null;default:0" json:"class"`
	Level      int32  `gorm:"not null;default:1" json:"level"`
	Exp        int64  `gorm:"not null;default:0" json:"exp"`
	Gold       int64  `gorm:"not null;default:0" json:"gold"`
	Honor      int32  `gorm:"not null;default:0" json:"honor"`
	KillValue  int32  `gorm:"column:kill_value;not null;default:0" json:"kill_value"`
	Str        int32  `gorm:"not null;default:0" json:"str"`
	Agi        int32  `gorm:"not null;default:0" json:"agi"`
	Int        int32  `gorm:"column:int_attr;not null;default:0" json:"int_attr"`
	Con        int32  `gorm:"not null;default:0" json:"con"`
	Def        int32  `gorm:"not null;default:0" json:"def"`
	AttrPoints int32  `gorm:"not null;default:0" json:"attr_points"`
	CreatedAt  int64  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定 PlayerORM 对应的数据库表名
func (PlayerORM) TableName() string { return "player" }

// ==================== 物品系统 ====================

// ItemTemplate 物品模板定义
type ItemTemplate struct {
	ID    uint32 // 物品ID
	Name  string // 物品名称
	HPVal int64  // 恢复HP值，0=不恢复HP
	MPVal int64  // 恢复MP值，0=不恢复MP
}

// ItemTemplates 物品模板表（硬编码，原型阶段）
var ItemTemplates = map[uint32]*ItemTemplate{
	1: {1, "小型生命药水", 100, 0},
	2: {2, "大型生命药水", 500, 0},
	3: {3, "小型魔法药水", 0, 100},
	4: {4, "大型魔法药水", 0, 500},
	5: {5, "全能药水", 300, 300},
}

// InitItems 初始化背包物品（登录时调用），如果背包为空则赠送初始药水
func (p *Player) InitItems() {
	if p.Items == nil {
		p.Items = make(map[uint32]int32)
	}
	// 新玩家赠送初始药水
	if len(p.Items) == 0 {
		p.Items[1] = 10 // 10瓶小型HP药水
		p.Items[3] = 5  // 5瓶小型MP药水
	}
	// 同步MP上限（DB不存MP，每次登录重算）
	p.MaxMp = p.CalcMaxMp()
	if p.Mp > p.MaxMp {
		p.Mp = p.MaxMp
	}
}
