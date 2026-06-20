package model

import (
	"math/rand"
	"sync"
)

// DungeonLayer 副本单层结构，包含该层的怪物、玩家和资源
type DungeonLayer struct {
	Layer     int32                // 层数编号
	IsBoss    bool                 // 是否为Boss层（每10层为Boss层）
	Monsters  map[uint64]*Monster  // 该层所有怪物，key为怪物ID
	Players   map[uint64]*Player   // 该层所有玩家，key为玩家ID
	Resources map[uint64]*Resource // 该层所有可采集资源，key为资源ID
	mu        sync.RWMutex         // 读写锁，保护并发访问
}

// Mu 返回 DungeonLayer 的读写锁指针，供外部按需加锁保护并发操作
func (d *DungeonLayer) Mu() *sync.RWMutex {
	return &d.mu
}

// 怪物AI状态常量
const (
	MonsterAIIdle   int32 = 0 // 空闲/巡逻
	MonsterAIAlert  int32 = 1 // 警戒（检测到玩家进入仇恨范围）
	MonsterAIChase  int32 = 2 // 追击（向玩家移动）
	MonsterAIAttack int32 = 3 // 攻击（在攻击范围内）
	MonsterAIReturn int32 = 4 // 返回（玩家脱离追击范围，回到出生点）
)

// 怪物行为类型常量
const (
	MonsterBehaviorMelee  int32 = 0 // 近战型：冲向玩家近距离攻击
	MonsterBehaviorRanged int32 = 1 // 远程型：保持距离远程攻击
	MonsterBehaviorTank   int32 = 2 // 坦克型：缓慢但高伤害
	MonsterBehaviorHealer int32 = 3 // 辅助型：治疗附近怪物
)

// ==================== 宝可梦风格元素系统 ====================

// 怪物元素类型常量（参考宝可梦属性系统）
const (
	ElementNone     int32 = 0 // 无属性
	ElementFire     int32 = 1 // 火系 - 高攻击，克制草系
	ElementWater    int32 = 2 // 水系 - 均衡，克制火系
	ElementGrass    int32 = 3 // 草系 - 辅助回复，克制水系
	ElementElectric int32 = 4 // 雷系 - 高速度，克制水系
	ElementDark     int32 = 5 // 暗系 - 隐身偷袭
	ElementLight    int32 = 6 // 光系 - 防御治疗
	ElementEarth    int32 = 7 // 地系 - 高防御
	ElementWind     int32 = 8 // 风系 - 高闪避
)

// ElementName 元素类型中文名称映射
var ElementName = map[int32]string{
	ElementNone:     "无",
	ElementFire:     "火",
	ElementWater:    "水",
	ElementGrass:    "草",
	ElementElectric: "雷",
	ElementDark:     "暗",
	ElementLight:    "光",
	ElementEarth:    "地",
	ElementWind:     "风",
}

// ElementChart 元素克制表（攻击方属性 -> 防御方属性 -> 伤害倍率）
// 1.5 = 克制，0.75 = 被克制，1.0 = 正常
var ElementChart = map[int32]map[int32]float64{
	ElementFire:     {ElementGrass: 1.5, ElementWater: 0.75, ElementFire: 0.75},
	ElementWater:    {ElementFire: 1.5, ElementGrass: 0.75, ElementWater: 0.75, ElementElectric: 0.75},
	ElementGrass:    {ElementWater: 1.5, ElementFire: 0.75, ElementGrass: 0.75},
	ElementElectric: {ElementWater: 1.5, ElementElectric: 0.75, ElementEarth: 0.75},
	ElementDark:     {ElementLight: 1.5, ElementDark: 0.75},
	ElementLight:    {ElementDark: 1.5, ElementLight: 0.75},
	ElementEarth:    {ElementElectric: 1.5, ElementWind: 0.75, ElementEarth: 0.75},
	ElementWind:     {ElementEarth: 1.5, ElementWind: 0.75},
}

// GetElementMultiplier 获取元素克制倍率
func GetElementMultiplier(atkElement, defElement int32) float64 {
	if chart, ok := ElementChart[atkElement]; ok {
		if mul, ok := chart[defElement]; ok {
			return mul
		}
	}
	return 1.0
}

// 怪物稀有度常量（参考宝可梦稀有度）
const (
	RarityCommon    int32 = 0 // 普通 - 白色
	RarityUncommon  int32 = 1 // 优秀 - 绿色
	RarityRare      int32 = 2 // 稀有 - 蓝色
	RarityEpic      int32 = 3 // 史诗 - 紫色
	RarityLegendary int32 = 4 // 传说 - 橙色
)

