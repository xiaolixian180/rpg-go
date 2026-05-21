package model

import (
	"math/rand"
	"sync"
)

// DungeonLayer 副本单层结构，包含该层的怪物、玩家和资源
type DungeonLayer struct {
	Layer     int32                // 层数编号
	IsBoss    bool                 // 是否为Boss层（每10层为Boss层）
	Monsters  map[uint64]*Monster  // 该层所有怪物，key为怪物ID
	Players   map[uint64]*Player   // 该层所有玩家，key为玩家ID
	Resources map[uint64]*Resource // 该层所有可采集资源，key为资源ID
	mu        sync.RWMutex         // 读写锁，保护并发访问
}

// Mu 返回 DungeonLayer 的读写锁指针，供外部按需加锁保护并发操作
func (d *DungeonLayer) Mu() *sync.RWMutex {
	return &d.mu
}

// Monster 副本中的普通怪物
type Monster struct {
	ID         uint64       // 怪物唯一标识
	Name       string       // 怪物名称
	Hp         int64        // 当前生命值
	MaxHp      int64        // 最大生命值
	Atk        int64        // 攻击力
	Def        int64        // 防御力
	ExpReward  int64        // 击杀后奖励的经验值
	GoldReward int64        // 击杀后奖励的金币数
	X          float64      // 地图中的X坐标
	Y          float64      // 地图中的Y坐标
	Dead       bool         // 是否已死亡（防止并发攻击重复发奖）
	mu         sync.RWMutex // 读写锁，保护并发访问
}

// Mu 返回 Monster 的读写锁指针，供外部按需加锁保护并发操作
func (m *Monster) Mu() *sync.RWMutex {
	return &m.mu
}

// Resource 副本中可采集的资源点
type Resource struct {
	ID        uint64       // 资源唯一标识
	Type      int32        // 资源类型（0=矿石 1=草药 2=木材）
	Name      string       // 资源名称
	ItemID    int32        // 产出的物品模板ID
	Count     int32        // 产出数量
	X         float64      // 地图中的X坐标
	Y         float64      // 地图中的Y坐标
	Harvested bool         // 是否已被采集
	mu        sync.RWMutex // 读写锁，保护 Harvested 等字段的并发访问
}

// Mu 返回 Resource 的读写锁指针，供外部按需加锁保护并发操作
func (r *Resource) Mu() *sync.RWMutex {
	return &r.mu
}

// MonsterTemplate 怪物模板
type MonsterTemplate struct {
	ID         uint64 // 模板ID
	Name       string // 怪物名称
	Hp         int64  // 基础HP
	Atk        int64  // 基础攻击
	Def        int64  // 基础防御
	ExpReward  int64  // 经验奖励
	GoldReward int64  // 金币奖励
}

