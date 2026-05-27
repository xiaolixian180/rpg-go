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

// mongoInventoryRepo 是 InventoryRepo 接口的 MongoDB 实现。
type mongoInventoryRepo struct {
	col *mongo.Collection
}

// NewMongoInventoryRepo 创建基于 MongoDB 的 InventoryRepo 实例。
func NewMongoInventoryRepo(mdb *database.MongoDB) InventoryRepo {
	col := mdb.Database.Collection(model.ColInventory)
	// 创建唯一索引 (player_id, item_id)
	col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "player_id", Value: 1}, {Key: "item_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &mongoInventoryRepo{col: col}
}

func (r *mongoInventoryRepo) GetItemCount(ctx context.Context, playerID uint64, itemID int32) (int32, error) {
	var doc model.InventoryDoc
	err := r.col.FindOne(ctx, bson.M{"player_id": playerID, "item_id": itemID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, fmt.Errorf("mongo get item count player=%d item=%d: %w", playerID, itemID, err)
	}
	return doc.Count, nil
}

func (r *mongoInventoryRepo) AddItem(ctx context.Context, playerID uint64, itemID int32, count int32) error {
	filter := bson.M{"player_id": playerID, "item_id": itemID}
	update := bson.M{"$inc": bson.M{"count": count}}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("mongo add item player=%d item=%d count=%d: %w", playerID, itemID, count, err)
	}
	return nil
}

func (r *mongoInventoryRepo) RemoveItem(ctx context.Context, playerID uint64, itemID int32, count int32) (bool, error) {
	// 先检查数量是否足够
	current, err := r.GetItemCount(ctx, playerID, itemID)
	if err != nil {
		return false, err
	}
	if current < count {
		return false, nil
	}

	// 扣减数量
	filter := bson.M{"player_id": playerID, "item_id": itemID, "count": bson.M{"$gte": count}}
	update := bson.M{"$inc": bson.M{"count": -count}}
	result, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, fmt.Errorf("mongo remove item player=%d item=%d: %w", playerID, itemID, err)
	}
	if result.MatchedCount == 0 {
		return false, nil
	}

	// 清理 count <= 0 的文档
	r.col.DeleteMany(ctx, bson.M{"player_id": playerID, "item_id": itemID, "count": bson.M{"$lte": 0}})
	return true, nil
}

func (r *mongoInventoryRepo) ListItems(ctx context.Context, playerID uint64) ([]*model.PlayerInventoryORM, error) {
	cursor, err := r.col.Find(ctx, bson.M{"player_id": playerID, "count": bson.M{"$gt": 0}})
	if err != nil {
		return nil, fmt.Errorf("mongo list items player=%d: %w", playerID, err)
	}
	defer cursor.Close(ctx)

	var items []*model.PlayerInventoryORM
	for cursor.Next(ctx) {
		var doc model.InventoryDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo decode inventory: %w", err)
		}
		items = append(items, &model.PlayerInventoryORM{
			PlayerID: doc.PlayerID,
			ItemID:   doc.ItemID,
			Count:    doc.Count,
		})
	}
	return items, nil
}
