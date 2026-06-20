package service

import (
	"context"
	"fmt"
	"sync"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/internal/service/player"

	loggerPkg "hero-quest/pkg/logger"
)

// GameManager 游戏世界状态管理器，负责：
//   - 在线玩家的内存生命周期管理（OnLogin/OnLogout）
//   - 地下城层实例的初始化与查询
//   - Boss 实例的增删查
//
// 业务逻辑由各 service 实现，消息处理由 handler 层实现，
// GameManager 不包含任何消息处理代码。
type GameManager struct {
	playerSvc  player.PlayerService // 玩家服务（用于登录加载数据、登出保存）
	playerRepo repo.PlayerRepo      // 玩家数据仓库（用于定时存档）
	equipRepo  repo.EquipRepo       // 装备数据仓库（用于登录加载装备）

	// 内存状态 — 使用 sync.Map 针对读多写少场景优化，万人在线无锁竞争
	players sync.Map // key: uint64 → *model.Player
	bosses  sync.Map // key: uint64 → *model.Boss

	// 地下城 — 固定30层，初始化后不再增删，各层自带 RWMutex
	dungeons map[int32]*model.DungeonLayer

	// 配置参数
	cfg GameConfig

	// 网关消息中心
	hub *gateway.Hub
}

// NewGameManager 创建游戏世界状态管理器
func NewGameManager(
	playerSvc player.PlayerService,
	playerRepo repo.PlayerRepo,
	equipRepo repo.EquipRepo,
	hub *gateway.Hub,
	cfg GameConfig,
) *GameManager {
	gm := &GameManager{
		playerSvc:  playerSvc,
		playerRepo: playerRepo,
		equipRepo:  equipRepo,
		hub:        hub,
		cfg:        cfg,
		dungeons:   make(map[int32]*model.DungeonLayer),
	}
	gm.initDungeons()
	return gm
}

// initDungeons 初始化所有地下城层实例，并生成怪物和资源
func (gm *GameManager) initDungeons() {
	for i := int32(1); i <= gm.cfg.MaxLayer; i++ {
		layer := &model.DungeonLayer{
			Layer:     i,
			IsBoss:    model.IsBossLayer(i),
			Monsters:  model.SpawnMonsters(i),
			Players:   make(map[uint64]*model.Player),
			Resources: model.SpawnResources(i),
		}
		gm.dungeons[i] = layer
	}

	// 为Boss层生成Boss实例
	for i := int32(1); i <= gm.cfg.MaxLayer; i++ {
		if !model.IsBossLayer(i) {
			continue
		}
		tmpl := model.GetBossTemplate(i)
		if tmpl == nil {
			continue
		}
		boss := model.SpawnBoss(tmpl)
		gm.bosses.Store(boss.ID, boss)
		loggerPkg.Info("Boss已生成", "layer", i, "boss_id", boss.ID, "name", boss.Name)
	}

	loggerPkg.Info("地下城初始化完成", "layers", gm.cfg.MaxLayer)
}

// ==================== World 接口实现 ====================

// GetOnlinePlayer 获取内存中的在线玩家
func (gm *GameManager) GetOnlinePlayer(playerID uint64) *model.Player {
	if v, ok := gm.players.Load(playerID); ok {
		return v.(*model.Player)
	}
	return nil
}

// GetDungeon 获取指定层的地下城实例
func (gm *GameManager) GetDungeon(layer int32) *model.DungeonLayer {
	return gm.dungeons[layer]
}

// LayerPlayerIDs 获取指定层当前玩家ID快照。
func (gm *GameManager) LayerPlayerIDs(layer int32) []uint64 {
	d := gm.dungeons[layer]
	if d == nil {
		return nil
	}
	d.Mu().RLock()
	defer d.Mu().RUnlock()
	ids := make([]uint64, 0, len(d.Players))
	for playerID := range d.Players {
		ids = append(ids, playerID)
	}
	return ids
}

// GetBoss 获取指定Boss实例
func (gm *GameManager) GetBoss(bossID uint64) *model.Boss {
	if v, ok := gm.bosses.Load(bossID); ok {
		return v.(*model.Boss)
	}
	return nil
}

// AddBoss 添加Boss实例到世界
func (gm *GameManager) AddBoss(boss *model.Boss) {
	gm.bosses.Store(boss.ID, boss)
}

// RemoveBoss 从世界中移除Boss实例
func (gm *GameManager) RemoveBoss(bossID uint64) {
	gm.bosses.Delete(bossID)
}

// GetLayerBoss 获取指定层的Boss实例（如果存在）
func (gm *GameManager) GetLayerBoss(layer int32) *model.Boss {
	var found *model.Boss
	gm.bosses.Range(func(key, value any) bool {
		b := value.(*model.Boss)
		if b.Layer == layer {
			found = b
			return false
		}
		return true
	})
	return found
}

