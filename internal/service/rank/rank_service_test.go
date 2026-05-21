package rank

import (
	"context"
	"testing"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
)

// ==================== Mock 实现 ====================

// mockCacheRepo 模拟缓存访问
type mockCacheRepo struct {
	rankItems []repo.RankItem
	rankErr   error
}

func (m *mockCacheRepo) SetPlayerCache(ctx context.Context, playerID uint64, data []byte) error {
	return nil
}
func (m *mockCacheRepo) GetPlayerCache(ctx context.Context, playerID uint64) ([]byte, error) {
	return nil, nil
}
func (m *mockCacheRepo) DelPlayerCache(ctx context.Context, playerID uint64) error { return nil }
func (m *mockCacheRepo) SetBossCooldown(ctx context.Context, layer int32, cooldown time.Duration) error {
	return nil
}
func (m *mockCacheRepo) IsBossCooldown(ctx context.Context, layer int32) (bool, error) {
	return false, nil
}
func (m *mockCacheRepo) UpdateRanking(ctx context.Context, key, member string, score float64) error {
	return nil
}
func (m *mockCacheRepo) GetRanking(ctx context.Context, key string, offset, count int64) ([]repo.RankItem, error) {
	return m.rankItems, m.rankErr
}

// mockPlayerRepo 模拟玩家数据访问
type mockPlayerRepo struct {
	players map[uint64]*model.Player
}

func (m *mockPlayerRepo) GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error) {
	if p, ok := m.players[playerID]; ok {
		return p, nil
	}
	return nil, nil
}
func (m *mockPlayerRepo) GetMaxLayer(ctx context.Context, playerID uint64) (int32, error) {
	return 0, nil
}
func (m *mockPlayerRepo) SavePlayer(ctx context.Context, p *model.Player) error { return nil }
func (m *mockPlayerRepo) SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error {
	return nil
}
func (m *mockPlayerRepo) CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error) {
	return id, nil
}

// ==================== GetRanking 测试 ====================

// TestGetRanking_ParsesPlayerIDs 测试从 Redis member 字符串正确解析玩家 ID
func TestGetRanking_ParsesPlayerIDs(t *testing.T) {
	cacheRepo := &mockCacheRepo{
		rankItems: []repo.RankItem{
			{Member: "1001", Score: 50.0},
			{Member: "1002", Score: 45.0},
			{Member: "1003", Score: 30.0},
		},
	}
	playerRepo := &mockPlayerRepo{
		players: map[uint64]*model.Player{
			1001: {ID: 1001, Name: "勇者"},
			1002: {ID: 1002, Name: "法师"},
			1003: {ID: 1003, Name: "刺客"},
		},
	}

	svc := NewRankService(cacheRepo, playerRepo)

	rankings, gameErr := svc.GetRanking(context.Background(), 0) // 0 = 等级排行榜
	if gameErr != nil {
		t.Fatalf("查询排行榜失败: %v", gameErr)
	}

	if len(rankings) != 3 {
		t.Fatalf("排行榜条目数 = %d, 期望 3", len(rankings))
	}

	// 验证排名、ID、名称、分数
	expected := []struct {
		rank     int32
		playerID uint64
		name     string
		value    int64
	}{
		{1, 1001, "勇者", 50},
		{2, 1002, "法师", 45},
		{3, 1003, "刺客", 30},
	}

	for i, exp := range expected {
		r := rankings[i]
		if r.Rank != exp.rank {
			t.Errorf("排名[%d].Rank = %d, 期望 %d", i, r.Rank, exp.rank)
		}
		if r.PlayerID != exp.playerID {
			t.Errorf("排名[%d].PlayerID = %d, 期望 %d", i, r.PlayerID, exp.playerID)
		}
		if r.Name != exp.name {
			t.Errorf("排名[%d].Name = %s, 期望 %s", i, r.Name, exp.name)
		}
		if r.Value != exp.value {
			t.Errorf("排名[%d].Value = %d, 期望 %d", i, r.Value, exp.value)
		}
	}
}

// TestGetRanking_EmptyRanking 测试空排行榜
func TestGetRanking_EmptyRanking(t *testing.T) {
	cacheRepo := &mockCacheRepo{rankItems: []repo.RankItem{}}
	playerRepo := &mockPlayerRepo{players: map[uint64]*model.Player{}}

	svc := NewRankService(cacheRepo, playerRepo)

	rankings, gameErr := svc.GetRanking(context.Background(), 0)
	if gameErr != nil {
		t.Fatalf("查询空排行榜失败: %v", gameErr)
	}

	if len(rankings) != 0 {
		t.Errorf("空排行榜条目数 = %d, 期望 0", len(rankings))
	}
}

