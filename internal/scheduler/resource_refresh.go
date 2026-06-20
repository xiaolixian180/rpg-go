// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、资源刷新、宠物探险结算等。
package scheduler

import (
	"time"

	"hero-quest/internal/protocol"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterResourceRefreshTask 注册资源刷新定时任务。
// 每60秒遍历所有副本层，将已采集的资源重置为可采集状态。
func RegisterResourceRefreshTask(s *Scheduler, world iface.World) {
	s.Add("resource_refresh", 60*time.Second, func() {
		for i := int32(1); i <= world.MaxLayer(); i++ {
			d := world.GetDungeon(i)
			if d == nil {
				continue
			}
			d.Mu().Lock()
			refreshed := make([]protocol.ResourceData, 0)
			for _, r := range d.Resources {
				r.Mu().Lock()
				if r.Harvested {
					r.Harvested = false
					refreshed = append(refreshed, protocol.ResourceData{
						ID: r.ID, Type: r.Type, Name: r.Name, X: r.X, Y: r.Y, Harvested: false,
					})
				}
				r.Mu().Unlock()
			}
			d.Mu().Unlock()
			if len(refreshed) > 0 {
				logger.Debug("资源已刷新", "layer", i, "count", len(refreshed))
			}
		}
	})
}
