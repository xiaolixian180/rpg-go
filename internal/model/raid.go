package model

import (
	"math/rand"
	"sync"
	"time"
)

// ==================== 战局数据模型 ====================

// RaidMap 战局实例 — 独立的临时战局
// 每场战局从模板创建，包含自己的怪物、玩家、战利品容器和撤离点。
type RaidMap struct {
	ID               uint64                     // 战局唯一ID（UnixNano生成）
	TemplateID       int32                      // 模板ID（对应RaidMapTemplate.TemplateID）
	Name             string                     // 战局名称（来自模板）
	Monsters         map[uint64]*Monster         // 战局内怪物，复用现有 Monster struct
	Players          map[uint64]*Player          // 战局内玩家
	Zones            []RaidZone                  // 战局区域列表
	LootContainers   map[uint64]*RaidLootContainer // 战利品容器
	ExtractionPoints []RaidExtractionPoint       // 撤离点列表
	StartTime        time.Time                  // 战局开始时间
	Duration         time.Duration              // 战局持续时间
	mu               sync.RWMutex               // 读写锁，保护并发访问
}

// Mu 返回 RaidMap 的读写锁指针，供外部按需加锁保护并发操作
func (r *RaidMap) Mu() *sync.RWMutex { return &r.mu }

// RemainingSeconds 返回剩余秒数
func (r *RaidMap) RemainingSeconds() int64 {
	elapsed := time.Since(r.StartTime)
	remain := r.Duration - elapsed
	if remain < 0 {
		return 0
	}
	return int64(remain.Seconds())
}

// FindZone 返回坐标所在区域的指针，不在任何区域返回 nil
func (r *RaidMap) FindZone(x, y float64) *RaidZone {
	for i := range r.Zones {
		if r.Zones[i].InZone(x, y) {
			return &r.Zones[i]
		}
	}
	return nil
}

// FindExtractionPoint 根据ID查找撤离点，未找到返回 nil
func (r *RaidMap) FindExtractionPoint(pointID int32) *RaidExtractionPoint {
	for i := range r.ExtractionPoints {
		if r.ExtractionPoints[i].ID == pointID {
			return &r.ExtractionPoints[i]
		}
	}
	return nil
}

// RaidZone 战局区域，用于判断PvP开关和战利品品质
type RaidZone struct {
	ID         int32   // 区域ID
	Name       string  // 区域名称
	X1, Y1     float64 // 左上角坐标
	X2, Y2     float64 // 右下角坐标
	PvPEnabled bool    // 是否允许PvP
	LootTier   int32   // 1~5 影响战利品品质
}

// InZone 判断坐标是否在区域内
func (z *RaidZone) InZone(x, y float64) bool {
	return x >= z.X1 && x <= z.X2 && y >= z.Y1 && y <= z.Y2
}

// RaidExtractionPoint 撤离点，玩家在此等待倒计时后成功撤离
type RaidExtractionPoint struct {
	ID              int32   // 撤离点ID
	X, Y            float64 // 撤离点坐标
	Radius          float64 // 进入撤离范围的半径
	ExtractDuration int32   // 撤离所需秒数
}

// RaidLootContainer 战利品容器，打开后生成随机物品
type RaidLootContainer struct {
	ID        uint64         // 容器唯一ID
	X, Y      float64        // 容器坐标
	LootTable int32          // 战利品表ID（决定掉落物池）
	Opened    bool           // 是否已打开
	Items     []RaidLootItem // 打开后填充的物品列表
	mu        sync.Mutex     // 互斥锁，保护 Opened 和 Items 的并发访问
}

// Mu 返回 RaidLootContainer 的互斥锁指针
func (c *RaidLootContainer) Mu() *sync.Mutex { return &c.mu }

// RaidState 玩家战局状态（挂在 Player.RaidState）
// 玩家在战局中时非 nil，离开/死亡/超时后清空
type RaidState struct {
	MapID                   uint64          // 当前所在战局ID
	RaidInventory           []*RaidLootItem // 战局内临时背包（撤离成功才保留）
	Extracting              bool            // 是否正在撤离中
	ExtractTimer            int32           // 撤离倒计时秒数
	ExtractPointID          int32           // 当前正在使用的撤离点ID
	CurrentZone             int32           // 当前所在区域ID
	PvPFlag                 bool            // 是否处于PvP区域
	LastOpenedContainerID   uint64          // 最近打开的容器ID（用于拾取操作）
}

// RaidLootItem 战利品物品，可放入战局临时背包或战局仓库
type RaidLootItem struct {
	ItemID  int32  // 物品模板ID
	Count   int32  // 数量
	Quality int32  // 品质等级（对应 QualityXxx 常量）
	Name    string // 物品名称
}

