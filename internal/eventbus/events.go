package eventbus

// Topic 常量，所有事件主题统一管理
const (
	TopicBossDie     = "boss.die"     // Boss被击杀
	TopicPlayerDie   = "player.die"   // 玩家死亡（PvP/PvE）
	TopicPlayerLogin = "player.login" // 玩家登录
)

// BossDieEvent Boss被击杀事件
type BossDieEvent struct {
	BossID     uint64
	BossName   string
	Layer      int32
	KillerID   uint64
	KillerName string
}

func (e *BossDieEvent) Topic() string { return TopicBossDie }

// PlayerDieEvent 玩家死亡事件
type PlayerDieEvent struct {
	PlayerID uint64
	KillerID uint64
	IsPvP    bool
}

func (e *PlayerDieEvent) Topic() string { return TopicPlayerDie }

// PlayerLoginEvent 玩家登录事件
type PlayerLoginEvent struct {
	PlayerID uint64
	Name     string
	Level    int32
}

func (e *PlayerLoginEvent) Topic() string { return TopicPlayerLogin }
