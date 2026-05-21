package model

// DropItem 掉落物品，怪物/Boss被击杀后掉落的物品
type DropItem struct {
	ItemID  uint64 // 物品模板ID
	Name    string // 物品名称
	Quality int32  // 品质等级：0=白 1=绿 2=蓝 3=紫 4=橙（对应 QualityXxx 常量）
	Count   int32  // 物品数量
}

// EnchantAttrTypes 附魔属性类型
var EnchantAttrTypes = []string{
	"力量+%d",
	"敏捷+%d",
	"智力+%d",
	"体质+%d",
	"防御+%d",
	"攻击+%d",
	"生命+%d",
}

// EnchantAttrValueRange 附魔属性值范围（按品质）
var EnchantAttrValueRange = map[int32][2]int32{
	QualityWhite:  {1, 3},
	QualityGreen:  {2, 5},
	QualityBlue:   {3, 8},
	QualityPurple: {5, 12},
	QualityOrange: {8, 20},
}