// ==================== 战局模板 ====================

// RaidMapTemplate 战局地图模板，硬编码原型阶段
type RaidMapTemplate struct {
	TemplateID     int32                   // 模板ID
	Name           string                  // 地图名称
	Duration       time.Duration           // 战局持续时间
	Zones          []RaidZone              // 区域定义
	MonsterLayer   int32                   // 使用哪一层的怪物模板
	MonsterCount   int                     // 生成的怪物数量
	LootContainers []RaidLootContainerSpec // 战利品容器规格
	ExtractPoints  []RaidExtractionPoint   // 撤离点定义
	MinLootTier    int32                   // 最低战利品品质
	MaxLootTier    int32                   // 最高战利品品质
}

// RaidLootContainerSpec 战利品容器规格（模板中使用）
type RaidLootContainerSpec struct {
	X, Y      float64 // 容器坐标
	LootTable int32   // 战利品表ID
}

// RaidMapTemplates 战局模板表（硬编码3张地图）
var RaidMapTemplates = map[int32]*RaidMapTemplate{
	// 翠绿密林：入门级战局，适合低等级玩家
	1: {
		TemplateID:   1,
		Name:         "翠绿密林",
		Duration:     20 * time.Minute,
		MonsterLayer: 5,
		MonsterCount: 5,
		MinLootTier:  1,
		MaxLootTier:  2,
		Zones: []RaidZone{
			{ID: 1, Name: "安全营地", X1: 0, Y1: 0, X2: 30, Y2: 30, PvPEnabled: false, LootTier: 1},
			{ID: 2, Name: "密林深处", X1: 30, Y1: 30, X2: 100, Y2: 100, PvPEnabled: true, LootTier: 2},
		},
		LootContainers: []RaidLootContainerSpec{
			{X: 45, Y: 55, LootTable: 1},
			{X: 65, Y: 40, LootTable: 1},
			{X: 80, Y: 75, LootTable: 2},
			{X: 55, Y: 85, LootTable: 1},
		},
		ExtractPoints: []RaidExtractionPoint{
			{ID: 1, X: 90, Y: 90, Radius: 5, ExtractDuration: 10},
		},
	},

	// 烈焰矿洞：中级战局，怪物更强，战利品更丰富
	2: {
		TemplateID:   2,
		Name:         "烈焰矿洞",
		Duration:     20 * time.Minute,
		MonsterLayer: 15,
		MonsterCount: 8,
		MinLootTier:  2,
		MaxLootTier:  3,
		Zones: []RaidZone{
			{ID: 1, Name: "矿洞入口", X1: 0, Y1: 0, X2: 20, Y2: 20, PvPEnabled: false, LootTier: 2},
			{ID: 2, Name: "熔岩矿脉", X1: 20, Y1: 20, X2: 80, Y2: 80, PvPEnabled: true, LootTier: 3},
		},
		LootContainers: []RaidLootContainerSpec{
			{X: 30, Y: 35, LootTable: 2},
			{X: 50, Y: 45, LootTable: 2},
			{X: 65, Y: 55, LootTable: 3},
			{X: 40, Y: 70, LootTable: 2},
			{X: 55, Y: 30, LootTable: 3},
			{X: 70, Y: 65, LootTable: 2},
		},
		ExtractPoints: []RaidExtractionPoint{
			{ID: 1, X: 75, Y: 75, Radius: 5, ExtractDuration: 12},
		},
	},

	// 暗影要塞：高级战局，三个区域，双撤离点
	3: {
		TemplateID:   3,
		Name:         "暗影要塞",
		Duration:     25 * time.Minute,
		MonsterLayer: 25,
		MonsterCount: 12,
		MinLootTier:  3,
		MaxLootTier:  5,
		Zones: []RaidZone{
			{ID: 1, Name: "要塞外围", X1: 0, Y1: 0, X2: 15, Y2: 15, PvPEnabled: false, LootTier: 3},
			{ID: 2, Name: "要塞回廊", X1: 15, Y1: 15, X2: 50, Y2: 50, PvPEnabled: false, LootTier: 4},
			{ID: 3, Name: "暗影王座", X1: 50, Y1: 50, X2: 100, Y2: 100, PvPEnabled: true, LootTier: 5},
		},
		LootContainers: []RaidLootContainerSpec{
			{X: 25, Y: 20, LootTable: 3},
			{X: 35, Y: 40, LootTable: 3},
			{X: 20, Y: 45, LootTable: 4},
			{X: 45, Y: 25, LootTable: 3},
			{X: 65, Y: 60, LootTable: 4},
			{X: 80, Y: 70, LootTable: 5},
			{X: 70, Y: 85, LootTable: 4},
			{X: 90, Y: 55, LootTable: 5},
		},
		ExtractPoints: []RaidExtractionPoint{
			{ID: 1, X: 5, Y: 95, Radius: 5, ExtractDuration: 15},
			{ID: 2, X: 95, Y: 5, Radius: 5, ExtractDuration: 15},
		},
	},
}

