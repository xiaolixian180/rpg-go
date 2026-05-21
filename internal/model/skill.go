package model

// SkillDef 技能定义，描述一个技能的基本属性
type SkillDef struct {
	ID         int32   // 技能ID
	Name       string  // 技能名称
	Class      int32   // 所属职业（对应ClassXxx常量）
	Type       int32   // 技能类型：0=主动 1=被动 2=终极
	MaxLevel   int32   // 技能最大可升级等级
	Multiplier float64 // 技能伤害/效果倍率（相对普攻）
	CD         float64 // 冷却时间（秒）
}

// SkillDefs 全局技能定义表，key为技能ID
// 初始化后只读，并发安全
var SkillDefs = map[int32]SkillDef{
	// 战士
	1: {ID: 1, Name: "旋风斩", Class: ClassWarrior, Type: 0, MaxLevel: 10, Multiplier: 1.8, CD: 5},
	2: {ID: 2, Name: "嘲讽盾", Class: ClassWarrior, Type: 0, MaxLevel: 10, Multiplier: 0.5, CD: 8},
	3: {ID: 3, Name: "冲锋", Class: ClassWarrior, Type: 0, MaxLevel: 10, Multiplier: 1.4, CD: 6},
	4: {ID: 4, Name: "钢铁意志", Class: ClassWarrior, Type: 1, MaxLevel: 10, Multiplier: 0, CD: 0},
	5: {ID: 5, Name: "战神降临", Class: ClassWarrior, Type: 2, MaxLevel: 10, Multiplier: 3.0, CD: 30},
	// 法师
	6:  {ID: 6, Name: "陨石术", Class: ClassMage, Type: 0, MaxLevel: 10, Multiplier: 2.2, CD: 6},
	7:  {ID: 7, Name: "暴风雪", Class: ClassMage, Type: 0, MaxLevel: 10, Multiplier: 1.6, CD: 8},
	8:  {ID: 8, Name: "闪现", Class: ClassMage, Type: 0, MaxLevel: 10, Multiplier: 0, CD: 10},
	9:  {ID: 9, Name: "元素亲和", Class: ClassMage, Type: 1, MaxLevel: 10, Multiplier: 0, CD: 0},
	10: {ID: 10, Name: "元素风暴", Class: ClassMage, Type: 2, MaxLevel: 10, Multiplier: 3.5, CD: 30},
	// 射手
	11: {ID: 11, Name: "穿云箭", Class: ClassArcher, Type: 0, MaxLevel: 10, Multiplier: 2.0, CD: 4},
	12: {ID: 12, Name: "冰冻陷阱", Class: ClassArcher, Type: 0, MaxLevel: 10, Multiplier: 0.8, CD: 12},
	13: {ID: 13, Name: "翻滚", Class: ClassArcher, Type: 0, MaxLevel: 10, Multiplier: 0, CD: 6},
	14: {ID: 14, Name: "鹰眼", Class: ClassArcher, Type: 1, MaxLevel: 10, Multiplier: 0, CD: 0},
	15: {ID: 15, Name: "万箭齐发", Class: ClassArcher, Type: 2, MaxLevel: 10, Multiplier: 2.8, CD: 25},
	// 牧师
	16: {ID: 16, Name: "圣光术", Class: ClassPriest, Type: 0, MaxLevel: 10, Multiplier: 1.5, CD: 3},
	17: {ID: 17, Name: "暗影鞭笞", Class: ClassPriest, Type: 0, MaxLevel: 10, Multiplier: 1.8, CD: 5},
	18: {ID: 18, Name: "治愈光环", Class: ClassPriest, Type: 0, MaxLevel: 10, Multiplier: 0, CD: 10},
	19: {ID: 19, Name: "信仰之力", Class: ClassPriest, Type: 1, MaxLevel: 10, Multiplier: 0, CD: 0},
	20: {ID: 20, Name: "天使降临", Class: ClassPriest, Type: 2, MaxLevel: 10, Multiplier: 3.2, CD: 30},
	// 刺客
	21: {ID: 21, Name: "背刺", Class: ClassAssassin, Type: 0, MaxLevel: 10, Multiplier: 2.5, CD: 4},
	22: {ID: 22, Name: "影分身", Class: ClassAssassin, Type: 0, MaxLevel: 10, Multiplier: 1.2, CD: 15},
	23: {ID: 23, Name: "隐身", Class: ClassAssassin, Type: 0, MaxLevel: 10, Multiplier: 0, CD: 12},
	24: {ID: 24, Name: "致命一击", Class: ClassAssassin, Type: 1, MaxLevel: 10, Multiplier: 0, CD: 0},
	25: {ID: 25, Name: "暗影绝杀", Class: ClassAssassin, Type: 2, MaxLevel: 10, Multiplier: 4.0, CD: 35},
}

// PlayerSkillORM 玩家技能表持久化模型，对应 player_skill 表。
type PlayerSkillORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID uint64 `gorm:"not null;uniqueIndex:uk_player_skill" json:"player_id"`
	SkillID  int32  `gorm:"not null;uniqueIndex:uk_player_skill" json:"skill_id"`
	Level    int32  `gorm:"not null;default:0" json:"level"`
}

// TableName 指定 PlayerSkillORM 对应的数据库表名
func (PlayerSkillORM) TableName() string { return "player_skill" }