// RarityName 稀有度中文名称映射
var RarityName = map[int32]string{
	RarityCommon:    "普通",
	RarityUncommon:  "优秀",
	RarityRare:      "稀有",
	RarityEpic:      "史诗",
	RarityLegendary: "传说",
}

// MonsterSkill 怪物技能（参考宝可梦技能系统）
type MonsterSkill struct {
	SkillID  int32   // 技能ID
	Name     string  // 技能名称
	Element  int32   // 技能元素类型
	Power    int64   // 技能威力
	Accuracy float64 // 命中率（0~1）
	CD       float64 // 冷却时间（秒）
	Range    float64 // 释放距离
	Effect   string  // 特殊效果描述
}

// Monster 副本中的普通怪物
type Monster struct {
	ID         uint64  // 怪物唯一标识
	Name       string  // 怪物名称
	Hp         int64   // 当前生命值
	MaxHp      int64   // 最大生命值
	Atk        int64   // 攻击力
	Def        int64   // 防御力
	ExpReward  int64   // 击杀后奖励的经验值
	GoldReward int64   // 击杀后奖励的金币数
	X          float64 // 地图中的X坐标
	Y          float64 // 地图中的Y坐标
	Dead       bool    // 是否已死亡（防止并发攻击重复发奖）
	Elite      bool    // 是否为精英怪（属性增强，掉落更好）
	// 宝可梦风格属性
	Element int32          // 元素类型（ElementXxx）
	Rarity  int32          // 稀有度（RarityXxx）
	Skills  []MonsterSkill // 技能列表
	// AI行为字段
	Behavior    int32        // 行为类型（MonsterBehaviorXxx）
	AIState     int32        // 当前AI状态（MonsterAIXxx）
	SpawnX      float64      // 出生点X坐标（用于返回）
	SpawnY      float64      // 出生点Y坐标（用于返回）
	AggroRange  float64      // 仇恨范围（检测玩家的距离）
	AtkRange    float64      // 攻击范围（可发动攻击的距离）
	AtkCD       float64      // 攻击冷却时间（秒）
	LastAtkTime int64        // 上次攻击时间戳（Unix秒）
	TargetID    uint64       // 当前仇恨目标玩家ID
	mu          sync.RWMutex // 读写锁，保护并发访问
}

// Mu 返回 Monster 的读写锁指针，供外部按需加锁保护并发操作
func (m *Monster) Mu() *sync.RWMutex {
	return &m.mu
}

// Resource 副本中可采集的资源点
type Resource struct {
	ID        uint64       // 资源唯一标识
	Type      int32        // 资源类型（0=矿石 1=草药 2=木材）
	Name      string       // 资源名称
	ItemID    int32        // 产出的物品模板ID
	Count     int32        // 产出数量
	X         float64      // 地图中的X坐标
	Y         float64      // 地图中的Y坐标
	Harvested bool         // 是否已被采集
	mu        sync.RWMutex // 读写锁，保护 Harvested 等字段的并发访问
}

// Mu 返回 Resource 的读写锁指针，供外部按需加锁保护并发操作
func (r *Resource) Mu() *sync.RWMutex {
	return &r.mu
}

// MonsterTemplate 怪物模板
type MonsterTemplate struct {
	ID         uint64         // 模板ID
	Name       string         // 怪物名称
	Hp         int64          // 基础HP
	Atk        int64          // 基础攻击
	Def        int64          // 基础防御
	ExpReward  int64          // 经验奖励
	GoldReward int64          // 金币奖励
	Elite      bool           // 是否为精英怪
	Element    int32          // 元素类型（ElementXxx）
	Rarity     int32          // 稀有度（RarityXxx）
	Skills     []MonsterSkill // 技能列表
	Behavior   int32          // 行为类型（MonsterBehaviorXxx，默认Melee）
	AggroRange float64        // 仇恨范围（默认30）
	AtkRange   float64        // 攻击范围（默认5）
	AtkCD      float64        // 攻击冷却秒数（默认3）
}

