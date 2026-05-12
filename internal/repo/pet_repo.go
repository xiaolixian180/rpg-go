package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 宠物数据访问实现 ====================

// petRepo 是 PetRepo 接口的具体实现，使用 gorm.DB 操作 MySQL。
type petRepo struct {
	db *gorm.DB
}

// NewPetRepo 创建 PetRepo 实例。
func NewPetRepo(db *database.DB) PetRepo {
	return &petRepo{db: db.DB}
}

// GetPetByUID 根据宠物实例 ID 查询宠物。
// 若未找到返回 nil 和 nil error。
func (r *petRepo) GetPetByUID(ctx context.Context, uid uint64) (*model.Pet, error) {
	var p model.PlayerPetORM
	err := r.db.WithContext(ctx).First(&p, uid).Error
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query pet uid=%d: %w", uid, err)
	}
	// 将 ORM 模型转换为运行时模型
	return &model.Pet{
		UID:     p.ID,
		OwnerID: p.PlayerID,
		PetID:   p.PetID,
		Level:   p.Level,
		Quality: p.Quality,
		Skills:  p.Skills,
	}, nil
}

// GetPetsByOwner 查询玩家拥有的所有宠物，按 ID 升序排列。
// 若玩家没有宠物，返回空切片和 nil error。
func (r *petRepo) GetPetsByOwner(ctx context.Context, ownerID uint64) ([]*model.Pet, error) {
	var rows []model.PlayerPetORM
	err := r.db.WithContext(ctx).
		Where("player_id = ?", ownerID).
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list pets owner=%d: %w", ownerID, err)
	}

	pets := make([]*model.Pet, 0, len(rows))
	for _, p := range rows {
		pets = append(pets, &model.Pet{
			UID:     p.ID,
			OwnerID: p.PlayerID,
			PetID:   p.PetID,
			Level:   p.Level,
			Quality: p.Quality,
			Skills:  p.Skills,
		})
	}
	return pets, nil
}

// SavePet 保存宠物数据，使用 GORM Clauses(clause.OnConflict{...}) 实现 INSERT ... ON DUPLICATE KEY UPDATE。
// 若为新插入，回写自增主键到 pet.UID。
func (r *petRepo) SavePet(ctx context.Context, pet *model.Pet) error {
	p := model.PlayerPetORM{
		PlayerID: pet.OwnerID,
		PetID:    pet.PetID,
		Level:    pet.Level,
		Quality:  pet.Quality,
		Skills:   pet.Skills,
	}
	// 若有自增ID，设置到ORM模型
	if pet.UID > 0 {
		p.ID = pet.UID
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"level", "quality", "skills",
			}),
		}).
		Create(&p)
	if result.Error != nil {
		return fmt.Errorf("save pet uid=%d: %w", pet.UID, result.Error)
	}

	// 回写自增主键
	pet.UID = p.ID
	return nil
}

// DeletePet 根据宠物实例 ID 删除宠物记录。
func (r *petRepo) DeletePet(ctx context.Context, uid uint64) error {
	err := r.db.WithContext(ctx).
		Where("id = ?", uid).
		Delete(&model.PlayerPetORM{}).Error
	if err != nil {
		return fmt.Errorf("delete pet uid=%d: %w", uid, err)
	}
	return nil
}
