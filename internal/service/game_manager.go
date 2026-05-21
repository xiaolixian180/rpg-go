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
	hub *gateway.Hub,
	cfg GameConfig,
) *GameManager {
	gm := &GameManager{
		playerSvc:  playerSvc,
		playerRepo: playerRepo,
		hub:        hub,
		cfg:        cfg,
		dungeons:   make(map[int32]*model.DungeonLayer),
	}
	gm.initDungeons()
	return gm
}

// initDungeons 初始化所有地下城层实例
func (gm *GameManager) initDungeons() {
	for i := int32(1); i <= gm.cfg.MaxLayer; i++ {
		gm.dungeons[i] = &model.DungeonLayer{
			Layer:     i,
			IsBoss:    model.IsBossLayer(i),
			Monsters:  make(map[uint64]*model.Monster),
			Players:   make(map[uint64]*model.Player),
			Resources: make(map[uint64]*model.Resource),
		}
	}
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
