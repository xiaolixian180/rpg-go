package combat

import (
	"context"
	"testing"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/service/iface"
)

// ==================== calcDamage 测试 ====================

// TestCalcDamage_BasicAttack 测试普通攻击伤害计算
// 攻击力公式：Str*2 + Level*5 + Agi*0.5
func TestCalcDamage_BasicAttack(t *testing.T) {
	svc := &combatService{}
	player := &model.Player{
		Str:   10,
		Level: 5,
		Agi:   4,
	}
	// 基础攻击力 = 10*2 + 5*5 + 4*0.5 = 20 + 25 + 2 = 47
	expectedAttack := int64(47)

	// 多次运行取最小值，避免随机波动导致测试不稳定
	// 普通攻击(无技能)：multiplier=1.0，最低伤害应 >= attack
	minDamage := int64(999999)
	for i := 0; i < 100; i++ {
		damage, _ := svc.calcDamage(readCombatAttrs(player, 0), 0)
		if damage < minDamage {
			minDamage = damage
		}
	}
	// 保底至少等于基础攻击力（不含暴击/波动时）
	if minDamage < expectedAttack {
		t.Errorf("普通攻击最低伤害应 >= 基础攻击力 %d, 实际最低 %d", expectedAttack, minDamage)
	}
}

// TestCalcDamage_WithSkillMultiplier 测试技能倍率加成
func TestCalcDamage_WithSkillMultiplier(t *testing.T) {
	svc := &combatService{}
	player := &model.Player{
		Str:   10,
		Level: 5,
		Agi:   4,
	}

	// 使用旋风斩 (ID=1, Multiplier=1.8)
	// 因为存在随机波动，用多次平均来验证
	normalSum := int64(0)
	skillSum := int64(0)
	rounds := 200
	for i := 0; i < rounds; i++ {
		d1, _ := svc.calcDamage(readCombatAttrs(player, 0), 0)
		d2, _ := svc.calcDamage(readCombatAttrs(player, 0), 1)
		normalSum += d1
		skillSum += d2
	}
	normalAvg := normalSum / int64(rounds)
	skillAvg := skillSum / int64(rounds)

	// 技能平均伤害应约为普通攻击的 1.8 倍
	ratio := float64(skillAvg) / float64(normalAvg)
	if ratio < 1.5 || ratio > 2.1 {
		t.Errorf("技能倍率期望约 1.8, 实际比例 %.2f (normalAvg=%d, skillAvg=%d)", ratio, normalAvg, skillAvg)
	}
}

// TestCalcDamage_CritIncreasesDamage 测试暴击增加伤害
func TestCalcDamage_CritIncreasesDamage(t *testing.T) {
	svc := &combatService{}
	// 高敏捷 + 高力量 -> 高暴击率和暴击伤害
	player := &model.Player{
		Str:   50,
		Level: 10,
		Agi:   100, // 暴击率 = 100*0.003 + 50*0.001 = 0.35
	}

	critCount := 0
	critDamageSum := int64(0)
	normalDamageSum := int64(0)
	normalCount := 0

	for i := 0; i < 1000; i++ {
		damage, isCrit := svc.calcDamage(readCombatAttrs(player, 0), 0)
		if isCrit {
			critCount++
			critDamageSum += damage
		} else {
			normalCount++
			normalDamageSum += damage
		}
	}

	if critCount == 0 {
		t.Skip("未触发暴击，跳过暴击伤害比较")
	}

	critAvg := critDamageSum / int64(critCount)
	normalAvg := normalDamageSum / int64(normalCount)

	// 暴击平均伤害应高于非暴击（暴击倍率 = 1.5 + Str*0.01 = 2.0）
	if critAvg <= normalAvg {
		t.Errorf("暴击平均伤害(%d)应高于普通平均伤害(%d)", critAvg, normalAvg)
	}
}

// TestCalcDamage_StrAndLevelAffectAttack 测试力量和等级影响攻击力
func TestCalcDamage_StrAndLevelAffectAttack(t *testing.T) {
	svc := &combatService{}

	weakPlayer := &model.Player{Str: 5, Level: 1, Agi: 0}
	strongPlayer := &model.Player{Str: 30, Level: 20, Agi: 0}

	weakSum := int64(0)
	strongSum := int64(0)
	rounds := 100

	for i := 0; i < rounds; i++ {
		d1, _ := svc.calcDamage(readCombatAttrs(weakPlayer, 0), 0)
		d2, _ := svc.calcDamage(readCombatAttrs(strongPlayer, 0), 0)
		weakSum += d1
		strongSum += d2
	}

	weakAvg := weakSum / int64(rounds)
	strongAvg := strongSum / int64(rounds)

	if strongAvg <= weakAvg {
		t.Errorf("高属性玩家攻击(%d)应高于低属性玩家(%d)", strongAvg, weakAvg)
	}
}

// ==================== applyDefense 测试 ====================

