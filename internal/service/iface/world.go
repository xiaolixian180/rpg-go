// Package iface 定义 service 层的核心接口和配置类型。
// 独立为子包，避免 service 各子域之间的循环依赖：
// 各子域（combat/player/boss 等）只依赖 iface.World，
// 不依赖 service 根包的 GameManager 实现。
package iface

import (
	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
)

// World 定义游戏世界状态访问接口。
// handler 层通过此接口获取在线玩家、场景实体和消息中心，
// 不直接依赖 GameManager 具体实现，便于测试和解耦。
type World interface {
	// GetOnlinePlayer 获取内存中的在线玩家，不存在返回 nil
	GetOnlinePlayer(playerID uint64) *model.Player
	// GetDungeon 获取指定层的地下城实例
	GetDungeon(layer int32) *model.DungeonLayer
	// LayerPlayerIDs 获取指定层当前玩家ID快照
	LayerPlayerIDs(layer int32) []uint64
	// GetBoss 获取指定Boss实例
	GetBoss(bossID uint64) *model.Boss
	// AddBoss 添加Boss实例到世界
	AddBoss(boss *model.Boss)
	// RemoveBoss 从世界中移除Boss实例
	RemoveBoss(bossID uint64)
	// GetLayerBoss 获取指定层的Boss实例（如果存在）
	GetLayerBoss(layer int32) *model.Boss
	// SpawnBossIfNeeded 检查Boss层是否需要重新生成Boss，返回新生成的Boss（已存在则返回nil）
	SpawnBossIfNeeded(layer int32) *model.Boss
	// Hub 获取网关消息中心，用于广播和私聊
	Hub() *gateway.Hub
	// OnLogin 处理玩家登录（加载到内存）
	OnLogin(playerID uint64) (*model.Player, error)
	// OnLogout 处理玩家登出（从内存移除并持久化）
	OnLogout(playerID uint64)
	// MaxLayer 返回地下城最大层数
	MaxLayer() int32
	// AllPlayers 遍历所有在线玩家（只读回调）
	AllPlayers(fn func(playerID uint64, p *model.Player))
	// Config 返回游戏配置参数
	Config() GameConfig
}

// GameConfig 游戏配置参数，供 service 和 handler 层使用
type GameConfig struct {
	MaxLayer          int32
	PvpGoldPenalty    float64
	PvpHonorGain      int32
	RedNameThreshold  int32
	SkillResetCost    int64
	TeleportCost      int64
	SkillMultiplier   float64
	InvincibleSec     int
	PetLevelUpCost    int64
	BountyGoldPerKill int64
	BountyHonorGain   int32
	BossDropPurple    float64
	BossDropOrange    float64
	PageSize          int32
}
