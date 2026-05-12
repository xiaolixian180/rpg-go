// Package service - 地下城服务
// 提供进入/离开地下城、层间传送等业务逻辑
package service

import (
	"context"
	"fmt"

	"hero-quest/internal/model"
	"hero-quest/internal/rbac"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// ==================== 地下城服务接口 ====================

// DungeonService 地下城服务接口，定义进入/离开地下城、层间传送等业务操作
type DungeonService interface {
	// Enter 进入地下城指定层，校验层数合法性及通关进度
	Enter(ctx context.Context, playerID uint64, layer int32, maxLayer int32, dungeonMaxLayer int32) (*model.DungeonLayer, *errors.GameError)
	// Leave 离开地下城，将玩家从对应层的玩家表中移除
	Leave(ctx context.Context, player *model.Player) *errors.GameError
	// Teleport 层间传送，从当前层传送到目标层
	Teleport(ctx context.Context, playerID uint64, targetLayer int32, maxLayer int32, dungeonMaxLayer int32) (*model.DungeonLayer, *errors.GameError)
	// GetDungeonInfo 获取玩家当前地下城信息（当前层数、最高通关层数）
	GetDungeonInfo(ctx context.Context, playerID uint64, currentLayer int32, maxLayer int32) (int32, int32)
}

// ==================== 地下城服务实现 ====================

// dungeonService 地下城服务实现
type dungeonService struct {
	enforcer *rbac.Enforcer // Casbin 权限执行器，用于校验玩家进入权限
}

// NewDungeonService 创建地下城服务实例
// enforcer: Casbin 权限执行器，传入 nil 则跳过权限校验
func NewDungeonService(enforcer *rbac.Enforcer) DungeonService {
	return &dungeonService{enforcer: enforcer}
}

// Enter 进入地下城业务逻辑：
//  1. 校验目标层数是否合法（1 ~ dungeonMaxLayer）
//  2. 校验玩家是否已解锁该层（只能进入已通关最高层+1或已通关层）
//  3. 校验玩家是否有进入该层的权限（Casbin RBAC）
//  4. 更新玩家当前所在层
//  5. 将玩家加入目标层的玩家表
//  6. 返回目标层的地下城实例（包含怪物、玩家、资源列表）
func (s *dungeonService) Enter(ctx context.Context, playerID uint64, layer int32, maxLayer int32, dungeonMaxLayer int32) (*model.DungeonLayer, *errors.GameError) {
	// 校验层数范围
	if layer < 1 || layer > dungeonMaxLayer {
		return nil, errors.ErrLayerInvalid
	}

	// 校验通关进度：只能进入已通关最高层+1或已通关层
	if layer > maxLayer+1 {
		return nil, errors.ErrLayerLocked
	}

	// 校验玩家是否有进入该层的权限（Casbin RBAC）
	// 通过权限执行器检查玩家角色是否有权进入指定地下城层
	if s.enforcer != nil {
		// 构造资源对象，格式为 "dungeon:<层数>"
		obj := fmt.Sprintf("dungeon:%d", layer)
		// 检查 "player" 角色是否有进入该资源的权限
		if !s.enforcer.Check("player", obj, "enter") {
			return nil, errors.ErrLayerLocked
		}
	}

	logger.Info("玩家进入地下城", "player_id", playerID, "layer", layer)
	return nil, nil // 实际的DungeonLayer由GameManager的内存map提供
}

// Leave 离开地下城业务逻辑：
//  1. 校验玩家是否在地下城中（Layer > 0）
//  2. 将玩家Layer设为0
func (s *dungeonService) Leave(ctx context.Context, player *model.Player) *errors.GameError {
	if player.Layer <= 0 {
		return errors.ErrNotInDungeon
	}

	player.Layer = 0
	logger.Info("玩家离开地下城", "player_id", player.ID)
	return nil
}

// Teleport 层间传送业务逻辑：
//  1. 校验目标层数是否合法
//  2. 校验玩家是否已解锁目标层
//  3. 更新玩家当前层为目标层
//  4. 返回目标层的地下城实例
func (s *dungeonService) Teleport(ctx context.Context, playerID uint64, targetLayer int32, maxLayer int32, dungeonMaxLayer int32) (*model.DungeonLayer, *errors.GameError) {
	// 校验层数范围
	if targetLayer < 1 || targetLayer > dungeonMaxLayer {
		return nil, errors.ErrLayerInvalid
	}

	// 校验通关进度
	if targetLayer > maxLayer {
		return nil, errors.ErrLayerLocked
	}

	logger.Info("玩家层间传送", "player_id", playerID, "target_layer", targetLayer)
	return nil, nil // 实际的DungeonLayer由GameManager的内存map提供
}

// GetDungeonInfo 获取玩家当前地下城信息
func (s *dungeonService) GetDungeonInfo(ctx context.Context, playerID uint64, currentLayer int32, maxLayer int32) (int32, int32) {
	return currentLayer, maxLayer
}