// GetRaidMapTemplate 获取战局模板，不存在返回 nil
func GetRaidMapTemplate(templateID int32) *RaidMapTemplate {
	if t, ok := RaidMapTemplates[templateID]; ok {
		return t
	}
	return nil
}

// ==================== 战利品表 ====================

// raidLootEntry 战利品表条目
type raidLootEntry struct {
	ItemID  int32
	Name    string
	Quality int32
	Weight  int32 // 权重，用于加权随机
}

// raidLootTables 战利品表定义，按 LootTable ID 分组
var raidLootTables = map[int32][]raidLootEntry{
	// 表1：初级战利品（翠绿密林）
	1: {
		{ItemID: 1, Name: "小型生命药水", Quality: QualityWhite, Weight: 40},
		{ItemID: 3, Name: "小型魔法药水", Quality: QualityWhite, Weight: 30},
		{ItemID: 2, Name: "大型生命药水", Quality: QualityGreen, Weight: 15},
		{ItemID: 5, Name: "全能药水", Quality: QualityGreen, Weight: 10},
		{ItemID: 2001, Name: "草药", Quality: QualityWhite, Weight: 30},
		{ItemID: 2003, Name: "木材", Quality: QualityGreen, Weight: 10},
	},
	// 表2：中级战利品（烈焰矿洞）
	2: {
		{ItemID: 2, Name: "大型生命药水", Quality: QualityGreen, Weight: 30},
		{ItemID: 4, Name: "大型魔法药水", Quality: QualityGreen, Weight: 25},
		{ItemID: 5, Name: "全能药水", Quality: QualityBlue, Weight: 15},
		{ItemID: 2002, Name: "铜矿石", Quality: QualityGreen, Weight: 20},
		{ItemID: 2004, Name: "秘银矿石", Quality: QualityBlue, Weight: 8},
	},
	// 表3：高级战利品（暗影要塞外围）
	3: {
		{ItemID: 2, Name: "大型生命药水", Quality: QualityGreen, Weight: 20},
		{ItemID: 4, Name: "大型魔法药水", Quality: QualityGreen, Weight: 20},
		{ItemID: 5, Name: "全能药水", Quality: QualityBlue, Weight: 20},
		{ItemID: 2004, Name: "秘银矿石", Quality: QualityBlue, Weight: 15},
		{ItemID: 2001, Name: "稀有草药", Quality: QualityBlue, Weight: 10},
	},
	// 表4：稀有战利品（暗影要塞回廊）
	4: {
		{ItemID: 5, Name: "全能药水", Quality: QualityBlue, Weight: 25},
		{ItemID: 2004, Name: "秘银矿石", Quality: QualityBlue, Weight: 20},
		{ItemID: 2002, Name: "银矿石", Quality: QualityPurple, Weight: 10},
	},
	// 表5：传说战利品（暗影王座）
	5: {
		{ItemID: 5, Name: "全能药水", Quality: QualityPurple, Weight: 20},
		{ItemID: 2004, Name: "秘银矿石", Quality: QualityPurple, Weight: 15},
		{ItemID: 2001, Name: "远古草药", Quality: QualityPurple, Weight: 10},
		{ItemID: 2003, Name: "火焰木", Quality: QualityOrange, Weight: 5},
	},
}

// GenerateRaidLoot 根据战利品表和品质范围生成随机战利品（1~3件）
func GenerateRaidLoot(lootTable int32, minTier, maxTier int32) []RaidLootItem {
	table, ok := raidLootTables[lootTable]
	if !ok || len(table) == 0 {
		return nil
	}

	// 计算总权重
	var totalWeight int32
	for _, e := range table {
		totalWeight += e.Weight
	}
	if totalWeight <= 0 {
		return nil
	}

	// 生成1~3件物品
	count := 1 + rand.Intn(3)
	items := make([]RaidLootItem, 0, count)
	for i := 0; i < count; i++ {
		r := rand.Int31n(totalWeight)
		var cumulative int32
		for _, e := range table {
			cumulative += e.Weight
			if r < cumulative {
				// 品质在 minTier~maxTier 范围内浮动
				quality := e.Quality
				if quality < minTier {
					quality = minTier
				}
				if quality > maxTier {
					quality = maxTier
				}
				items = append(items, RaidLootItem{
					ItemID:  e.ItemID,
					Count:   1 + int32(rand.Intn(3)), // 1~3个
					Quality: quality,
					Name:    e.Name,
				})
				break
			}
		}
	}
	return items
}

