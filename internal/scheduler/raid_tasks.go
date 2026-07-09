package scheduler

import (
	"time"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service/iface"
	"hero-quest/internal/service/raid"
)

// RegisterRaidTimerTask 注册战局计时器定时任务。
// 每2秒调用 raidSvc.TickRaidTimers 处理撤离倒计时和战局超时，
// 并向战局内所有玩家推送剩余时间。
func RegisterRaidTimerTask(s *Scheduler, raidSvc raid.RaidService, hub *gateway.Hub, world iface.World) {
	s.Add("raid_timer", 2*time.Second, func() {
		// 委托给 service 层处理撤离倒计时和战局超时
		raidSvc.TickRaidTimers(world)

		// 推送剩余时间给所有战局内玩家
		raids := raidSvc.GetAllRaids()
		for _, ri := range raids {
			remaining := ri.RemainingSeconds()

			ri.Mu().RLock()
			playerIDs := make([]uint64, 0, len(ri.Players))
			for pid := range ri.Players {
				playerIDs = append(playerIDs, pid)
			}
			ri.Mu().RUnlock()

			for _, pid := range playerIDs {
				hub.SendToPlayer(pid, protocol.MsgIDRaidTimer, &protocol.S2CRaidTimer{
					Remaining: remaining,
				})
			}
		}
	})
}

// RegisterRaidMonsterAITask 注册战局怪物AI定时任务。
// 每1秒调用 raidSvc.TickRaidMonsterAI 处理战局内怪物的AI行为。
func RegisterRaidMonsterAITask(s *Scheduler, raidSvc raid.RaidService, world iface.World) {
	s.Add("raid_monster_ai", 1*time.Second, func() {
		raidSvc.TickRaidMonsterAI(world)
	})
}
