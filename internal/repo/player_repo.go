package repo

import (
	"context"
	"fmt"

	"hero-quest/internal/database"
	"hero-quest/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 玩家数据访问实现 ====================

type playerRepo struct {
	db *gorm.DB
}

func NewPlayerRepo(db *database.DB) PlayerRepo {
	return &playerRepo{db: db.DB}
}

func (r *playerRepo) GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error) {
	var p model.PlayerORM
	err := r.db.WithContext(ctx).First(&p, playerID).Error
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query player by id %d: %w", playerID, err)
	}
	return &model.Player{
		ID:         p.ID,
		Name:       p.Name,
		Class:      p.Class,
		Level:      p.Level,
		Exp:        p.Exp,
		Gold:       p.Gold,
		Honor:      p.Honor,
		KillValue:  p.KillValue,
		Str:        p.Str,
		Agi:        p.Agi,
		Int:        p.Int,
		Con:        p.Con,
		Def:        p.Def,
		AttrPoints: p.AttrPoints,
	}, nil
}

func (r *playerRepo) GetMaxLayer(ctx context.Context, playerID uint64) (int32, error) {
	var maxLayer int32
	err := r.db.WithContext(ctx).
		Model(&model.DungeonProgressORM{}).
		Select("COALESCE(max_layer, 0)").
		Where("player_id = ?", playerID).
		Scan(&maxLayer).Error
	if err != nil {
		return 0, fmt.Errorf("query max_layer player=%d: %w", playerID, err)
	}
	return maxLayer, nil
}

func (r *playerRepo) SavePlayer(ctx context.Context, p *model.Player) error {
	err := r.db.WithContext(ctx).
		Model(&model.PlayerORM{}).
		Where("id = ?", p.ID).
		Select("name", "class", "level", "exp", "gold", "honor", "kill_value",
			"str", "agi", "int_attr", "con", "def", "attr_points").
		Updates(map[string]interface{}{
			"name":        p.Name,
			"class":       p.Class,
			"level":       p.Level,
			"exp":         p.Exp,
			"gold":        p.Gold,
			"honor":       p.Honor,
			"kill_value":  p.KillValue,
			"str":         p.Str,
			"agi":         p.Agi,
			"int_attr":    p.Int,
			"con":         p.Con,
			"def":         p.Def,
			"attr_points": p.AttrPoints,
		}).Error
	if err != nil {
		return fmt.Errorf("save player %d: %w", p.ID, err)
	}
	return nil
}

func (r *playerRepo) SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error {
	dp := model.DungeonProgressORM{
		PlayerID: playerID,
		MaxLayer: maxLayer,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "player_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"max_layer"}),
		}).
		Create(&dp).Error
	if err != nil {
		return fmt.Errorf("save max_layer player=%d layer=%d: %w", playerID, maxLayer, err)
	}
	return nil
}

// AddGold 原子地给玩家加金币（gold = gold + delta）。
// 用于离线卖家交易结算等场景，避免读取-修改-写入的并发竞争。
func (r *playerRepo) AddGold(ctx context.Context, playerID uint64, delta int64) error {
	err := r.db.WithContext(ctx).
		Model(&model.PlayerORM{}).
		Where("id = ?", playerID).
		UpdateColumn("gold", gorm.Expr("gold + ?", delta)).Error
	if err != nil {
		return fmt.Errorf("add gold player=%d delta=%d: %w", playerID, delta, err)
	}
	return nil
}

func (r *playerRepo) CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error) {
	p := model.PlayerORM{
		ID:         id,
		Name:       name,
		Class:      class,
		Level:      1,
		AttrPoints: 5,
	}
	err := r.db.WithContext(ctx).Create(&p).Error
	if err != nil {
		return 0, fmt.Errorf("create player name=%s class=%d: %w", name, class, err)
	}
	return p.ID, nil
}
