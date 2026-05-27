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

// mongoSkillRepo 是 SkillRepo 接口的 MongoDB 实现。
type mongoSkillRepo struct {
	col *mongo.Collection
}

// NewMongoSkillRepo 创建基于 MongoDB 的 SkillRepo 实例。
func NewMongoSkillRepo(mdb *database.MongoDB) SkillRepo {
	col := mdb.Database.Collection(model.ColSkills)
	// 创建唯一索引 (player_id, skill_id)
	col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "player_id", Value: 1}, {Key: "skill_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &mongoSkillRepo{col: col}
}

func (r *mongoSkillRepo) GetSkillLevel(ctx context.Context, playerID uint64, skillID int32) (int32, error) {
	var doc model.SkillDoc
	err := r.col.FindOne(ctx, bson.M{"player_id": playerID, "skill_id": skillID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, fmt.Errorf("mongo get skill level player=%d skill=%d: %w", playerID, skillID, err)
	}
	return doc.Level, nil
}

func (r *mongoSkillRepo) SetSkillLevel(ctx context.Context, playerID uint64, skillID int32, level int32) error {
	filter := bson.M{"player_id": playerID, "skill_id": skillID}
	update := bson.M{"$set": bson.M{"level": level}}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("mongo set skill level player=%d skill=%d level=%d: %w", playerID, skillID, level, err)
	}
	return nil
}

func (r *mongoSkillRepo) GetAllSkills(ctx context.Context, playerID uint64) (map[int32]int32, error) {
	cursor, err := r.col.Find(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return nil, fmt.Errorf("mongo get all skills player=%d: %w", playerID, err)
	}
	defer cursor.Close(ctx)

	result := make(map[int32]int32)
	for cursor.Next(ctx) {
		var doc model.SkillDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo decode skill: %w", err)
		}
		result[doc.SkillID] = doc.Level
	}
	return result, nil
}

func (r *mongoSkillRepo) DeleteAllSkills(ctx context.Context, playerID uint64) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return fmt.Errorf("mongo delete all skills player=%d: %w", playerID, err)
	}
	return nil
}
