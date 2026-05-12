// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、宠物探险结算等。
package scheduler

import (
	"time"

	"hero-quest/pkg/logger"
)

// RegisterMonsterRefreshTask 注册怪物刷新定时任务。
// 按照指定间隔检查并刷新地下城中的怪物，
// 当前为占位实现，仅打印日志，后续对接怪物刷新逻辑。
func RegisterMonsterRefreshTask(s *Scheduler) {
	s.Add("monster_refresh", 30*time.Second, func() {
		// TODO: 实现怪物刷新逻辑
		// 1. 遍历所有活跃的地下城实例
		// 2. 检查怪物刷新条件（时间/击杀数等）
		// 3. 生成新怪物并通过 Hub 广播刷新通知
		logger.Info("[monster_refresh] 怪物刷新检查执行中（占位实现）")
	})
}
