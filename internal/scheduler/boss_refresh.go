package scheduler

import (
	"time"

	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterBossRefreshTask 注册Boss刷新定时任务。
// 每60秒检查所有Boss层，如果Boss已被击杀且冷却时间已过，则重新生成Boss。
func RegisterBossRefreshTask(s *Scheduler, world iface.World) {
	s.Add("boss_refresh", 60*time.Second, func() {
		for i := int32(1); i <= world.MaxLayer(); i++ {
			if !world.GetDungeon(i).IsBoss {
				continue
			}
			newBoss := world.SpawnBossIfNeeded(i)
			if newBoss != nil {
				logger.Info("Boss定时刷新成功", "layer", i, "boss_id", newBoss.ID, "name", newBoss.Name)
				// 通知当前层所有玩家Boss出现
				world.Hub().BroadcastToPlayers(world.LayerPlayerIDs(i), 1203, map[string]interface{}{
					"boss_id": newBoss.ID,
					"name":    newBoss.Name,
					"hp":      newBoss.Hp,
					"max_hp":  newBoss.MaxHp,
					"layer":   newBoss.Layer,
					"x":       newBoss.X,
					"y":       newBoss.Y,
				})
			}
		}
	})
}