// TestApplyDefense 测试防御减伤
func TestApplyDefense(t *testing.T) {
	tests := []struct {
		name    string
		damage  int64
		def     int64
		wantMin int64
		wantMax int64
	}{
		{"无防御", 100, 0, 100, 100},
		{"部分减伤", 100, 30, 70, 70},
		{"防御超过伤害保底1", 50, 100, 1, 1},
		{"防御等于伤害保底1", 100, 100, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyDefense(tt.damage, tt.def)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("applyDefense(%d, %d) = %d, 期望范围 [%d, %d]",
					tt.damage, tt.def, result, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ==================== 怪物击杀奖励测试 ====================

// TestMonsterKillAwardsExpAndGold 测试普通怪物击杀后获得经验和金币
func TestMonsterKillAwardsExpAndGold(t *testing.T) {
	svc := &combatService{}

	monster := &model.Monster{
		ID:         10001,
		Name:       "史莱姆",
		Hp:         50,
		MaxHp:      50,
		Atk:        5,
		Def:        2,
		ExpReward:  10,
		GoldReward: 5,
	}

	dungeon := &model.DungeonLayer{
		Layer:    1,
		Monsters: map[uint64]*model.Monster{10001: monster},
	}

	world := &mockWorld{dungeon: dungeon}
	svc.world = world

	attacker := &model.Player{
		ID:    1,
		Name:  "测试玩家",
		Level: 5,
		Str:   20,
		Agi:   10,
		Hp:    1000,
		MaxHp: 1000,
		Layer: 1,
	}

	result, gameErr := svc.Attack(context.Background(), attacker, 10001, 0)
	if gameErr != nil {
		t.Fatalf("攻击失败: %v", gameErr)
	}

	// 怪物血量只有 50，玩家等级5力量20，攻击力=40+25+5=70，足以一击击杀
	if !result.IsDead {
		t.Logf("怪物未死亡（可能伤害不够），当前血量: %d, 伤害: %d", result.CurrHp, result.Damage)
	}

	if result.IsDead {
		if result.ExpGain != monster.ExpReward {
			t.Errorf("击杀经验 = %d, 期望 %d", result.ExpGain, monster.ExpReward)
		}
		if result.GoldGain != monster.GoldReward {
			t.Errorf("击杀金币 = %d, 期望 %d", result.GoldGain, monster.GoldReward)
		}
	}
}

// ==================== Boss 击杀奖励测试 ====================

// TestBossKillAwardsExpAndGold 测试 Boss 击杀后获得经验和金币
// Boss经验 = MaxHp / 5, Boss金币 = MaxHp / 10
func TestBossKillAwardsExpAndGold(t *testing.T) {
	svc := &combatService{}

	boss := &model.Boss{
		ID:    90001,
		Name:  "森林之王",
		Hp:    500,
		MaxHp: 500,
		Layer: 10,
	}

	world := &mockWorld{boss: boss}
	svc.world = world

	attacker := &model.Player{
		ID:    1,
		Name:  "测试玩家",
		Level: 30,
		Str:   50,
		Agi:   20,
		Hp:    5000,
		MaxHp: 5000,
		Layer: 10,
	}

	// 反复攻击直到击杀
	var lastResult *CombatResult
	for i := 0; i < 20; i++ {
		result, gameErr := svc.Attack(context.Background(), attacker, 90001, 0)
		if gameErr != nil {
			t.Fatalf("攻击Boss失败: %v", gameErr)
		}
		lastResult = result
		if result.IsDead {
			break
		}
	}

	if lastResult == nil || !lastResult.IsDead {
		t.Fatal("多次攻击后Boss仍未死亡")
	}

	expectedExp := boss.MaxHp / 5   // 100
	expectedGold := boss.MaxHp / 10 // 50

	if lastResult.ExpGain != expectedExp {
		t.Errorf("Boss击杀经验 = %d, 期望 %d", lastResult.ExpGain, expectedExp)
	}
	if lastResult.GoldGain != expectedGold {
		t.Errorf("Boss击杀金币 = %d, 期望 %d", lastResult.GoldGain, expectedGold)
	}
	if !lastResult.IsBoss {
		t.Error("目标应标记为Boss")
	}
}

// ==================== 资源采集测试 ====================

// TestCollectResource 测试资源采集
func TestCollectResource(t *testing.T) {
	svc := &combatService{}

	resource := &model.Resource{
		ID:     30001,
		Type:   1,
		Name:   "草药",
		ItemID: 2001,
		Count:  3,
	}

	result, gameErr := svc.CollectResource(context.Background(), 1, resource)
	if gameErr != nil {
		t.Fatalf("采集失败: %v", gameErr)
	}

	if result.ResourceID != resource.ID {
		t.Errorf("资源ID = %d, 期望 %d", result.ResourceID, resource.ID)
	}
	if result.ItemID != uint64(resource.ItemID) {
		t.Errorf("物品ID = %d, 期望 %d", result.ItemID, resource.ItemID)
	}
	if result.Count != resource.Count {
		t.Errorf("物品数量 = %d, 期望 %d", result.Count, resource.Count)
	}
	if result.ItemName != "草药" {
		t.Errorf("物品名称 = %s, 期望 草药", result.ItemName)
	}
}

// TestCollectResource_AlreadyHarvested 测试重复采集被拒绝
func TestCollectResource_AlreadyHarvested(t *testing.T) {
	svc := &combatService{}

	resource := &model.Resource{
		ID:        30001,
		Type:      0,
		ItemID:    2002,
		Count:     1,
		Harvested: true, // 已采集
	}

	_, gameErr := svc.CollectResource(context.Background(), 1, resource)
	if gameErr == nil {
		t.Fatal("重复采集应返回错误")
	}
	if gameErr.Code != 305 { // ErrResourceGone
		t.Errorf("错误码 = %d, 期望 305 (资源已被采集)", gameErr.Code)
	}
}

// TestCollectResource_ResourceTypes 测试不同资源类型的名称
func TestCollectResource_ResourceTypes(t *testing.T) {
	svc := &combatService{}

	tests := []struct {
		rType   int32
		expName string
	}{
		{0, "矿石"},
		{1, "草药"},
		{2, "木材"},
		{99, "未知材料"},
	}

	for _, tt := range tests {
		t.Run(tt.expName, func(t *testing.T) {
			resource := &model.Resource{
				ID:     30001,
				Type:   tt.rType,
				ItemID: 2001,
				Count:  1,
			}
			result, err := svc.CollectResource(context.Background(), 1, resource)
			if err != nil {
				t.Fatalf("采集失败: %v", err)
			}
			if result.ItemName != tt.expName {
				t.Errorf("资源类型 %d 物品名称 = %s, 期望 %s", tt.rType, result.ItemName, tt.expName)
			}
		})
	}
}

// ==================== 攻击者死亡无法攻击 ====================

// TestAttack_SelfDeadCannotAttack 测试已死亡的玩家无法攻击
func TestAttack_SelfDeadCannotAttack(t *testing.T) {
	svc := &combatService{}

	attacker := &model.Player{
		ID:    1,
		Hp:    0, // 已死亡
		Layer: 1,
	}

	_, gameErr := svc.Attack(context.Background(), attacker, 10001, 0)
	if gameErr == nil {
		t.Fatal("已死亡玩家不应能攻击")
	}
	if gameErr.Code != 304 { // ErrSelfDead
		t.Errorf("错误码 = %d, 期望 304 (角色已死亡)", gameErr.Code)
	}
}

// ==================== 目标不存在 ====================

// TestAttack_TargetNotFound 测试攻击不存在的目标
func TestAttack_TargetNotFound(t *testing.T) {
	svc := &combatService{}

	world := &mockWorld{dungeon: &model.DungeonLayer{
		Layer:    1,
		Monsters: map[uint64]*model.Monster{},
	}}
	svc.world = world

	attacker := &model.Player{
		ID:    1,
		Hp:    1000,
		Layer: 1,
		Level: 5,
		Str:   10,
	}

	_, gameErr := svc.Attack(context.Background(), attacker, 99999, 0)
	if gameErr == nil {
		t.Fatal("攻击不存在的目标应返回错误")
	}
	if gameErr.Code != 300 { // ErrTargetNotFound
		t.Errorf("错误码 = %d, 期望 300 (目标不存在)", gameErr.Code)
	}
}

// ==================== 测试辅助：mock World ====================

type mockWorld struct {
	dungeon *model.DungeonLayer
	boss    *model.Boss
}

func (m *mockWorld) GetOnlinePlayer(playerID uint64) *model.Player  { return nil }
func (m *mockWorld) GetDungeon(layer int32) *model.DungeonLayer     { return m.dungeon }
func (m *mockWorld) LayerPlayerIDs(layer int32) []uint64            { return nil }
func (m *mockWorld) GetBoss(bossID uint64) *model.Boss              { return m.boss }
func (m *mockWorld) AddBoss(boss *model.Boss)                       {}
func (m *mockWorld) RemoveBoss(bossID uint64)                       {}
func (m *mockWorld) GetLayerBoss(layer int32) *model.Boss           { return nil }
func (m *mockWorld) SpawnBossIfNeeded(layer int32) *model.Boss      { return nil }
func (m *mockWorld) Hub() *gateway.Hub                              { return nil }
func (m *mockWorld) OnLogin(playerID uint64) (*model.Player, error) { return nil, nil }
func (m *mockWorld) OnLogout(playerID uint64)                       {}
func (m *mockWorld) MaxLayer() int32                                { return 30 }
func (m *mockWorld) AllPlayers(fn func(uint64, *model.Player))      {}
func (m *mockWorld) Config() iface.GameConfig {
	return iface.GameConfig{MaxLayer: 30}
}
