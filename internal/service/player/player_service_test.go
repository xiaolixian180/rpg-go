package player

import (
	"context"
	"testing"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/pkg/errors"
)

// ==================== Mock 实现 ====================

// mockPlayerRepo 模拟玩家数据访问
type mockPlayerRepo struct {
	saveCalled  bool
	savedPlayer *model.Player
	saveErr     error
}

func (m *mockPlayerRepo) GetPlayerByID(ctx context.Context, playerID uint64) (*model.Player, error) {
	return nil, nil
}
func (m *mockPlayerRepo) GetMaxLayer(ctx context.Context, playerID uint64) (int32, error) {
	return 0, nil
}
func (m *mockPlayerRepo) SavePlayer(ctx context.Context, p *model.Player) error {
	m.saveCalled = true
	m.savedPlayer = p
	return m.saveErr
}
func (m *mockPlayerRepo) SaveMaxLayer(ctx context.Context, playerID uint64, maxLayer int32) error {
	return nil
}
func (m *mockPlayerRepo) CreatePlayer(ctx context.Context, id uint64, name string, class int32) (uint64, error) {
	return id, nil
}
func (m *mockPlayerRepo) AddGold(ctx context.Context, playerID uint64, delta int64) error {
	return nil
}

// mockCacheRepo 模拟缓存访问
type mockCacheRepo struct{}

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
	return nil, nil
}

// newTestService 创建测试用的 playerService
func newTestService() (*playerService, *mockPlayerRepo) {
	playerRepo := &mockPlayerRepo{}
	cacheRepo := &mockCacheRepo{}
	return &playerService{
		playerRepo: playerRepo,
		cacheRepo:  cacheRepo,
	}, playerRepo
}

// ==================== AssignAttr 测试 ====================

// TestAssignAttr_AppliesPointsCorrectly 测试正确分配属性点
func TestAssignAttr_AppliesPointsCorrectly(t *testing.T) {
	svc, mockRepo := newTestService()

	tests := []struct {
		name       string
		attr       string
		points     int32
		attrPoints int32
		checkField func(p *model.Player) int32
	}{
		{
			name: "分配力量", attr: "str", points: 3, attrPoints: 5,
			checkField: func(p *model.Player) int32 { return p.Str },
		},
		{
			name: "分配敏捷", attr: "agi", points: 2, attrPoints: 5,
			checkField: func(p *model.Player) int32 { return p.Agi },
		},
		{
			name: "分配智力", attr: "int", points: 4, attrPoints: 4,
			checkField: func(p *model.Player) int32 { return p.Int },
		},
		{
			name: "分配体质", attr: "con", points: 1, attrPoints: 1,
			checkField: func(p *model.Player) int32 { return p.Con },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.saveCalled = false
			player := &model.Player{
				ID:         1,
				Name:       "测试玩家",
				Level:      1,
				AttrPoints: tt.attrPoints,
				Str:        0,
				Agi:        0,
				Int:        0,
				Con:        0,
				Def:        0,
			}

			gameErr := svc.AssignAttr(context.Background(), player, tt.attr, tt.points)
			if gameErr != nil {
				t.Fatalf("分配属性失败: %v", gameErr)
			}

			// 验证属性增加
			if got := tt.checkField(player); got != tt.points {
				t.Errorf("属性 %s = %d, 期望 %d", tt.attr, got, tt.points)
			}

			// 验证剩余属性点
			expected := tt.attrPoints - tt.points
			if player.AttrPoints != expected {
				t.Errorf("剩余属性点 = %d, 期望 %d", player.AttrPoints, expected)
			}

			// 验证保存被调用
			if !mockRepo.saveCalled {
				t.Error("分配属性后应调用 SavePlayer")
			}
		})
	}
}

// TestAssignAttr_RejectsInvalidAttrs 测试拒绝无效属性名
func TestAssignAttr_RejectsInvalidAttrs(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:         1,
		AttrPoints: 10,
	}

	invalidAttrs := []string{"strength", "dexterity", "hp", "mp", "", "STR", "AGI"}
	for _, attr := range invalidAttrs {
		t.Run(attr, func(t *testing.T) {
			gameErr := svc.AssignAttr(context.Background(), player, attr, 1)
			if gameErr == nil {
				t.Errorf("属性名 %q 应被拒绝", attr)
			}
			if gameErr.Code != errors.ErrParamInvalid.Code {
				t.Errorf("属性 %q 错误码 = %d, 期望 %d (参数无效)", attr, gameErr.Code, errors.ErrParamInvalid.Code)
			}
		})
	}
}

