package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ==================== MongoDB 文档模型 ====================
// 这些结构体用于 MongoDB 存储，替代原来 MySQL 中的序列化 string 字段，
// 支持结构化查询和聚合。

// EnchantAttr 装备附魔属性（结构化），替代原来的 string 存储。
type EnchantAttr struct {
	Type  string `bson:"type"`  // "力量"/"敏捷"/"智力"/"体质"/"防御"/"暴击率"/"闪避率"
	Value int32  `bson:"value"`
}

// EquipDoc MongoDB 中的装备文档，存储附魔等结构化数据。
// 装备主记录仍走 MySQL（PlayerEquipORM），MongoDB 仅存附魔属性数组。
type EquipDoc struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	PlayerID     uint64        `bson:"player_id"`
	Slot         int32         `bson:"slot"`
	EnchantAttrs []EnchantAttr `bson:"enchant_attrs"`
}

// SkillDoc MongoDB 中的技能文档。
type SkillDoc struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	PlayerID uint64        `bson:"player_id"`
	SkillID  int32         `bson:"skill_id"`
	Level    int32         `bson:"level"`
}

// InventoryDoc MongoDB 中的背包文档。
type InventoryDoc struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	PlayerID uint64        `bson:"player_id"`
	ItemID   int32         `bson:"item_id"`
	Count    int32         `bson:"count"`
}

// PetSkill 宠物技能（结构化），替代原来的序列化 string。
type PetSkill struct {
	SkillID uint64 `bson:"skill_id"`
	Level   int32  `bson:"level"`
}

// PetDoc MongoDB 中的宠物文档。
type PetDoc struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	PlayerID     uint64        `bson:"player_id"`
	PetUID       uint64        `bson:"pet_uid"`
	PetID        int32         `bson:"pet_id"`
	Name         string        `bson:"name"`
	Level        int32         `bson:"level"`
	Quality      int32         `bson:"quality"`
	Exp          int32         `bson:"exp"`
	Skills       []PetSkill    `bson:"skills"`
	Exploring    bool          `bson:"exploring"`
	ExploreStart time.Time     `bson:"explore_start,omitempty"`
	ExploreEnd   time.Time     `bson:"explore_end,omitempty"`
	Type         int32         `bson:"type"`
	Attack       int32         `bson:"attack"`
	Defense      int32         `bson:"defense"`
	HP           int32         `bson:"hp"`
	PlunderRate  float64       `bson:"plunder_rate"`
}

// Collection 名称常量
const (
	ColEquipEnchant = "equip_enchant" // 装备附魔属性集合
	ColSkills       = "player_skills" // 玩家技能集合
	ColInventory    = "player_inventory_mongo" // 背包集合（避免与 MySQL 表名冲突）
	ColPets         = "player_pets"   // 宠物集合
)
