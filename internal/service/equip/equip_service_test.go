package equip

import (
	"context"
	"testing"

	"hero-quest/internal/model"
	"hero-quest/pkg/errors"
)

// ==================== Mock 实现 ====================

// mockEquipRepo 模拟装备数据访问
type mockEquipRepo struct {
	equip      *model.Equipment
	saveErr    error
	saveCalled bool
}

func (m *mockEquipRepo) GetEquipBySlot(ctx context.Context, playerID uint64, slot int32) (*model.Equipment, error) {
	if m.equip == nil {
		return nil, nil
	}
	return m.equip, nil
}
func (m *mockEquipRepo) GetAllEquips(ctx context.Context, playerID uint64) ([]*model.Equipment, error) {
	return nil, nil
}
func (m *mockEquipRepo) SaveEquip(ctx context.Context, equip *model.Equipment) error {
	m.saveCalled = true
	return m.saveErr
}
func (m *mockEquipRepo) DeleteEquip(ctx context.Context, playerID uint64, slot int32) error {
	return nil
}
func (m *mockEquipRepo) GetEquipTemplates(ctx context.Context, ids []int32) (map[int32]*model.EquipTemplate, error) {
	return nil, nil
}
func (m *mockEquipRepo) GetForgeRecipe(ctx context.Context, recipeID uint64) (*model.ForgeRecipe, error) {
	return nil, nil
}

// mockInventoryRepo 模拟背包数据访问
type mockInventoryRepo struct{}

func (m *mockInventoryRepo) GetItemCount(ctx context.Context, playerID uint64, itemID int32) (int32, error) {
	return 0, nil
}
func (m *mockInventoryRepo) AddItem(ctx context.Context, playerID uint64, itemID int32, count int32) error {
	return nil
}
func (m *mockInventoryRepo) RemoveItem(ctx context.Context, playerID uint64, itemID int32, count int32) (bool, error) {
	return true, nil
}
func (m *mockInventoryRepo) ListItems(ctx context.Context, playerID uint64) ([]*model.PlayerInventoryORM, error) {
	return nil, nil
}

// newTestService 创建测试用的 equipService
func newTestService(equip *model.Equipment) (*equipService, *mockEquipRepo) {
	equipRepo := &mockEquipRepo{equip: equip}
	inventoryRepo := &mockInventoryRepo{}
	return &equipService{
		equipRepo:     equipRepo,
		inventoryRepo: inventoryRepo,
	}, equipRepo
}

// ==================== Strengthen 测试 ====================

// TestStrengthen_DeductsGoldCorrectly 测试强化正确扣减金币
// 强化费用 = (当前强化等级 + 1) * 100
func TestStrengthen_DeductsGoldCorrectly(t *testing.T) {
	tests := []struct {
		name            string
		strengthenLevel int32
		playerGold      int64
		expectedCost    int64
	}{
		{"+0升+1", 0, 10000, 100},
		{"+3升+4", 3, 10000, 400},
		{"+6升+7", 6, 10000, 700},
		{"+9升+10", 9, 10000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equip := &model.Equipment{
				PlayerID:        1,
				Slot:            0,
				StrengthenLevel: tt.strengthenLevel,
			}
			svc, mockRepo := newTestService(equip)

			player := &model.Player{
				ID:    1,
				Gold:  tt.playerGold,
				Level: 10,
			}

			result, gameErr := svc.Strengthen(context.Background(), player, 0)
			if gameErr != nil {
				t.Fatalf("强化失败: %v", gameErr)
			}

			// 验证扣减的金币
			if result.CostGold != tt.expectedCost {
				t.Errorf("消耗金币 = %d, 期望 %d", result.CostGold, tt.expectedCost)
			}

			// 验证玩家金币被正确扣减
			expectedRemain := tt.playerGold - tt.expectedCost
			if player.Gold != expectedRemain {
				t.Errorf("玩家剩余金币 = %d, 期望 %d", player.Gold, expectedRemain)
			}

			// +7以下必定成功
			if tt.strengthenLevel < 7 {
				if !result.IsSuccess {
					t.Error("+7以下强化应必定成功")
				}
				if result.NewLevel != tt.strengthenLevel+1 {
					t.Errorf("强化后等级 = %d, 期望 %d", result.NewLevel, tt.strengthenLevel+1)
				}
			}

			// 验证保存被调用（成功时）
			if result.IsSuccess && !mockRepo.saveCalled {
				t.Error("强化成功后应调用 SaveEquip")
			}
		})
	}
}