// MonsterTemplates 按层数分组的怪物模板（宝可梦风格）
var MonsterTemplates = map[int32][]*MonsterTemplate{
	// 1~10层 翠绿森林（草系/水系为主）
	1: {{ID: 10001, Name: "水滴史莱姆", Hp: 60, Atk: 5, Def: 2, ExpReward: 10, GoldReward: 5,
		Element: ElementWater, Rarity: RarityCommon, Behavior: MonsterBehaviorMelee, AggroRange: 20, AtkRange: 3, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1001, Name: "水枪", Element: ElementWater, Power: 8, Accuracy: 0.95, CD: 2, Range: 5}}}},
	2: {{ID: 10002, Name: "森林狼", Hp: 80, Atk: 10, Def: 5, ExpReward: 20, GoldReward: 8,
		Element: ElementNone, Rarity: RarityCommon, Behavior: MonsterBehaviorMelee, AggroRange: 30, AtkRange: 4, AtkCD: 1.5,
		Skills: []MonsterSkill{{SkillID: 1002, Name: "咬咬", Element: ElementNone, Power: 12, Accuracy: 0.9, CD: 1.5, Range: 4}}}},
	3: {{ID: 10003, Name: "毒孢子", Hp: 60, Atk: 8, Def: 3, ExpReward: 15, GoldReward: 6,
		Element: ElementGrass, Rarity: RarityCommon, Behavior: MonsterBehaviorTank, AggroRange: 15, AtkRange: 5, AtkCD: 4,
		Skills: []MonsterSkill{{SkillID: 1003, Name: "毒粉", Element: ElementGrass, Power: 5, Accuracy: 0.85, CD: 4, Range: 6, Effect: "中毒"}}},
		{ID: 10053, Name: "★毒孢子王", Hp: 200, Atk: 20, Def: 10, ExpReward: 60, GoldReward: 25,
			Elite: true, Element: ElementGrass, Rarity: RarityRare, Behavior: MonsterBehaviorHealer, AggroRange: 25, AtkRange: 8, AtkCD: 3,
			Skills: []MonsterSkill{{SkillID: 1003, Name: "毒粉", Element: ElementGrass, Power: 5, Accuracy: 0.85, CD: 4, Range: 6, Effect: "中毒"},
				{SkillID: 1031, Name: "孢子治愈", Element: ElementGrass, Power: 0, Accuracy: 1.0, CD: 8, Range: 10, Effect: "治疗"}}}},
	4: {{ID: 10004, Name: "哥布林", Hp: 100, Atk: 12, Def: 6, ExpReward: 25, GoldReward: 10,
		Element: ElementNone, Rarity: RarityCommon, Behavior: MonsterBehaviorMelee, AggroRange: 25, AtkRange: 4, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1004, Name: "棍击", Element: ElementNone, Power: 15, Accuracy: 0.9, CD: 2, Range: 4}}}},
	5: {{ID: 10005, Name: "树精", Hp: 150, Atk: 15, Def: 10, ExpReward: 40, GoldReward: 15,
		Element: ElementGrass, Rarity: RarityUncommon, Behavior: MonsterBehaviorTank, AggroRange: 20, AtkRange: 6, AtkCD: 4,
		Skills: []MonsterSkill{{SkillID: 1005, Name: "藤鞭", Element: ElementGrass, Power: 18, Accuracy: 0.85, CD: 4, Range: 6}}},
		{ID: 10050, Name: "★狂暴树精", Hp: 400, Atk: 30, Def: 20, ExpReward: 120, GoldReward: 45,
			Elite: true, Element: ElementGrass, Rarity: RarityEpic, Behavior: MonsterBehaviorTank, AggroRange: 30, AtkRange: 8, AtkCD: 3,
			Skills: []MonsterSkill{{SkillID: 1005, Name: "藤鞭", Element: ElementGrass, Power: 18, Accuracy: 0.85, CD: 4, Range: 6},
				{SkillID: 1050, Name: "地震", Element: ElementEarth, Power: 30, Accuracy: 0.8, CD: 6, Range: 10}}}},
	6: {{ID: 10006, Name: "毒蛛", Hp: 120, Atk: 18, Def: 8, ExpReward: 35, GoldReward: 12,
		Element: ElementDark, Rarity: RarityCommon, Behavior: MonsterBehaviorMelee, AggroRange: 25, AtkRange: 4, AtkCD: 1.5,
		Skills: []MonsterSkill{{SkillID: 1006, Name: "毒咬", Element: ElementDark, Power: 14, Accuracy: 0.9, CD: 1.5, Range: 4, Effect: "中毒"}}}},
	7: {{ID: 10007, Name: "光辉独角兽", Hp: 200, Atk: 20, Def: 12, ExpReward: 50, GoldReward: 20,
		Element: ElementLight, Rarity: RarityRare, Behavior: MonsterBehaviorMelee, AggroRange: 30, AtkRange: 5, AtkCD: 2.5,
		Skills: []MonsterSkill{{SkillID: 1007, Name: "光之冲击", Element: ElementLight, Power: 22, Accuracy: 0.9, CD: 2.5, Range: 5}}},
		{ID: 10057, Name: "★暗影独角兽", Hp: 550, Atk: 40, Def: 25, ExpReward: 150, GoldReward: 60,
			Elite: true, Element: ElementDark, Rarity: RarityLegendary, Behavior: MonsterBehaviorMelee, AggroRange: 35, AtkRange: 6, AtkCD: 2,
			Skills: []MonsterSkill{{SkillID: 1007, Name: "光之冲击", Element: ElementLight, Power: 22, Accuracy: 0.9, CD: 2.5, Range: 5},
				{SkillID: 1057, Name: "暗影突袭", Element: ElementDark, Power: 35, Accuracy: 0.85, CD: 4, Range: 8}}}},
	8: {{ID: 10008, Name: "风之精灵", Hp: 180, Atk: 25, Def: 10, ExpReward: 55, GoldReward: 22,
		Element: ElementWind, Rarity: RarityUncommon, Behavior: MonsterBehaviorRanged, AggroRange: 40, AtkRange: 20, AtkCD: 3,
		Skills: []MonsterSkill{{SkillID: 1008, Name: "风刃", Element: ElementWind, Power: 20, Accuracy: 0.85, CD: 3, Range: 20}}}},
	9: {{ID: 10009, Name: "岩石守卫", Hp: 250, Atk: 28, Def: 15, ExpReward: 65, GoldReward: 25,
		Element: ElementEarth, Rarity: RarityUncommon, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 6, AtkCD: 3.5,
		Skills: []MonsterSkill{{SkillID: 1009, Name: "岩石封锁", Element: ElementEarth, Power: 25, Accuracy: 0.8, CD: 3.5, Range: 6, Effect: "减速"}}}},
	// 11~20层 腐蚀沼泽（水系/暗系为主）
	11: {{ID: 10011, Name: "沼泽蜥蜴", Hp: 300, Atk: 30, Def: 18, ExpReward: 80, GoldReward: 30,
		Element: ElementWater, Rarity: RarityCommon, Behavior: MonsterBehaviorMelee, AggroRange: 25, AtkRange: 4, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1011, Name: "水之尾", Element: ElementWater, Power: 28, Accuracy: 0.9, CD: 2, Range: 4}}}},
	12: {{ID: 10012, Name: "毒蛇女王", Hp: 350, Atk: 35, Def: 20, ExpReward: 100, GoldReward: 35,
		Element: ElementDark, Rarity: RarityRare, Behavior: MonsterBehaviorRanged, AggroRange: 35, AtkRange: 15, AtkCD: 2.5,
		Skills: []MonsterSkill{{SkillID: 1012, Name: "毒液喷射", Element: ElementDark, Power: 30, Accuracy: 0.85, CD: 2.5, Range: 15, Effect: "中毒"}}}},
	13: {{ID: 10013, Name: "腐烂树精", Hp: 400, Atk: 38, Def: 25, ExpReward: 110, GoldReward: 40,
		Element: ElementGrass, Rarity: RarityRare, Behavior: MonsterBehaviorTank, AggroRange: 20, AtkRange: 6, AtkCD: 4,
		Skills: []MonsterSkill{{SkillID: 1013, Name: "腐烂之触", Element: ElementGrass, Power: 32, Accuracy: 0.85, CD: 4, Range: 6, Effect: "中毒"}}},
		{ID: 10063, Name: "★远古腐树", Hp: 1000, Atk: 70, Def: 45, ExpReward: 300, GoldReward: 120,
			Elite: true, Element: ElementGrass, Rarity: RarityLegendary, Behavior: MonsterBehaviorHealer, AggroRange: 30, AtkRange: 10, AtkCD: 3,
			Skills: []MonsterSkill{{SkillID: 1013, Name: "腐烂之触", Element: ElementGrass, Power: 32, Accuracy: 0.85, CD: 4, Range: 6, Effect: "中毒"},
				{SkillID: 1063, Name: "生命汲取", Element: ElementGrass, Power: 40, Accuracy: 0.8, CD: 6, Range: 10, Effect: "吸血"}}}},
	14: {{ID: 10014, Name: "沼泽巨蛙", Hp: 380, Atk: 40, Def: 22, ExpReward: 120, GoldReward: 42,
		Element: ElementWater, Rarity: RarityUncommon, Behavior: MonsterBehaviorMelee, AggroRange: 30, AtkRange: 5, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1014, Name: "舌头卷击", Element: ElementWater, Power: 35, Accuracy: 0.9, CD: 2, Range: 5}}}},
	15: {{ID: 10015, Name: "暗影蛇", Hp: 450, Atk: 45, Def: 28, ExpReward: 140, GoldReward: 50,
		Element: ElementDark, Rarity: RarityRare, Behavior: MonsterBehaviorMelee, AggroRange: 30, AtkRange: 4, AtkCD: 1.5,
		Skills: []MonsterSkill{{SkillID: 1015, Name: "暗影缠绕", Element: ElementDark, Power: 38, Accuracy: 0.9, CD: 1.5, Range: 4}}},
		{ID: 10051, Name: "★暗影蛇王", Hp: 1200, Atk: 90, Def: 50, ExpReward: 400, GoldReward: 150,
			Elite: true, Element: ElementDark, Rarity: RarityLegendary, Behavior: MonsterBehaviorMelee, AggroRange: 35, AtkRange: 6, AtkCD: 1.5,
			Skills: []MonsterSkill{{SkillID: 1015, Name: "暗影缠绕", Element: ElementDark, Power: 38, Accuracy: 0.9, CD: 1.5, Range: 4},
				{SkillID: 1051, Name: "剧毒之牙", Element: ElementDark, Power: 50, Accuracy: 0.85, CD: 3, Range: 6, Effect: "剧毒"}}}},
	16: {{ID: 10016, Name: "腐化骑士", Hp: 500, Atk: 48, Def: 30, ExpReward: 160, GoldReward: 55,
		Element: ElementDark, Rarity: RarityUncommon, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 6, AtkCD: 3,
		Skills: []MonsterSkill{{SkillID: 1016, Name: "暗影斩", Element: ElementDark, Power: 42, Accuracy: 0.85, CD: 3, Range: 6}}}},
	17: {{ID: 10017, Name: "沼泽领主", Hp: 550, Atk: 52, Def: 35, ExpReward: 180, GoldReward: 60,
		Element: ElementWater, Rarity: RarityRare, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 7, AtkCD: 3.5,
		Skills: []MonsterSkill{{SkillID: 1017, Name: "沼泽爆发", Element: ElementWater, Power: 45, Accuracy: 0.8, CD: 3.5, Range: 7, Effect: "减速"}}}},
	18: {{ID: 10018, Name: "暗夜猎手", Hp: 520, Atk: 55, Def: 32, ExpReward: 190, GoldReward: 65,
		Element: ElementDark, Rarity: RarityRare, Behavior: MonsterBehaviorRanged, AggroRange: 40, AtkRange: 18, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1018, Name: "暗影箭", Element: ElementDark, Power: 48, Accuracy: 0.85, CD: 2, Range: 18}}},
		{ID: 10068, Name: "★暗夜猎手首领", Hp: 1400, Atk: 100, Def: 55, ExpReward: 500, GoldReward: 180,
			Elite: true, Element: ElementDark, Rarity: RarityLegendary, Behavior: MonsterBehaviorRanged, AggroRange: 45, AtkRange: 22, AtkCD: 2,
			Skills: []MonsterSkill{{SkillID: 1018, Name: "暗影箭", Element: ElementDark, Power: 48, Accuracy: 0.85, CD: 2, Range: 18},
				{SkillID: 1068, Name: "暗影风暴", Element: ElementDark, Power: 60, Accuracy: 0.75, CD: 5, Range: 25}}}},
	19: {{ID: 10019, Name: "沼泽巫妖", Hp: 600, Atk: 58, Def: 38, ExpReward: 210, GoldReward: 70,
		Element: ElementDark, Rarity: RarityEpic, Behavior: MonsterBehaviorRanged, AggroRange: 35, AtkRange: 18, AtkCD: 3,
		Skills: []MonsterSkill{{SkillID: 1019, Name: "暗影爆破", Element: ElementDark, Power: 52, Accuracy: 0.8, CD: 3, Range: 18}}}},
	// 21~30层 烈焰火山（火系/雷系为主）
	21: {{ID: 10021, Name: "火焰精灵", Hp: 700, Atk: 60, Def: 40, ExpReward: 250, GoldReward: 80,
		Element: ElementFire, Rarity: RarityUncommon, Behavior: MonsterBehaviorRanged, AggroRange: 35, AtkRange: 15, AtkCD: 2.5,
		Skills: []MonsterSkill{{SkillID: 1021, Name: "火球术", Element: ElementFire, Power: 55, Accuracy: 0.85, CD: 2.5, Range: 15}}}},
	22: {{ID: 10022, Name: "熔岩巨人", Hp: 800, Atk: 65, Def: 50, ExpReward: 300, GoldReward: 90,
		Element: ElementFire, Rarity: RarityRare, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 8, AtkCD: 4,
		Skills: []MonsterSkill{{SkillID: 1022, Name: "熔岩拳", Element: ElementFire, Power: 60, Accuracy: 0.8, CD: 4, Range: 8}}},
		{ID: 10072, Name: "★熔岩巨像", Hp: 2000, Atk: 120, Def: 80, ExpReward: 800, GoldReward: 280,
			Elite: true, Element: ElementFire, Rarity: RarityLegendary, Behavior: MonsterBehaviorTank, AggroRange: 30, AtkRange: 10, AtkCD: 3.5,
			Skills: []MonsterSkill{{SkillID: 1022, Name: "熔岩拳", Element: ElementFire, Power: 60, Accuracy: 0.8, CD: 4, Range: 10},
				{SkillID: 1072, Name: "火山爆发", Element: ElementFire, Power: 80, Accuracy: 0.7, CD: 8, Range: 15}}}},
	23: {{ID: 10023, Name: "火蜥蜴", Hp: 750, Atk: 68, Def: 45, ExpReward: 320, GoldReward: 95,
		Element: ElementFire, Rarity: RarityUncommon, Behavior: MonsterBehaviorMelee, AggroRange: 30, AtkRange: 5, AtkCD: 2,
		Skills: []MonsterSkill{{SkillID: 1023, Name: "火焰牙", Element: ElementFire, Power: 58, Accuracy: 0.9, CD: 2, Range: 5}}}},
	24: {{ID: 10024, Name: "烈焰骑士", Hp: 850, Atk: 72, Def: 52, ExpReward: 350, GoldReward: 100,
		Element: ElementFire, Rarity: RarityRare, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 6, AtkCD: 3,
		Skills: []MonsterSkill{{SkillID: 1024, Name: "烈焰斩", Element: ElementFire, Power: 65, Accuracy: 0.85, CD: 3, Range: 6}}}},
	25: {{ID: 10025, Name: "火山守卫", Hp: 900, Atk: 75, Def: 55, ExpReward: 380, GoldReward: 110,
		Element: ElementEarth, Rarity: RarityRare, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 7, AtkCD: 3.5,
		Skills: []MonsterSkill{{SkillID: 1025, Name: "岩石重击", Element: ElementEarth, Power: 68, Accuracy: 0.8, CD: 3.5, Range: 7}}},
		{ID: 10052, Name: "★熔岩领主", Hp: 2500, Atk: 150, Def: 100, ExpReward: 1000, GoldReward: 350,
			Elite: true, Element: ElementFire, Rarity: RarityLegendary, Behavior: MonsterBehaviorTank, AggroRange: 35, AtkRange: 10, AtkCD: 3,
			Skills: []MonsterSkill{{SkillID: 1025, Name: "岩石重击", Element: ElementEarth, Power: 68, Accuracy: 0.8, CD: 3.5, Range: 10},
				{SkillID: 1052, Name: "烈焰吐息", Element: ElementFire, Power: 90, Accuracy: 0.75, CD: 6, Range: 15}}}},
	26: {{ID: 10026, Name: "炎魔", Hp: 1000, Atk: 80, Def: 58, ExpReward: 420, GoldReward: 120,
		Element: ElementFire, Rarity: RarityEpic, Behavior: MonsterBehaviorRanged, AggroRange: 35, AtkRange: 16, AtkCD: 2.5,
		Skills: []MonsterSkill{{SkillID: 1026, Name: "地狱火", Element: ElementFire, Power: 72, Accuracy: 0.8, CD: 2.5, Range: 16}}}},
	27: {{ID: 10027, Name: "火焰领主", Hp: 1100, Atk: 85, Def: 62, ExpReward: 460, GoldReward: 130,
		Element: ElementFire, Rarity: RarityEpic, Behavior: MonsterBehaviorTank, AggroRange: 25, AtkRange: 8, AtkCD: 3,
		Skills: []MonsterSkill{{SkillID: 1027, Name: "烈焰冲击", Element: ElementFire, Power: 78, Accuracy: 0.8, CD: 3, Range: 8}}},
		{ID: 10077, Name: "★烈焰魔君", Hp: 3000, Atk: 170, Def: 110, ExpReward: 1200, GoldReward: 400,
			Elite: true, Element: ElementFire, Rarity: RarityLegendary, Behavior: MonsterBehaviorRanged, AggroRange: 40, AtkRange: 20, AtkCD: 2,
			Skills: []MonsterSkill{{SkillID: 1027, Name: "烈焰冲击", Element: ElementFire, Power: 78, Accuracy: 0.8, CD: 3, Range: 20},
				{SkillID: 1077, Name: "陨石坠落", Element: ElementFire, Power: 120, Accuracy: 0.6, CD: 10, Range: 30}}}},
	28: {{ID: 10028, Name: "地狱犬", Hp: 1050, Atk: 88, Def: 60, ExpReward: 480, GoldReward: 135,
		Element: ElementFire, Rarity: RarityRare, Behavior: MonsterBehaviorMelee, AggroRange: 35, AtkRange: 5, AtkCD: 1.5,
		Skills: []MonsterSkill{{SkillID: 1028, Name: "烈焰咬", Element: ElementFire, Power: 75, Accuracy: 0.9, CD: 1.5, Range: 5}}}},
	29: {{ID: 10029, Name: "炎龙幼崽", Hp: 1200, Atk: 92, Def: 65, ExpReward: 520, GoldReward: 150,
		Element: ElementFire, Rarity: RarityEpic, Behavior: MonsterBehaviorRanged, AggroRange: 40, AtkRange: 18, AtkCD: 2.5,
		Skills: []MonsterSkill{{SkillID: 1029, Name: "龙息", Element: ElementFire, Power: 82, Accuracy: 0.8, CD: 2.5, Range: 18}}}},
}