// TestGetRanking_InvalidRankType 测试无效排行榜类型
func TestGetRanking_InvalidRankType(t *testing.T) {
	cacheRepo := &mockCacheRepo{}
	playerRepo := &mockPlayerRepo{}

	svc := NewRankService(cacheRepo, playerRepo)

	_, gameErr := svc.GetRanking(context.Background(), 99) // 无效类型
	if gameErr == nil {
		t.Fatal("无效排行榜类型应返回错误")
	}
}

// TestGetRanking_InvalidMemberFallback 测试无法解析的 member 使用原字符串作为名称
func TestGetRanking_InvalidMemberFallback(t *testing.T) {
	cacheRepo := &mockCacheRepo{
		rankItems: []repo.RankItem{
			{Member: "not_a_number", Score: 100.0},
		},
	}
	playerRepo := &mockPlayerRepo{players: map[uint64]*model.Player{}}

	svc := NewRankService(cacheRepo, playerRepo)

	rankings, gameErr := svc.GetRanking(context.Background(), 0)
	if gameErr != nil {
		t.Fatalf("查询排行榜失败: %v", gameErr)
	}

	if len(rankings) != 1 {
		t.Fatalf("排行榜条目数 = %d, 期望 1", len(rankings))
	}

	// 无法解析的 member，PlayerID 应为 0，名称使用原字符串
	if rankings[0].PlayerID != 0 {
		t.Errorf("PlayerID = %d, 期望 0 (解析失败)", rankings[0].PlayerID)
	}
	if rankings[0].Name != "not_a_number" {
		t.Errorf("Name = %s, 期望 not_a_number (默认回退)", rankings[0].Name)
	}
}

// TestGetRanking_PlayerNotFoundUsesMember 测试玩家不存在时使用 member 作为名称
func TestGetRanking_PlayerNotFoundUsesMember(t *testing.T) {
	cacheRepo := &mockCacheRepo{
		rankItems: []repo.RankItem{
			{Member: "9999", Score: 200.0},
		},
	}
	playerRepo := &mockPlayerRepo{players: map[uint64]*model.Player{}} // 空的，找不到玩家

	svc := NewRankService(cacheRepo, playerRepo)

	rankings, gameErr := svc.GetRanking(context.Background(), 1) // 1 = 战力排行榜
	if gameErr != nil {
		t.Fatalf("查询排行榜失败: %v", gameErr)
	}

	if len(rankings) != 1 {
		t.Fatalf("排行榜条目数 = %d, 期望 1", len(rankings))
	}

	// 玩家不存在时，PlayerID 正确解析但名称使用 member 字符串
	if rankings[0].PlayerID != 9999 {
		t.Errorf("PlayerID = %d, 期望 9999", rankings[0].PlayerID)
	}
	if rankings[0].Name != "9999" {
		t.Errorf("Name = %s, 期望 9999 (玩家不存在回退)", rankings[0].Name)
	}
	if rankings[0].Value != 200 {
		t.Errorf("Value = %d, 期望 200", rankings[0].Value)
	}
}

// TestGetRanking_AllRankTypes 测试所有排行榜类型
func TestGetRanking_AllRankTypes(t *testing.T) {
	cacheRepo := &mockCacheRepo{
		rankItems: []repo.RankItem{
			{Member: "1", Score: 100.0},
		},
	}
	playerRepo := &mockPlayerRepo{
		players: map[uint64]*model.Player{
			1: {ID: 1, Name: "玩家1"},
		},
	}

	svc := NewRankService(cacheRepo, playerRepo)

	validTypes := []int32{0, 1, 2} // 等级/战力/荣誉
	for _, rankType := range validTypes {
		t.Run(map[int32]string{0: "等级", 1: "战力", 2: "荣誉"}[rankType], func(t *testing.T) {
			rankings, gameErr := svc.GetRanking(context.Background(), rankType)
			if gameErr != nil {
				t.Fatalf("排行榜类型 %d 查询失败: %v", rankType, gameErr)
			}
			if len(rankings) != 1 {
				t.Errorf("排行榜类型 %d 条目数 = %d, 期望 1", rankType, len(rankings))
			}
		})
	}
}
