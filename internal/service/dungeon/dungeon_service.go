// Package dungeon - 地下城服务
// 提供进入/离开地下城、层间传送等地下城相关业务逻辑
package dungeon

import (
	"context"
	"fmt"

	"hero-quest/internal/model"
	"hero-quest/internal/rbac"
	"hero-quest/internal/repo"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 地下城服务接口 ====================

// DungeonService 地下城服务接口，定义进入/离开地下城、层间传送等业务操作
type DungeonService interface {
	// Enter 进入地下城指定层，校验合法性，更新玩家所在层，返回目标层实例
	Enter(ctx context.Context, player *model.Player, layer int32) (*model.DungeonLayer, *errors.GameError)
	// Leave 离开地下城，将玩家从对应层的玩家表中移除
	Leave(ctx context.Context, player *model.Player) *errors.GameError
	// Teleport 层间传送，从当前层传送到目标层，消耗金币
	Teleport(ctx context.Context, player *model.Player, targetLayer int32) (*model.DungeonLayer, *errors.GameError)
	// GetDungeonInfo 获取玩家当前地下城详细信息（当前层、最高层、层数据摘要）
	GetDungeonInfo(ctx context.Context, player *model.Player) (*DungeonInfoResult, *errors.GameError)
}

// ==================== 地下城信息结果结构体 ====================

// DungeonInfoResult 地下城信息结果
type DungeonInfoResult struct {
	CurrentLayer  int32  // 当前层数
	MaxLayer      int32  // 已通关最高层
	MonsterCount  int32  // 当前层存活怪物数
	ResourceCount int32  // 当前层可采集资源数
	PlayerCount   int32  // 当前层在线玩家数
	ZoneName      string // 当前区域名称
}

// ==================== 地下城服务实现 ====================

// dungeonService 地下城服务实现
type dungeonService struct {
	enforcer   *rbac.Enforcer  // Casbin 权限执行器，用于校验玩家进入权限
	world      iface.World     // 游戏世界（获取地下城实例和配置）
	playerRepo repo.PlayerRepo // 玩家数据访问（持久化通关进度）
}

// NewDungeonService 创建地下城服务实例
// enforcer: Casbin 权限执行器，传入 nil 则跳过权限校验
// world: 游戏世界接口，用于获取地下城实例和配置参数
func NewDungeonService(enforcer *rbac.Enforcer, world iface.World, playerRepo repo.PlayerRepo) DungeonService {
	return &dungeonService{enforcer: enforcer, world: world, playerRepo: playerRepo}
}

// Enter 进入地下城业务逻辑（修复：读取player.MaxLayer加锁保护）：
//  1. 校验目标层数是否合法（1 ~ dungeonMaxLayer）
//  2. 校验玩家是否已解锁该层（只能进入已通关最高层+1或已通关层）
//  3. 校验玩家是否有进入该层的权限（Casbin RBAC）
//  4. 更新玩家当前所在层
//  5. 将玩家加入目标层的玩家表
//  6. 返回目标层的地下城实例（包含怪物、玩家、资源列表）
func (s *dungeonService) Enter(ctx context.Context, player *model.Player, layer int32) (*model.DungeonLayer, *errors.GameError) {
	maxLayer := s.world.MaxLayer()

	// 校验层数范围
	if layer < 1 || layer > maxLayer {
		return nil, errors.ErrLayerInvalid
	}

	// 校验通关进度：只能进入已通关最高层+1或已通关层（加锁读取MaxLayer）
	player.Mu().RLock()
	playerMaxLayer := player.MaxLayer
	player.Mu().RUnlock()
	if layer > playerMaxLayer+1 {
		return nil, errors.ErrLayerLocked
	}

	// 校验玩家是否有进入该层的权限（Casbin RBAC）
	if s.enforcer != nil {
		obj := fmt.Sprintf("dungeon:%d", layer)
		if !s.enforcer.Check("player", obj, "enter") {
			return nil, errors.ErrLayerLocked
		}
	}

	// 获取目标层地下城实例
	dungeon := s.world.GetDungeon(layer)
	if dungeon == nil {
		return nil, errors.ErrLayerInvalid
	}

	// 更新玩家所在层（先从旧层移除，再加入新层）
	player.Mu().Lock()
	oldLayer := player.Layer
	player.Layer = layer
	playerID := player.ID
	// 进入新最高层时更新MaxLayer并持久化
	if layer > player.MaxLayer {
		player.MaxLayer = layer
	}
	player.Mu().Unlock()

	// 持久化通关进度到数据库
	if s.playerRepo != nil && layer > playerMaxLayer {
		if err := s.playerRepo.SaveMaxLayer(ctx, playerID, layer); err != nil {
			logger.TError(ctx, "保存通关进度失败", "player_id", playerID, "layer", layer, "err", err)
		}
	}

	// 从旧层移除（如果之前在某层）
	if oldLayer > 0 {
		if oldD := s.world.GetDungeon(oldLayer); oldD != nil {
			oldD.Mu().Lock()
			delete(oldD.Players, playerID)
			oldD.Mu().Unlock()
		}
	}

	// 加入新层
	dungeon.Mu().Lock()
	dungeon.Players[playerID] = player
	dungeon.Mu().Unlock()

	logger.TInfo(ctx, "玩家进入地下城", "player_id", playerID, "layer", layer)
	return dungeon, nil
}

// Leave 离开地下城业务逻辑：
//  1. 校验玩家是否在地下城中（Layer > 0）
//  2. 将玩家从当前层的玩家表中移除
//  3. 将玩家Layer设为0
func (s *dungeonService) Leave(ctx context.Context, player *model.Player) *errors.GameError {
	player.Mu().Lock()
	layer := player.Layer
	playerID := player.ID
	if layer <= 0 {
		player.Mu().Unlock()
		return errors.ErrNotInDungeon
	}
	player.Layer = 0
	player.Mu().Unlock()

	// 从地下城层的玩家表中移除
	if d := s.world.GetDungeon(layer); d != nil {
		d.Mu().Lock()
		delete(d.Players, playerID)
		d.Mu().Unlock()
	}

	logger.TInfo(ctx, "玩家离开地下城", "player_id", playerID, "layer", layer)
	return nil
}

// Teleport 层间传送业务逻辑：
//  1. 校验目标层数是否合法
//  2. 校验玩家是否已解锁目标层
//  3. 扣除传送金币
//  4. 从旧层移除，加入新层
//  5. 返回目标层的地下城实例
func (s *dungeonService) Teleport(ctx context.Context, player *model.Player, targetLayer int32) (*model.DungeonLayer, *errors.GameError) {
	maxLayer := s.world.MaxLayer()
	cfg := s.world.Config()

	// 校验层数范围
	if targetLayer < 1 || targetLayer > maxLayer {
		return nil, errors.ErrLayerInvalid
	}

	// 校验通关进度（加锁读取MaxLayer）
	player.Mu().RLock()
	playerMaxLayer := player.MaxLayer
	player.Mu().RUnlock()
	if targetLayer > playerMaxLayer {
		return nil, errors.ErrLayerLocked
	}

	// 扣除传送金币，并更新所在层（同一把锁内完成）
	player.Mu().Lock()
	if player.Gold < cfg.TeleportCost {
		player.Mu().Unlock()
		return nil, errors.ErrGoldNotEnough
	}
	player.Gold -= cfg.TeleportCost
	oldLayer := player.Layer
	player.Layer = targetLayer
	playerID := player.ID
	player.Mu().Unlock()

	// 从旧层移除
	if oldLayer > 0 {
		if oldD := s.world.GetDungeon(oldLayer); oldD != nil {
			oldD.Mu().Lock()
			delete(oldD.Players, playerID)
			oldD.Mu().Unlock()
		}
	}

	// 获取目标层并加入
	dungeon := s.world.GetDungeon(targetLayer)
	if dungeon != nil {
		dungeon.Mu().Lock()
		dungeon.Players[playerID] = player
		dungeon.Mu().Unlock()
	}

	logger.TInfo(ctx, "玩家层间传送", "player_id", playerID, "from", oldLayer, "to", targetLayer, "cost", cfg.TeleportCost)
	return dungeon, nil
}

// GetDungeonInfo 获取玩家当前地下城详细信息（修复：返回有用的层数据摘要）：
//  1. 读取玩家当前层和最高层
//  2. 查询当前层的怪物、资源、玩家数量
//  3. 返回完整地下城信息
func (s *dungeonService) GetDungeonInfo(ctx context.Context, player *model.Player) (*DungeonInfoResult, *errors.GameError) {
	if player == nil {
		return nil, errors.ErrNotLogin
	}

	player.Mu().RLock()
	currentLayer := player.Layer
	maxLayer := player.MaxLayer
	player.Mu().RUnlock()

	result := &DungeonInfoResult{
		CurrentLayer: currentLayer,
		MaxLayer:     maxLayer,
	}

	// 如果玩家在地下城中，查询当前层数据
	if currentLayer > 0 {
		dungeon := s.world.GetDungeon(currentLayer)
		if dungeon != nil {
			dungeon.Mu().RLock()
			result.MonsterCount = int32(len(dungeon.Monsters))
			result.ResourceCount = int32(len(dungeon.Resources))
			result.PlayerCount = int32(len(dungeon.Players))

			// 统计存活怪物数
			aliveCount := int32(0)
			for _, m := range dungeon.Monsters {
				m.Mu().RLock()
				if !m.Dead {
					aliveCount++
				}
				m.Mu().RUnlock()
			}
			result.MonsterCount = aliveCount

			// 统计未采集资源数
			availableCount := int32(0)
			for _, r := range dungeon.Resources {
				r.Mu().RLock()
				if !r.Harvested {
					availableCount++
				}
				r.Mu().RUnlock()
			}
			result.ResourceCount = availableCount

			dungeon.Mu().RUnlock()
		}

		result.ZoneName = model.GetZoneByLayer(currentLayer)
	}

	return result, nil
}