// BossTemplates 按层数定义的Boss模板（每10层一个Boss）
var BossTemplates = map[int32]*Boss{
	10: {
		ID: 90010, Name: "森林守护者·古树", Hp: 2000, MaxHp: 2000,
		Layer: 10, X: 50, Y: 50, Cooldown: 300 * 1e9, // 5分钟冷却
		Skills: []BossSkill{
			{SkillID: 9001, Name: "根须缠绕", CD: 8, Range: 15, Damage: 80},
			{SkillID: 9002, Name: "落叶风暴", CD: 12, Range: 20, Damage: 120},
		},
	},
	20: {
		ID: 90020, Name: "沼泽之王·毒龙", Hp: 5000, MaxHp: 5000,
		Layer: 20, X: 50, Y: 50, Cooldown: 300 * 1e9,
		Skills: []BossSkill{
			{SkillID: 9003, Name: "毒雾吐息", CD: 6, Range: 18, Damage: 150},
			{SkillID: 9004, Name: "沼泽爆发", CD: 15, Range: 25, Damage: 250},
		},
	},
	30: {
		ID: 90030, Name: "烈焰领主·炎魔", Hp: 10000, MaxHp: 10000,
		Layer: 30, X: 50, Y: 50, Cooldown: 600 * 1e9, // 10分钟冷却
		Skills: []BossSkill{
			{SkillID: 9005, Name: "烈焰冲击", CD: 5, Range: 15, Damage: 200},
			{SkillID: 9006, Name: "陨石坠落", CD: 20, Range: 30, Damage: 500},
			{SkillID: 9007, Name: "火焰吐息", CD: 10, Range: 20, Damage: 300},
		},
	},
}