// TestAssignAttr_RejectsInsufficientPoints 测试拒绝属性点不足
func TestAssignAttr_RejectsInsufficientPoints(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:         1,
		AttrPoints: 2,
		Str:        5,
	}

	gameErr := svc.AssignAttr(context.Background(), player, "str", 5)
	if gameErr == nil {
		t.Fatal("属性点不足应被拒绝")
	}
	if gameErr.Code != errors.ErrAttrPointsNotEnough.Code {
		t.Errorf("错误码 = %d, 期望 %d (属性点不足)", gameErr.Code, errors.ErrAttrPointsNotEnough.Code)
	}

	// 验证属性未被修改
	if player.Str != 5 {
		t.Errorf("属性点不足时 Str 不应被修改, 实际 = %d", player.Str)
	}
	if player.AttrPoints != 2 {
		t.Errorf("属性点不足时 AttrPoints 不应被扣减, 实际 = %d", player.AttrPoints)
	}
}

// TestAssignAttr_RejectsZeroOrNegative 测试拒绝零或负值
func TestAssignAttr_RejectsZeroOrNegative(t *testing.T) {
	svc, _ := newTestService()

	tests := []struct {
		name   string
		points int32
	}{
		{"零值", 0},
		{"负值", -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := &model.Player{
				ID:         1,
				AttrPoints: 10,
				Str:        0,
			}
			gameErr := svc.AssignAttr(context.Background(), player, "str", tt.points)
			if gameErr == nil {
				t.Errorf("分配 %d 点应被拒绝", tt.points)
			}
			if player.Str != 0 {
				t.Errorf("属性不应被修改, 实际 = %d", player.Str)
			}
		})
	}
}

// TestAssignAttr_NilPlayer 测试空玩家返回错误
func TestAssignAttr_NilPlayer(t *testing.T) {
	svc, _ := newTestService()

	gameErr := svc.AssignAttr(context.Background(), nil, "str", 1)
	if gameErr == nil {
		t.Fatal("空玩家应返回错误")
	}
	if gameErr.Code != errors.ErrNotLogin.Code {
		t.Errorf("错误码 = %d, 期望 %d (未登录)", gameErr.Code, errors.ErrNotLogin.Code)
	}
}

// TestAssignAttr_RecalculatesMaxHp 测试分配体质后重算 MaxHp
func TestAssignAttr_RecalculatesMaxHp(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:         1,
		Level:      1,
		AttrPoints: 5,
		Con:        0,
		Def:        0,
		Str:        0,
	}
	player.MaxHp = player.CalcMaxHp() // 100 + 1*20 = 120

	gameErr := svc.AssignAttr(context.Background(), player, "con", 3)
	if gameErr != nil {
		t.Fatalf("分配体质失败: %v", gameErr)
	}

	// MaxHp 应该增加：Con +3 => MaxHp +30
	expectedMaxHp := int64(100+1*20) + int64(3)*10 // 120 + 30 = 150
	if player.MaxHp != expectedMaxHp {
		t.Errorf("MaxHp = %d, 期望 %d", player.MaxHp, expectedMaxHp)
	}
}

// ==================== AddExp 测试 ====================