// MonsterTemplates 按层数分组的怪物模板
var MonsterTemplates = map[int32][]*MonsterTemplate{
	// 1~10层 翠绿森林
	1: {{ID: 10001, Name: "史莱姆", Hp: 50, Atk: 5, Def: 2, ExpReward: 10, GoldReward: 5}},
	2: {{ID: 10002, Name: "野狼", Hp: 80, Atk: 10, Def: 5, ExpReward: 20, GoldReward: 8}},
	3: {{ID: 10003, Name: "毒蘑菇", Hp: 60, Atk: 8, Def: 3, ExpReward: 15, GoldReward: 6}},
	4: {{ID: 10004, Name: "哥布林", Hp: 100, Atk: 12, Def: 6, ExpReward: 25, GoldReward: 10}},
	5: {{ID: 10005, Name: "树人", Hp: 150, Atk: 15, Def: 10, ExpReward: 40, GoldReward: 15}},
	6: {{ID: 10006, Name: "森林蜘蛛", Hp: 120, Atk: 18, Def: 8, ExpReward: 35, GoldReward: 12}},
	7: {{ID: 10007, Name: "独角兽", Hp: 200, Atk: 20, Def: 12, ExpReward: 50, GoldReward: 20}},
	8: {{ID: 10008, Name: "精灵射手", Hp: 180, Atk: 25, Def: 10, ExpReward: 55, GoldReward: 22}},
	9: {{ID: 10009, Name: "森林守卫", Hp: 250, Atk: 28, Def: 15, ExpReward: 65, GoldReward: 25}},
	// 11~20层 腐蚀沼泽
	11: {{ID: 10011, Name: "沼泽蜥蜴", Hp: 300, Atk: 30, Def: 18, ExpReward: 80, GoldReward: 30}},
	12: {{ID: 10012, Name: "毒蛇女王", Hp: 350, Atk: 35, Def: 20, ExpReward: 100, GoldReward: 35}},
	13: {{ID: 10013, Name: "腐烂树精", Hp: 400, Atk: 38, Def: 25, ExpReward: 110, GoldReward: 40}},
	14: {{ID: 10014, Name: "沼泽巨蛙", Hp: 380, Atk: 40, Def: 22, ExpReward: 120, GoldReward: 42}},
	15: {{ID: 10015, Name: "暗影蛇", Hp: 450, Atk: 45, Def: 28, ExpReward: 140, GoldReward: 50}},
	16: {{ID: 10016, Name: "腐化骑士", Hp: 500, Atk: 48, Def: 30, ExpReward: 160, GoldReward: 55}},
	17: {{ID: 10017, Name: "沼泽领主", Hp: 550, Atk: 52, Def: 35, ExpReward: 180, GoldReward: 60}},
	18: {{ID: 10018, Name: "暗夜猎手", Hp: 520, Atk: 55, Def: 32, ExpReward: 190, GoldReward: 65}},
	19: {{ID: 10019, Name: "沼泽巫妖", Hp: 600, Atk: 58, Def: 38, ExpReward: 210, GoldReward: 70}},
	// 21~30层 烈焰火山
	21: {{ID: 10021, Name: "火焰元素", Hp: 700, Atk: 60, Def: 40, ExpReward: 250, GoldReward: 80}},
	22: {{ID: 10022, Name: "熔岩巨人", Hp: 800, Atk: 65, Def: 50, ExpReward: 300, GoldReward: 90}},
	23: {{ID: 10023, Name: "火蜥蜴", Hp: 750, Atk: 68, Def: 45, ExpReward: 320, GoldReward: 95}},
	24: {{ID: 10024, Name: "烈焰骑士", Hp: 850, Atk: 72, Def: 52, ExpReward: 350, GoldReward: 100}},
	25: {{ID: 10025, Name: "火山守卫", Hp: 900, Atk: 75, Def: 55, ExpReward: 380, GoldReward: 110}},
	26: {{ID: 10026, Name: "炎魔", Hp: 1000, Atk: 80, Def: 58, ExpReward: 420, GoldReward: 120}},
	27: {{ID: 10027, Name: "火焰领主", Hp: 1100, Atk: 85, Def: 62, ExpReward: 460, GoldReward: 130}},
	28: {{ID: 10028, Name: "地狱犬", Hp: 1050, Atk: 88, Def: 60, ExpReward: 480, GoldReward: 135}},
	29: {{ID: 10029, Name: "炎龙幼崽", Hp: 1200, Atk: 92, Def: 65, ExpReward: 520, GoldReward: 150}},
}

// ResourceTemplates 按层数分组的资源模板
var ResourceTemplates = map[int32][]*Resource{
	1:  {{ID: 30001, Type: 1, Name: "草药", ItemID: 2001, Count: 2}, {ID: 30002, Type: 0, Name: "铜矿石", ItemID: 2002, Count: 1}},
	5:  {{ID: 30005, Type: 2, Name: "木材", ItemID: 2003, Count: 2}},
	11: {{ID: 30011, Type: 1, Name: "稀有草药", ItemID: 2001, Count: 3}, {ID: 30012, Type: 0, Name: "银矿石", ItemID: 2002, Count: 2}},
	21: {{ID: 30021, Type: 0, Name: "秘银矿石", ItemID: 2004, Count: 1}, {ID: 30022, Type: 2, Name: "火焰木", ItemID: 2003, Count: 3}},
}

