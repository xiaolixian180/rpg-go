package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// mongoEquipRepo 是 EquipRepo 接口的混合实现。
// 装备主记录走 MySQL（PlayerEquipORM），附魔属性走 MongoDB（EnchantAttrs）。
type mongoEquipRepo struct {
	mysqlRepo EquipRepo        // 复用原有 MySQL 实现
	col       *mongo.Collection // MongoDB 附魔属性集合
}

// NewMongoEquipRepo 创建混合模式的 EquipRepo。
// mysqlRepo 处理装备主记录，MongoDB 处理结构化附魔属性。
func NewMongoEquipRepo(mysqlRepo EquipRepo, mdb *database.MongoDB) EquipRepo {
	col := mdb.Database.Collection(model.ColEquipEnchant)
	// 创建 (player_id, slot) 唯一索引
	col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "player_id", Value: 1}, {Key: "slot", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &mongoEquipRepo{mysqlRepo: mysqlRepo, col: col}
}

func (r *mongoEquipRepo) GetEquipBySlot(ctx context.Context, playerID uint64, slot int32) (*model.Equipment, error) {
	// 从 MySQL 获取装备主记录
	equip, err := r.mysqlRepo.GetEquipBySlot(ctx, playerID, slot)
	if err != nil || equip == nil {
		return equip, err
	}

	// 从 MongoDB 获取附魔属性
	var doc model.EquipDoc
	err = r.col.FindOne(ctx, bson.M{"player_id": playerID, "slot": slot}).Decode(&doc)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("mongo get enchant player=%d slot=%d: %w", playerID, slot, err)
	}

	// 将结构化附魔属性转为 string，保持 Equipment.EnchantAttr 兼容
	if len(doc.EnchantAttrs) > 0 {
		equip.EnchantAttr = formatEnchantAttrs(doc.EnchantAttrs)
	}
	return equip, nil
}

func (r *mongoEquipRepo) GetAllEquips(ctx context.Context, playerID uint64) ([]*model.Equipment, error) {
	// 从 MySQL 获取所有装备主记录
	equips, err := r.mysqlRepo.GetAllEquips(ctx, playerID)
	if err != nil {
		return nil, err
	}

	// 从 MongoDB 批量获取附魔属性
	cursor, err := r.col.Find(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return equips, nil // 降级：附魔查询失败不影响主流程
	}
	defer cursor.Close(ctx)

	enchantMap := make(map[int32][]model.EnchantAttr) // slot -> attrs
	for cursor.Next(ctx) {
		var doc model.EquipDoc
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		enchantMap[doc.Slot] = doc.EnchantAttrs
	}

	// 合并附魔属性到装备
	for _, equip := range equips {
		if attrs, ok := enchantMap[equip.Slot]; ok && len(attrs) > 0 {
			equip.EnchantAttr = formatEnchantAttrs(attrs)
		}
	}
	return equips, nil
}

func (r *mongoEquipRepo) SaveEquip(ctx context.Context, equip *model.Equipment) error {
	// MySQL 保存主记录（不存 EnchantAttr string）
	mysqlEquip := *equip
	mysqlEquip.EnchantAttr = ""
	if err := r.mysqlRepo.SaveEquip(ctx, &mysqlEquip); err != nil {
		return err
	}

	// MongoDB 保存结构化附魔属性
	if equip.EnchantAttr != "" {
		attrs := parseEnchantAttr(equip.EnchantAttr)
		if len(attrs) > 0 {
			filter := bson.M{"player_id": equip.PlayerID, "slot": equip.Slot}
			update := bson.M{"$set": bson.M{"enchant_attrs": attrs}}
			opts := options.UpdateOne().SetUpsert(true)
			_, err := r.col.UpdateOne(ctx, filter, update, opts)
			if err != nil {
				return fmt.Errorf("mongo save enchant player=%d slot=%d: %w", equip.PlayerID, equip.Slot, err)
			}
		}
	}
	return nil
}

func (r *mongoEquipRepo) DeleteEquip(ctx context.Context, playerID uint64, slot int32) error {
	// MySQL 删除主记录
	if err := r.mysqlRepo.DeleteEquip(ctx, playerID, slot); err != nil {
		return err
	}
	// MongoDB 删除附魔属性
	r.col.DeleteOne(ctx, bson.M{"player_id": playerID, "slot": slot})
	return nil
}

func (r *mongoEquipRepo) GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error) {
	return r.mysqlRepo.GetEquipTemplates(ctx, ids)
}

func (r *mongoEquipRepo) GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error) {
	return r.mysqlRepo.GetForgeRecipe(ctx, recipeID)
}

// formatEnchantAttrs 将结构化附魔属性格式化为兼容 string。
func formatEnchantAttrs(attrs []model.EnchantAttr) string {
	result := ""
	for i, a := range attrs {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%s+%d", a.Type, a.Value)
	}
	return result
}

// parseEnchantAttr 解析附魔属性 string 为结构化数据。
// 格式: "力量+5,敏捷+3" 或 "力量+5"
func parseEnchantAttr(s string) []model.EnchantAttr {
	if s == "" {
		return nil
	}
	var attrs []model.EnchantAttr
	pairs := splitSkills(s) // 复用逗号分割
	for _, p := range pairs {
		var typ string
		var val int32
		// 按 "+" 分割
		for i := 0; i < len(p); i++ {
			if p[i] == '+' {
				typ = p[:i]
				fmt.Sscanf(p[i+1:], "%d", &val)
				break
			}
		}
		if typ != "" {
			attrs = append(attrs, model.EnchantAttr{Type: typ, Value: val})
		}
	}
	return attrs
}