// TestAddExp_LevelUpWhenEnoughExp 测试经验足够时自动升级
func TestAddExp_LevelUpWhenEnoughExp(t *testing.T) {
	svc, mockRepo := newTestService()

	// 等级1升到2需要100经验
	player := &model.Player{
		ID:    1,
		Level: 1,
		Exp:   0,
		Str:   0,
		Con:   0,
		Def:   0,
	}

	result, gameErr := svc.AddExp(context.Background(), player, 150)
	if gameErr != nil {
		t.Fatalf("增加经验失败: %v", gameErr)
	}

	// 升级后属性检查
	if !result.LevelUped {
		t.Error("应触发升级")
	}
	if result.OldLevel != 1 {
		t.Errorf("旧等级 = %d, 期望 1", result.OldLevel)
	}
	if result.NewLevel < 2 {
		t.Errorf("新等级 = %d, 期望 >= 2", result.NewLevel)
	}
	if player.Level < 2 {
		t.Errorf("玩家等级 = %d, 期望 >= 2", player.Level)
	}
	if result.ExpGained != 150 {
		t.Errorf("获得经验 = %d, 期望 150", result.ExpGained)
	}

	// 每次升级获得3属性点
	expectedAttrPoints := (player.Level - 1) * 3
	if player.AttrPoints != expectedAttrPoints {
		t.Errorf("属性点 = %d, 期望 %d", player.AttrPoints, expectedAttrPoints)
	}

	// 剩余经验：150 - 100(升到2级所需) = 50
	if player.Level == 2 {
		if player.Exp != 50 {
			t.Errorf("剩余经验 = %d, 期望 50", player.Exp)
		}
	}

	if !mockRepo.saveCalled {
		t.Error("升级后应调用 SavePlayer")
	}
}

// TestAddExp_NoLevelUpWhenInsufficientExp 测试经验不足时不升级
func TestAddExp_NoLevelUpWhenInsufficientExp(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:    1,
		Level: 2,
		Exp:   0,
	}

	// 等级2升到3需要250经验，只给50
	result, gameErr := svc.AddExp(context.Background(), player, 50)
	if gameErr != nil {
		t.Fatalf("增加经验失败: %v", gameErr)
	}

	if result.LevelUped {
		t.Error("经验不足不应升级")
	}
	if player.Level != 2 {
		t.Errorf("玩家等级 = %d, 期望 2", player.Level)
	}
	if player.Exp != 50 {
		t.Errorf("玩家经验 = %d, 期望 50", player.Exp)
	}
}

// TestAddExp_ContinuousLevelUp 测试连续升级
func TestAddExp_ContinuousLevelUp(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:    1,
		Level: 1,
		Exp:   0,
	}

	// 给大量经验，应触发连续升级
	// 等级1->2需要100, 等级2->3需要250, 总共需要350
	result, gameErr := svc.AddExp(context.Background(), player, 1000)
	if gameErr != nil {
		t.Fatalf("增加经验失败: %v", gameErr)
	}

	if !result.LevelUped {
		t.Error("应触发升级")
	}
	if result.OldLevel != 1 {
		t.Errorf("旧等级 = %d, 期望 1", result.OldLevel)
	}

	// 检查是否连续升级
	if player.Level <= 2 {
		t.Errorf("应连续升级多级, 实际等级 = %d", player.Level)
	}

	// 验证属性点正确累加：每次升级+3
	expectedAttrPoints := (player.Level - 1) * 3
	if player.AttrPoints != expectedAttrPoints {
		t.Errorf("属性点 = %d, 期望 %d", player.AttrPoints, expectedAttrPoints)
	}
}

// TestAddExp_RejectsZeroOrNegative 测试拒绝零或负经验
func TestAddExp_RejectsZeroOrNegative(t *testing.T) {
	svc, _ := newTestService()

	player := &model.Player{
		ID:    1,
		Level: 1,
		Exp:   0,
	}

	tests := []struct {
		name string
		exp  int64
	}{
		{"零经验", 0},
		{"负经验", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gameErr := svc.AddExp(context.Background(), player, tt.exp)
			if gameErr == nil {
				t.Errorf("经验值 %d 应被拒绝", tt.exp)
			}
			if gameErr.Code != errors.ErrParamInvalid.Code {
				t.Errorf("错误码 = %d, 期望 %d", gameErr.Code, errors.ErrParamInvalid.Code)
			}
		})
	}
}

// TestAddExp_NilPlayer 测试空玩家返回错误
func TestAddExp_NilPlayer(t *testing.T) {
	svc, _ := newTestService()

	_, gameErr := svc.AddExp(context.Background(), nil, 100)
	if gameErr == nil {
		t.Fatal("空玩家应返回错误")
	}
	if gameErr.Code != errors.ErrNotLogin.Code {
		t.Errorf("错误码 = %d, 期望 %d", gameErr.Code, errors.ErrNotLogin.Code)
	}
}
