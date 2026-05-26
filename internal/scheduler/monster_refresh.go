// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、宠物探险结算等。
package scheduler

import (
	"time"

	"hero-quest/internal/protocol"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterMonsterRefreshTask 注册怪物刷新定时任务。
// 每30秒遍历所有副本层，将已死亡的怪物按模板重置属性实现刷新。
func RegisterMonsterRefreshTask(s *Scheduler, world iface.World) {
	s.Add("monster_refresh", 30*time.Second, func() {
		// 遍历所有副本层，刷新已死亡的怪物
		for i := int32(1); i <= world.MaxLayer(); i++ {
			d := world.GetDungeon(i)
			if d == nil {
				continue
			}
			d.Mu().Lock()
			refreshed := make([]protocol.MonsterData, 0)
			for _, m := range d.Monsters {
				m.Mu().Lock()
				if m.Dead {
					// 使用怪物自身的 MaxHp 恢复，避免模板查找错误
					m.Hp = m.MaxHp
					m.Dead = false
					refreshed = append(refreshed, protocol.MonsterData{
						ID: m.ID, Name: m.Name, Hp: m.Hp, MaxHp: m.MaxHp, X: m.X, Y: m.Y,
					})
				}
				m.Mu().Unlock()
			}
			d.Mu().Unlock()
			if len(refreshed) > 0 {
				logger.Debug("怪物已刷新", "layer", i, "count", len(refreshed))
				world.Hub().BroadcastToPlayers(world.LayerPlayerIDs(i), protocol.MsgIDMonsterRefresh, &protocol.S2CMonsterRefresh{
					Monsters: refreshed,
				})
			}
		}
	})
}