// GetBossTemplate 获取指定层的Boss模板，非Boss层返回nil
func GetBossTemplate(layer int32) *Boss {
	if t, ok := BossTemplates[layer]; ok {
		return t
	}
	return nil
}

// SpawnBoss 根据模板生成一个Boss实例（深拷贝，避免并发修改模板）
func SpawnBoss(template *Boss) *Boss {
	return &Boss{
		ID:       template.ID,
		Name:     template.Name,
		Hp:       template.Hp,
		MaxHp:    template.MaxHp,
		Layer:    template.Layer,
		X:        template.X,
		Y:        template.Y,
		Cooldown: template.Cooldown,
		Skills:   template.Skills,
	}
}

// ResourceTemplates 按层数分组的资源模板
var ResourceTemplates = map[int32][]*Resource{
	1:  {{ID: 30001, Type: 1, Name: "草药", ItemID: 2001, Count: 2}, {ID: 30002, Type: 0, Name: "铜矿石", ItemID: 2002, Count: 1}},
	5:  {{ID: 30005, Type: 2, Name: "木材", ItemID: 2003, Count: 2}},
	11: {{ID: 30011, Type: 1, Name: "稀有草药", ItemID: 2001, Count: 3}, {ID: 30012, Type: 0, Name: "银矿石", ItemID: 2002, Count: 2}},
	21: {{ID: 30021, Type: 0, Name: "秘银矿石", ItemID: 2004, Count: 1}, {ID: 30022, Type: 2, Name: "火焰木", ItemID: 2003, Count: 3}},
}

