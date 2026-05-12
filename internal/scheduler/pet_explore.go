// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、宠物探险结算等。
package scheduler

import (
	"time"

	"hero-quest/pkg/logger"
)

// RegisterPetExploreTask 注册宠物探险结算定时任务。
// 按照指定间隔检查已到期的宠物探险并发放奖励，
// 当前为占位实现，仅打印日志，后续对接探险结算逻辑。
func RegisterPetExploreTask(s *Scheduler) {
	s.Add("pet_explore", 60*time.Second, func() {
		// TODO: 实现宠物探险结算逻辑
		// 1. 查询所有已到期的探险记录
		// 2. 根据探险时长和宠物品质计算奖励
		// 3. 发放奖励到玩家背包
		// 4. 推送探险完成通知给在线玩家
		logger.Info("[pet_explore] 宠物探险结算检查执行中（占位实现）")
	})
}