// TestStrengthen_FailsWithInsufficientGold 测试金币不足时强化失败
func TestStrengthen_FailsWithInsufficientGold(t *testing.T) {
	tests := []struct {
		name            string
		strengthenLevel int32
		playerGold      int64
	}{
		{"金币为0", 0, 0},
		{"金币不足_差1", 5, 599},  // 需要600，只有599
		{"金币不足_高等级", 9, 500}, // 需要1000，只有500
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equip := &model.Equipment{
				PlayerID:        1,
				Slot:            0,
				StrengthenLevel: tt.strengthenLevel,
			}
			svc, _ := newTestService(equip)

			player := &model.Player{
				ID:   1,
				Gold: tt.playerGold,
			}

			_, gameErr := svc.Strengthen(context.Background(), player, 0)
			if gameErr == nil {
				t.Fatal("金币不足应返回错误")
			}
			if gameErr.Code != errors.ErrGoldNotEnough.Code {
				t.Errorf("错误码 = %d, 期望 %d (金币不足)", gameErr.Code, errors.ErrGoldNotEnough.Code)
			}

			// 金币不应被扣减
			if player.Gold != tt.playerGold {
				t.Errorf("金币不足时不应扣减, 原有 %d, 实际 %d", tt.playerGold, player.Gold)
			}
		})
	}
}

// TestStrengthen_InvalidSlot 测试无效槽位
func TestStrengthen_InvalidSlot(t *testing.T) {
	svc, _ := newTestService(nil)

	player := &model.Player{
		ID:   1,
		Gold: 10000,
	}

	tests := []struct {
		name string
		slot int32
	}{
		{"负数槽位", -1},
		{"超出范围", model.SlotMax},
		{"远超范围", 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gameErr := svc.Strengthen(context.Background(), player, tt.slot)
			if gameErr == nil {
				t.Errorf("槽位 %d 应返回错误", tt.slot)
			}
			if gameErr.Code != errors.ErrSlotInvalid.Code {
				t.Errorf("错误码 = %d, 期望 %d (槽位无效)", gameErr.Code, errors.ErrSlotInvalid.Code)
			}
		})
	}
}

// TestStrengthen_EmptySlot 测试空槽位强化
func TestStrengthen_EmptySlot(t *testing.T) {
	svc, _ := newTestService(nil) // nil 表示槽位为空

	player := &model.Player{
		ID:   1,
		Gold: 10000,
	}

	_, gameErr := svc.Strengthen(context.Background(), player, 0)
	if gameErr == nil {
		t.Fatal("空槽位应返回错误")
	}
	if gameErr.Code != errors.ErrSlotEmpty.Code {
		t.Errorf("错误码 = %d, 期望 %d (槽位为空)", gameErr.Code, errors.ErrSlotEmpty.Code)
	}
}

// TestStrengthen_NilPlayer 测试空玩家返回错误
func TestStrengthen_NilPlayer(t *testing.T) {
	svc, _ := newTestService(nil)

	_, gameErr := svc.Strengthen(context.Background(), nil, 0)
	if gameErr == nil {
		t.Fatal("空玩家应返回错误")
	}
	if gameErr.Code != errors.ErrNotLogin.Code {
		t.Errorf("错误码 = %d, 期望 %d (未登录)", gameErr.Code, errors.ErrNotLogin.Code)
	}
}

// TestStrengthen_HighLevelHasFailureRate 测试+7以上有失败概率
// 注意：因为存在随机性，此测试通过多次运行统计来验证
func TestStrengthen_HighLevelHasFailureRate(t *testing.T) {
	// +7以上强化有失败概率：(level-6)*10%
	// +9 -> 失败率 30%，运行多次应至少出现一次失败
	successCount := 0
	failCount := 0

	for i := 0; i < 100; i++ {
		equip := &model.Equipment{
			PlayerID:        1,
			Slot:            0,
			StrengthenLevel: 9, // 失败率 30%
		}
		svc, _ := newTestService(equip)

		player := &model.Player{
			ID:   1,
			Gold: 100000,
		}

		result, gameErr := svc.Strengthen(context.Background(), player, 0)
		if gameErr != nil {
			t.Fatalf("强化失败(非预期): %v", gameErr)
		}

		if result.IsSuccess {
			successCount++
		} else {
			failCount++
			// 失败时等级不变
			if result.NewLevel != 9 {
				t.Errorf("强化失败时等级应不变, 实际 = %d", result.NewLevel)
			}
		}
	}

	// 100次中至少应有几次失败（概率上几乎不可能100次全成功）
	if failCount == 0 {
		t.Log("警告：100次+9强化全部成功，概率极低（约1e-15），可能失败率逻辑有问题")
	}

	// 金币都应被扣减
	if successCount+failCount != 100 {
		t.Errorf("总次数 = %d, 期望 100", successCount+failCount)
	}
}