// GetMonsterTemplates 获取指定层的怪物模板，没有则按层级生成
func GetMonsterTemplates(layer int32) []*MonsterTemplate {
	if t, ok := MonsterTemplates[layer]; ok {
		return t
	}
	// 按层级自动生成（深渊怪物 - 暗系）
	levelMul := int64(layer)
	return []*MonsterTemplate{
		{
			ID:         uint64(20000 + layer),
			Name:       "深渊怪物",
			Hp:         50 + levelMul*30,
			Atk:        5 + levelMul*3,
			Def:        2 + levelMul*2,
			ExpReward:  10 + levelMul*10,
			GoldReward: 5 + levelMul*5,
			Element:    ElementDark,
			Rarity:     RarityCommon,
			Behavior:   MonsterBehaviorMelee,
			AggroRange: 30,
			AtkRange:   5,
			AtkCD:      3,
			Skills: []MonsterSkill{
				{SkillID: 20000 + int32(layer), Name: "暗影爪", Element: ElementDark, Power: 5 + levelMul*3, Accuracy: 0.9, CD: 3, Range: 5},
			},
		},
	}
}

// GetResourceTemplates 获取指定层的资源模板
func GetResourceTemplates(layer int32) []*Resource {
	if t, ok := ResourceTemplates[layer]; ok {
		return t
	}
	return nil
}