// GetMonsterTemplates 获取指定层的怪物模板，没有则按层级生成
func GetMonsterTemplates(layer int32) []*MonsterTemplate {
	if t, ok := MonsterTemplates[layer]; ok {
		return t
	}
	// 按层级自动生成
	levelMul := int64(layer)
	return []*MonsterTemplate{
		{
			ID:         uint64(20000 + layer),
			Name:       "深渊怪物",
			Hp:         50 + levelMul*30,
			Atk:        5 + levelMul*3,
			Def:        2 + levelMul*2,
			ExpReward:  10 + levelMul*10,
			GoldReward: 5 + levelMul*5,
		},
	}
}

// GetResourceTemplates 获取指定层的资源模板
func GetResourceTemplates(layer int32) []*Resource {
	if t, ok := ResourceTemplates[layer]; ok {
		return t
	}
	return nil
}

// SpawnMonsters 为指定层生成怪物实例
func SpawnMonsters(layer int32) map[uint64]*Monster {
	templates := GetMonsterTemplates(layer)
	monsters := make(map[uint64]*Monster, len(templates))
	for i, t := range templates {
		layerMul := int64(layer)
		maxHp := t.Hp + layerMul*10
		m := &Monster{
			ID:         t.ID + uint64(i)*100,
			Name:       t.Name,
			MaxHp:      maxHp,
			Hp:         maxHp + rand.Int63n(20), // 随机HP波动不超过MaxHp+20
			Atk:        t.Atk,
			Def:        t.Def,
			ExpReward:  t.ExpReward,
			GoldReward: t.GoldReward,
			X:          rand.Float64() * 100,
			Y:          rand.Float64() * 100,
		}
		// 确保Hp不超过MaxHp
		if m.Hp > m.MaxHp {
			m.Hp = m.MaxHp
		}
		monsters[m.ID] = m
	}
	return monsters
}

// SpawnResources 为指定层生成资源实例
func SpawnResources(layer int32) map[uint64]*Resource {
	templates := GetResourceTemplates(layer)
	resources := make(map[uint64]*Resource, len(templates))
	for _, t := range templates {
		r := &Resource{
			ID:     t.ID,
			Type:   t.Type,
			Name:   t.Name,
			ItemID: t.ItemID,
			Count:  t.Count,
			X:      rand.Float64() * 100,
			Y:      rand.Float64() * 100,
		}
		resources[r.ID] = r
	}
	return resources
}

// 区域名称常量，按副本层数划分不同区域
const (
	ZoneForest  = "翠绿森林" // 1~10层区域
	ZoneSwamp   = "腐蚀沼泽" // 11~20层区域
	ZoneVolcano = "烈焰火山" // 21~30层区域
)

// GetZoneByLayer 根据副本层数返回对应的区域名称
func GetZoneByLayer(layer int32) string {
	switch {
	case layer >= 1 && layer <= 10:
		return ZoneForest
	case layer >= 11 && layer <= 20:
		return ZoneSwamp
	case layer >= 21 && layer <= 30:
		return ZoneVolcano
	default:
		return "未知区域"
	}
}

// IsBossLayer 判断指定层数是否为Boss层（每10层出现Boss）
func IsBossLayer(layer int32) bool {
	return layer%10 == 0
}

// DungeonProgressORM 副本进度表持久化模型，对应 dungeon_progress 表。
type DungeonProgressORM struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PlayerID uint64 `gorm:"not null;uniqueIndex;index:idx_player_id" json:"player_id"`
	MaxLayer int32  `gorm:"not null;default:0" json:"max_layer"`
}

// TableName 指定 DungeonProgressORM 对应的数据库表名
func (DungeonProgressORM) TableName() string { return "dungeon_progress" }