// SpawnBossIfNeeded 检查Boss层是否需要重新生成Boss。
// 当Boss被击杀后从内存移除，冷却时间过后需要重新生成。
// 返回新生成的Boss实例（如果已存在则返回nil）。
func (gm *GameManager) SpawnBossIfNeeded(layer int32) *model.Boss {
	if !model.IsBossLayer(layer) {
		return nil
	}
	// 已有Boss则不重复生成
	if gm.GetLayerBoss(layer) != nil {
		return nil
	}
	tmpl := model.GetBossTemplate(layer)
	if tmpl == nil {
		return nil
	}
	boss := model.SpawnBoss(tmpl)
	gm.bosses.Store(boss.ID, boss)
	loggerPkg.Info("Boss重新生成", "layer", layer, "boss_id", boss.ID, "name", boss.Name)
	return boss
}

// Hub 获取网关消息中心
func (gm *GameManager) Hub() *gateway.Hub {
	return gm.hub
}

// MaxLayer 返回地下城最大层数
func (gm *GameManager) MaxLayer() int32 {
	return gm.cfg.MaxLayer
}

// Config 返回游戏配置参数
func (gm *GameManager) Config() GameConfig {
	return gm.cfg
}

// ==================== 生命周期管理 ====================

// OnLogin 处理玩家登录，加载到内存
func (gm *GameManager) OnLogin(playerID uint64) (*model.Player, error) {
	// 断线重连：已在内存中，直接标记在线
	if v, ok := gm.players.Load(playerID); ok {
		p := v.(*model.Player)
		p.Mu().Lock()
		p.Online = true
		p.Mu().Unlock()
		return p, nil
	}

	// 首次登录：从数据库加载
	ctx := context.Background()
	p, gameErr := gm.playerSvc.Login(ctx, playerID)
	if gameErr != nil {
		return nil, fmt.Errorf("player login: %s", gameErr.Error())
	}

	// 加载装备数据到内存
	if gm.equipRepo != nil {
		equips, err := gm.equipRepo.GetAllEquips(ctx, playerID)
		if err == nil {
			p.Mu().Lock()
			for _, eq := range equips {
				if eq.Slot >= 0 && int(eq.Slot) < len(p.EquippedItems) {
					p.EquippedItems[eq.Slot] = eq
				}
			}
			// 重新计算MaxHp（包含装备加成）
			p.MaxHp = p.CalcMaxHp()
			p.Hp = p.MaxHp
			p.Mu().Unlock()
		}
	}

	// 初始化物品背包和MP（DB不存，每次登录重算）
	p.InitItems()

	gm.players.Store(playerID, p)
	return p, nil
}

// OnLogout 处理玩家登出，从内存移除并持久化
func (gm *GameManager) OnLogout(playerID uint64) {
	v, ok := gm.players.LoadAndDelete(playerID)
	if !ok {
		return
	}
	p := v.(*model.Player)

	// 读取当前所在层，并从地下城层中移除
	p.Mu().RLock()
	layer := p.Layer
	p.Mu().RUnlock()
	if layer > 0 {
		if d, ok := gm.dungeons[layer]; ok {
			d.Mu().Lock()
			delete(d.Players, playerID)
			d.Mu().Unlock()
		}
	}

	// 标记为离线
	p.Mu().Lock()
	p.Online = false
	p.Mu().Unlock()

	// 使用内存中的玩家数据直接持久化，避免从 Redis/DB 读取过期数据
	ctx := context.Background()
	gm.playerSvc.LogoutWithPlayer(ctx, p)
}

// ==================== 辅助方法 ====================

// AllPlayers 遍历所有在线玩家（只读）
func (gm *GameManager) AllPlayers(fn func(playerID uint64, p *model.Player)) {
	gm.players.Range(func(key, value any) bool {
		fn(key.(uint64), value.(*model.Player))
		return true
	})
}

// PlayerCount 返回在线玩家数
func (gm *GameManager) PlayerCount() int {
	count := 0
	gm.players.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// ensure GameManager implements World at compile time
var _ World = (*GameManager)(nil)

// SaveAllDirty 遍历所有在线玩家，将标记为 Dirty 的玩家持久化到数据库。
// 由定时存档任务调用。
func (gm *GameManager) SaveAllDirty() {
	ctx := context.Background()
	saved := 0
	gm.players.Range(func(key, value any) bool {
		p := value.(*model.Player)
		p.Mu().Lock()
		dirty := p.Dirty
		if dirty {
			p.Dirty = false
		}
		p.Mu().Unlock()

		if dirty {
			if err := gm.playerRepo.SavePlayer(ctx, p); err != nil {
				loggerPkg.TError(ctx, "定时存档保存玩家失败", "player_id", key.(uint64), "err", err)
			} else {
				saved++
			}
		}
		return true
	})
	if saved > 0 {
		loggerPkg.Debug("定时存档完成", "saved", saved)
	}
}
