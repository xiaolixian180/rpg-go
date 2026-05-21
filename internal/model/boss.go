package model

import (
	"sync"
	"time"
)

// Boss 副本中的Boss怪物，每10层出现一次
type Boss struct {
	ID       uint64        // Boss唯一标识
	Name     string        // Boss名称
	Hp       int64         // 当前生命值
	MaxHp    int64         // 最大生命值
	Layer    int32         // 所在副本层数
	X        float64       // 地图中的X坐标
	Y        float64       // 地图中的Y坐标
	Cooldown time.Duration // 技能公共冷却时间
	Skills   []BossSkill   // Boss拥有的技能列表
	mu       sync.RWMutex  // 读写锁，保护并发访问
}

// Mu 返回 Boss 的读写锁指针，供外部按需加锁保护并发操作
func (b *Boss) Mu() *sync.RWMutex {
	return &b.mu
}

// BossSkill Boss技能定义
type BossSkill struct {
	SkillID int32   // 技能ID
	Name    string  // 技能名称
	CD      float64 // 技能冷却时间（秒）
	Range   float64 // 技能施放范围
	Damage  int64   // 技能造成的伤害值
}