// ==================== 战局创建 ====================

// monsterIDCounter 战局怪物ID计数器，避免与地下城怪物ID冲突
var monsterIDCounter uint64 = 1_000_000_000

// lootContainerIDCounter 战利品容器ID计数器
var lootContainerIDCounter uint64 = 2_000_000_000

// CreateRaidMap 根据模板创建战局实例
// templateID: 地图模板ID
// playerID: 创建战局的玩家ID
// player: 创建战局的玩家指针
// 返回 nil 表示模板不存在
func CreateRaidMap(templateID int32, playerID uint64, player *Player) *RaidMap {
	tmpl := GetRaidMapTemplate(templateID)
	if tmpl == nil {
		return nil
	}

	// 生成战局唯一ID
	raidID := uint64(time.Now().UnixNano())

	// 生成怪物：从模板指定层的怪物模板池中循环生成 MonsterCount 只
	templates := GetMonsterTemplates(tmpl.MonsterLayer)
	monsters := make(map[uint64]*Monster, tmpl.MonsterCount)
	for i := 0; i < tmpl.MonsterCount; i++ {
		t := templates[i%len(templates)]
		monsterID := monsterIDCounter
		monsterIDCounter++

		// 在PvP区域内随机生成坐标（优先在非安全区）
		var x, y float64
		if len(tmpl.Zones) > 1 {
			// 在第二个区域（野区）中随机生成
			zone := tmpl.Zones[1]
			x = zone.X1 + rand.Float64()*(zone.X2-zone.X1)
			y = zone.Y1 + rand.Float64()*(zone.Y2-zone.Y1)
		} else {
			x = rand.Float64() * 100
			y = rand.Float64() * 100
		}

		layerMul := int64(tmpl.MonsterLayer)
		maxHp := t.Hp + layerMul*10

		// 默认AI参数
		aggroRange := t.AggroRange
		atkRange := t.AtkRange
		atkCD := t.AtkCD
		if aggroRange == 0 {
			aggroRange = 30
		}
		if atkRange == 0 {
			atkRange = 5
		}
		if atkCD == 0 {
			atkCD = 3
		}
		if t.Elite {
			aggroRange *= 1.3
			atkCD *= 0.8
		}

		m := &Monster{
			ID:         monsterID,
			Name:       t.Name,
			MaxHp:      maxHp,
			Hp:         maxHp + rand.Int63n(20),
			Atk:        t.Atk,
			Def:        t.Def,
			ExpReward:  t.ExpReward,
			GoldReward: t.GoldReward,
			X:          x,
			Y:          y,
			Elite:      t.Elite,
			Element:    t.Element,
			Rarity:     t.Rarity,
			Skills:     t.Skills,
			Behavior:   t.Behavior,
			AIState:    MonsterAIIdle,
			SpawnX:     x,
			SpawnY:     y,
			AggroRange: aggroRange,
			AtkRange:   atkRange,
			AtkCD:      atkCD,
		}
		if m.Hp > m.MaxHp {
			m.Hp = m.MaxHp
		}
		monsters[monsterID] = m
	}

	// 生成战利品容器
	lootContainers := make(map[uint64]*RaidLootContainer, len(tmpl.LootContainers))
	for _, spec := range tmpl.LootContainers {
		cid := lootContainerIDCounter
		lootContainerIDCounter++
		lootContainers[cid] = &RaidLootContainer{
			ID:        cid,
			X:         spec.X,
			Y:         spec.Y,
			LootTable: spec.LootTable,
		}
	}

	// 复制区域和撤离点（避免共享模板数据）
	zones := make([]RaidZone, len(tmpl.Zones))
	copy(zones, tmpl.Zones)
	extractPoints := make([]RaidExtractionPoint, len(tmpl.ExtractPoints))
	copy(extractPoints, tmpl.ExtractPoints)

	// 创建战局实例
	raid := &RaidMap{
		ID:               raidID,
		TemplateID:       templateID,
		Name:             tmpl.Name,
		Monsters:         monsters,
		Players:          map[uint64]*Player{playerID: player},
		Zones:            zones,
		LootContainers:   lootContainers,
		ExtractionPoints: extractPoints,
		StartTime:        time.Now(),
		Duration:         tmpl.Duration,
	}

	// 设置玩家战局状态
	player.Mu().Lock()
	player.RaidState = &RaidState{
		MapID:         raidID,
		RaidInventory: make([]*RaidLootItem, 0),
		CurrentZone:   1, // 默认在安全区
	}
	player.X = 5 // 出生点在安全区
	player.Y = 5
	player.Mu().Unlock()

	return raid
}
