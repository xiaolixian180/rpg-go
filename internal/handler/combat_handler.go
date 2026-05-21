package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/eventbus"
	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/boss"
	"hero-quest/internal/service/combat"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// CombatHandler 战斗模块消息处理器
type CombatHandler struct {
	world     service.World        // 游戏世界（获取在线玩家和场景资源）
	combatSvc combat.CombatService // 战斗服务接口
	bossSvc   boss.BossService     // Boss服务（Boss死亡时处理掉落/通关/冷却）
	bus       *eventbus.Bus        // 事件总线（发布Boss死亡事件）
}

// NewCombatHandler 创建战斗模块处理器实例
func NewCombatHandler(world service.World, combatSvc combat.CombatService, bossSvc boss.BossService, bus *eventbus.Bus) *CombatHandler {
	return &CombatHandler{world: world, combatSvc: combatSvc, bossSvc: bossSvc, bus: bus}
}

// HandleAttack 处理普通攻击请求
func (h *CombatHandler) HandleAttack(conn *gateway.Conn, body []byte) {
	handleReq(h.world, conn, body, func(ctx context.Context, player *model.Player, req protocol.C2SAttack) {
		cr, ge := h.combatSvc.Attack(ctx, player, req.TargetID, req.SkillID)
		if ge != nil {
			return
		}

		conn.Send(protocol.MsgIDDamage, &protocol.S2CDamage{
			TargetID: cr.TargetID,
			Damage:   cr.Damage,
			CurrHp:   cr.CurrHp,
			IsDead:   cr.IsDead,
		})

		// Boss 死亡：处理掉落、通关进度、冷却、全服播报
		if cr.IsDead && cr.IsBoss {
			h.handleBossDie(conn, player, req.TargetID)
		}
	})
}

// HandleSkillCast 处理技能释放请求
func (h *CombatHandler) HandleSkillCast(conn *gateway.Conn, body []byte) {
	handleReq(h.world, conn, body, func(ctx context.Context, player *model.Player, req protocol.C2SSkillCast) {
		sr, ge := h.combatSvc.SkillCast(ctx, player, req.SkillID, req.TargetID, req.X, req.Y)
		if ge != nil {
			return
		}

		targets := make([]protocol.DamageInfo, len(sr.Targets))
		for i, t := range sr.Targets {
			targets[i] = protocol.DamageInfo{
				TargetID: t.TargetID, Damage: t.Damage, CurrHp: t.CurrHp, IsDead: t.IsDead,
			}
		}
		conn.Send(protocol.MsgIDSkillEffect, &protocol.S2CSkillEffect{
			CasterID: sr.CasterID, SkillID: sr.SkillID,
			X: sr.X, Y: sr.Y, Targets: targets,
		})
	})
}

// HandleCollectResource 处理采集资源请求
func (h *CombatHandler) HandleCollectResource(conn *gateway.Conn, body []byte) {
	var req protocol.C2SCollectResource
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	player := onlinePlayer(h.world, conn)
	if player == nil {
		return
	}

	// 通过 World 获取场景中的资源
	resource := h.lookupResource(player.Layer, req.ResourceID)
	if resource == nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
			Code: errors.ErrResourceGone.Code,
		})
		return
	}

	cr, ge := h.combatSvc.CollectResource(connCtx(conn), conn.PlayerID, resource)
	if ge != nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
		Code:       errors.ErrSuccess.Code,
		ResourceID: cr.ResourceID,
		ItemID:     cr.ItemID,
		ItemName:   cr.ItemName,
		Count:      cr.Count,
	})
}

// handleBossDie 处理Boss死亡：掉落、通关进度、冷却、全服播报
func (h *CombatHandler) handleBossDie(conn *gateway.Conn, player *model.Player, bossID uint64) {
	b := h.world.GetBoss(bossID)
	if b == nil {
		return
	}

	dieResult, err := h.bossSvc.OnDie(connCtx(conn), player, b)
	if err != nil {
		logger.Error("Boss死亡处理失败", "boss_id", bossID, "err", err)
	}
	h.world.RemoveBoss(b.ID)

	// 广播Boss死亡和掉落
	if dieResult != nil {
		drops := make([]protocol.DropItem, len(dieResult.Drops))
		for i, d := range dieResult.Drops {
			drops[i] = protocol.DropItem{
				ItemID: d.ItemID, Name: d.Name,
				Quality: d.Quality, Count: d.Count,
			}
		}
		h.world.Hub().Broadcast(protocol.MsgIDBossDie, &protocol.S2CBossDie{
			BossID: b.ID, Drops: drops,
		})
	}

	// 发布Boss死亡事件，由事件总线分发给订阅者（播报、排行榜等）
	player.Mu().RLock()
	playerName := player.Name
	playerID := player.ID
	player.Mu().RUnlock()
	h.bus.Publish(eventbus.TopicBossDie, &eventbus.BossDieEvent{
		BossID:     b.ID,
		BossName:   b.Name,
		Layer:      b.Layer,
		KillerID:   playerID,
		KillerName: playerName,
	})
}

// lookupResource 在指定层的资源表中查找资源
func (h *CombatHandler) lookupResource(layer int32, resourceID uint64) *model.Resource {
	dungeon := h.world.GetDungeon(layer)
	if dungeon == nil {
		return nil
	}
	dungeon.Mu().RLock()
	defer dungeon.Mu().RUnlock()
	return dungeon.Resources[resourceID]
}
