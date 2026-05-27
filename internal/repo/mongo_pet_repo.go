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

// mongoPetRepo 是 PetRepo 接口的 MongoDB 实现。
type mongoPetRepo struct {
	col *mongo.Collection
}

// NewMongoPetRepo 创建基于 MongoDB 的 PetRepo 实例。
func NewMongoPetRepo(mdb *database.MongoDB) PetRepo {
	col := mdb.Database.Collection(model.ColPets)
	// 创建 pet_uid 唯一索引
	col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "pet_uid", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	// 创建 player_id 普通索引，加速按玩家查询
	col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "player_id", Value: 1}},
	})
	return &mongoPetRepo{col: col}
}

func (r *mongoPetRepo) GetPetByUID(ctx context.Context, uid uint64) (*model.Pet, error) {
	var doc model.PetDoc
	err := r.col.FindOne(ctx, bson.M{"pet_uid": uid}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("mongo get pet uid=%d: %w", uid, err)
	}
	return petDocToModel(&doc), nil
}

func (r *mongoPetRepo) GetPetsByOwner(ctx context.Context, ownerID uint64) ([]*model.Pet, error) {
	cursor, err := r.col.Find(ctx, bson.M{"player_id": ownerID}, options.Find().SetSort(bson.D{{Key: "pet_uid", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("mongo list pets owner=%d: %w", ownerID, err)
	}
	defer cursor.Close(ctx)

	var pets []*model.Pet
	for cursor.Next(ctx) {
		var doc model.PetDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo decode pet: %w", err)
		}
		pets = append(pets, petDocToModel(&doc))
	}
	return pets, nil
}

func (r *mongoPetRepo) SavePet(ctx context.Context, pet *model.Pet) error {
	filter := bson.M{"pet_uid": pet.UID}
	if pet.UID == 0 {
		// 新宠物，先获取下一个 UID
		pet.UID = r.nextPetUID(ctx)
		filter = bson.M{"pet_uid": pet.UID}
	}

	doc := petModelToDoc(pet)
	update := bson.M{"$set": doc}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("mongo save pet uid=%d: %w", pet.UID, err)
	}
	return nil
}

func (r *mongoPetRepo) DeletePet(ctx context.Context, uid uint64) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"pet_uid": uid})
	if err != nil {
		return fmt.Errorf("mongo delete pet uid=%d: %w", uid, err)
	}
	return nil
}

// nextPetUID 生成下一个宠物 UID（简单自增方案）。
func (r *mongoPetRepo) nextPetUID(ctx context.Context) uint64 {
	// 使用 findAndModify 模式生成自增 ID
	col := r.col.Database().Collection("counters")
	filter := bson.M{"_id": "pet_uid"}
	update := bson.M{"$inc": bson.M{"seq": int64(1)}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var result struct {
		Seq uint64 `bson:"seq"`
	}
	err := col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		// fallback: 使用时间戳
		return uint64(1000000)
	}
	return result.Seq
}

// petDocToModel 将 MongoDB 文档转换为运行时 Pet 模型。
func petDocToModel(doc *model.PetDoc) *model.Pet {
	skills := ""
	if len(doc.Skills) > 0 {
		// 将结构化技能序列化为原来的 string 格式，保持兼容
		for i, s := range doc.Skills {
			if i > 0 {
				skills += ","
			}
			skills += fmt.Sprintf("%d:%d", s.SkillID, s.Level)
		}
	}
	return &model.Pet{
		UID:              doc.PetUID,
		OwnerID:          doc.PlayerID,
		PetID:            doc.PetID,
		Name:             doc.Name,
		Level:            doc.Level,
		Quality:          doc.Quality,
		Skills:           skills,
		Exploring:        doc.Exploring,
		ExploreStartTime: doc.ExploreStart,
		ExploreEndTime:   doc.ExploreEnd,
	}
}

// petModelToDoc 将运行时 Pet 模型转换为 MongoDB 文档。
func petModelToDoc(pet *model.Pet) *model.PetDoc {
	var skills []model.PetSkill
	if pet.Skills != "" {
		// 解析 "skillID:level,skillID:level" 格式
		pairs := splitSkills(pet.Skills)
		skills = make([]model.PetSkill, 0, len(pairs))
		for _, p := range pairs {
			var sid uint64
			var lvl int32
			fmt.Sscanf(p, "%d:%d", &sid, &lvl)
			skills = append(skills, model.PetSkill{SkillID: sid, Level: lvl})
		}
	}
	return &model.PetDoc{
		PlayerID:     pet.OwnerID,
		PetUID:       pet.UID,
		PetID:        pet.PetID,
		Name:         pet.Name,
		Level:        pet.Level,
		Quality:      pet.Quality,
		Skills:       skills,
		Exploring:    pet.Exploring,
		ExploreStart: pet.ExploreStartTime,
		ExploreEnd:   pet.ExploreEndTime,
	}
}

// splitSkills 按逗号分割技能字符串。
func splitSkills(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
