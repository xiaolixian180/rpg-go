// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、宠物探险结算等。
package scheduler

import (
	"context"
	"time"

	"hero-quest/internal/model"
	"hero-quest/internal/repo"
	"hero-quest/internal/service/iface"
	"hero-quest/pkg/logger"
)

// RegisterPetExploreTask 注册宠物探险结算定时任务。
// 每60秒检查所有在线玩家的宠物探险状态，对已到期的探险进行结算并发放奖励。
func RegisterPetExploreTask(s *Scheduler, world iface.World, petRepo repo.PetRepo, playerRepo repo.PlayerRepo, invRepo repo.InventoryRepo) {
	s.Add("pet_explore", 60*time.Second, func() {
		ctx := context.Background()
		now := time.Now()

		world.AllPlayers(func(playerID uint64, p *model.Player) {
			p.Mu().RLock()
			online := p.Online
			p.Mu().RUnlock()
			if !online {
				return
			}

			pets, err := petRepo.GetPetsByOwner(ctx, playerID)
			if err != nil || len(pets) == 0 {
				return
			}

			for _, pet := range pets {
				pet.Mu().Lock()
				exploring := pet.Exploring
				startTime := pet.ExploreStartTime
				endTime := pet.ExploreEndTime
				if exploring && now.After(endTime) {
					duration := endTime.Sub(startTime)
					if duration <= 0 {
						duration = time.Hour
					}
					reward := pet.CalcExploreReward(duration)

					pet.Exploring = false
					pet.ExploreStartTime = time.Time{}
					pet.ExploreEndTime = time.Time{}
					pet.Mu().Unlock()

					// 发放奖励
					p.Mu().Lock()
					p.Gold += reward.Gold
					p.Mu().Unlock()

					// 保存宠物和玩家
					if err := petRepo.SavePet(ctx, pet); err != nil {
						logger.Error("保存宠物探索结算失败", "player", playerID, "pet", pet.UID, "err", err)
					}
					if err := playerRepo.SavePlayer(ctx, p); err != nil {
						logger.Error("保存玩家探索奖励失败", "player", playerID, "err", err)
					}

					// 发放掉落物品
					if reward.DropItemID > 0 && reward.DropCount > 0 {
						if err := invRepo.AddItem(ctx, playerID, reward.DropItemID, reward.DropCount); err != nil {
							logger.Error("发放探索掉落物品失败", "player", playerID, "item", reward.DropItemID, "err", err)
						}
					}

					logger.Debug("宠物探索结算", "player", playerID, "pet", pet.UID, "gold", reward.Gold)
				} else {
					pet.Mu().Unlock()
				}
			}
		})
	})
}