// SpawnMonsters 为指定层生成怪物实例
func SpawnMonsters(layer int32) map[uint64]*Monster {
	templates := GetMonsterTemplates(layer)
	monsters := make(map[uint64]*Monster, len(templates))
	for i, t := range templates {
		layerMul := int64(layer)
		maxHp := t.Hp + layerMul*10
		x := rand.Float64() * 100
		y := rand.Float64() * 100

		// 默认AI参数
		behavior := t.Behavior
		aggroRange := t.AggroRange
		atkRange := t.AtkRange
		atkCD := t.AtkCD
		if aggroRange == 0 {
			aggroRange = 30
		}
		if atkRange == 0 {
			atkRange = 5
		}
		if atkCD == 0 {
			atkCD = 3
		}
		// 精英怪仇恨范围和攻击力加成
		if t.Elite {
			aggroRange *= 1.3
			atkCD *= 0.8
		}

		m := &Monster{
			ID:         t.ID + uint64(i)*100,
			Name:       t.Name,
			MaxHp:      maxHp,
			Hp:         maxHp + rand.Int63n(20), // 随机HP波动不超过MaxHp+20
			Atk:        t.Atk,
			Def:        t.Def,
			ExpReward:  t.ExpReward,
			GoldReward: t.GoldReward,
			X:          x,
			Y:          y,
			Elite:      t.Elite,
			Element:    t.Element,
			Rarity:     t.Rarity,
			Skills:     t.Skills,
			Behavior:   behavior,
			AIState:    MonsterAIIdle,
			SpawnX:     x,
			SpawnY:     y,
			AggroRange: aggroRange,
			AtkRange:   atkRange,
			AtkCD:      atkCD,
		}
		// 确保Hp不超过MaxHp
		if m.Hp > m.MaxHp {
			m.Hp = m.MaxHp
		}
		monsters[m.ID] = m
	}
	return monsters
}

// SpawnResources 为指定层生成资源实例
func SpawnResources(layer int32) map[uint64]*Resource {
	templates := GetResourceTemplates(layer)
	resources := make(map[uint64]*Resource, len(templates))
	for _, t := range templates {
		r := &Resource{
			ID:     t.ID,
			Type:   t.Type,
			Name:   t.Name,
			ItemID: t.ItemID,
			Count:  t.Count,
			X:      rand.Float64() * 100,
			Y:      rand.Float64() * 100,
		}
		resources[r.ID] = r
	}
	return resources
}

// 区域名称常量，按副本层数划分不同区域
const (
	ZoneForest  = "翠绿森林" // 1~10层区域
	ZoneSwamp   = "腐蚀沼泽" // 11~20层区域
	ZoneVolcano = "烈焰火山" // 21~30层区域
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

// DungeonProgressORM 副本进度表持久化模型，对应 dungeon_progress 表。
type DungeonProgressORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID uint64 `gorm:"not null;uniqueIndex;index:idx_player_id" json:"player_id"`
	MaxLayer int32  `gorm:"not null;default:0" json:"max_layer"`
}

// TableName 指定 DungeonProgressORM 对应的数据库表名
func (DungeonProgressORM) TableName() string { return "dungeon_progress" }
