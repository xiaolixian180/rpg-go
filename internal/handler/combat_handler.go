package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/model"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// CombatHandler 战斗模块消息处理器
// 负责处理普通攻击、技能释放和资源采集的网络消息
type CombatHandler struct {
	combatSvc       service.CombatService  // 战斗服务接口
	playerSvc       service.PlayerService  // 玩家服务接口（用于获取玩家数据）
	resourceLookup  func(resourceID uint64) *model.Resource // 资源查找回调（由 GameManager 提供）
}

// NewCombatHandler 创建战斗模块处理器实例
func NewCombatHandler(combatSvc service.CombatService, playerSvc service.PlayerService, resourceLookup func(resourceID uint64) *model.Resource) *CombatHandler {
	return &CombatHandler{combatSvc: combatSvc, playerSvc: playerSvc, resourceLookup: resourceLookup}
}

// HandleAttack 处理普通攻击请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（目标ID+技能ID）
//  2. 获取玩家数据，调用 CombatService.Attack 执行攻击逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送伤害结算结果
func (h *CombatHandler) HandleAttack(conn *gateway.Conn, body []byte) {
	// 反序列化攻击请求
	var req protocol.C2SAttack
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，直接丢弃消息（攻击无独立错误响应）
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用战斗服务执行攻击逻辑
	cr, ge := h.combatSvc.Attack(context.Background(), player, req.TargetID, req.SkillID)
	if ge != nil {
		// 攻击失败，记录日志（战斗场景下不发送错误码，仅不发送伤害反馈）
		_ = ge
		return
	}

	// 攻击成功，将 CombatResult 转换为协议层伤害结算消息并发送
	conn.Send(protocol.MsgIDDamage, &protocol.S2CDamage{
		TargetID: cr.TargetID,
		Damage:   cr.Damage,
		CurrHp:   cr.CurrHp,
		IsDead:   cr.IsDead,
	})
}

// HandleSkillCast 处理技能释放请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（技能ID+目标ID+坐标）
//  2. 获取玩家数据，调用 CombatService.SkillCast 执行技能释放逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送技能效果数据
func (h *CombatHandler) HandleSkillCast(conn *gateway.Conn, body []byte) {
	// 反序列化技能释放请求
	var req protocol.C2SSkillCast
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，直接丢弃消息
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 调用战斗服务执行技能释放逻辑
	sr, ge := h.combatSvc.SkillCast(context.Background(), player, req.SkillID, req.TargetID, req.X, req.Y)
	if ge != nil {
		// 技能释放失败，不发送错误响应
		_ = ge
		return
	}

	// 技能释放成功，将 SkillResult 转换为协议层技能效果消息并发送
	targets := make([]protocol.DamageInfo, len(sr.Targets))
	for i, t := range sr.Targets {
		targets[i] = protocol.DamageInfo{
			TargetID: t.TargetID,
			Damage:   t.Damage,
			CurrHp:   t.CurrHp,
			IsDead:   t.IsDead,
		}
	}
	conn.Send(protocol.MsgIDSkillEffect, &protocol.S2CSkillEffect{
		CasterID: sr.CasterID,
		SkillID:  sr.SkillID,
		X:        sr.X,
		Y:        sr.Y,
		Targets:  targets,
	})
}

// HandleCollectResource 处理采集资源请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（资源ID）
//  2. 通过资源查找回调获取 *model.Resource 对象
//  3. 调用 CombatService.CollectResource 执行采集逻辑
//  4. 根据 service 返回的 GameError 设置响应的 Code 字段
//  5. 通过 conn.Send 发送采集结果（获得的物品和数量）
func (h *CombatHandler) HandleCollectResource(conn *gateway.Conn, body []byte) {
	// 反序列化采集资源请求
	var req protocol.C2SCollectResource
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取玩家数据
	player := getPlayer(conn, h.playerSvc)
	if player == nil {
		return
	}

	// 通过资源查找回调获取场景中的资源对象
	resource := h.resourceLookup(req.ResourceID)
	if resource == nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
			Code: errors.ErrResourceGone.Code,
		})
		return
	}

	// 调用战斗服务执行采集逻辑（传入 playerID 和 *model.Resource）
	cr, ge := h.combatSvc.CollectResource(context.Background(), conn.PlayerID, resource)
	if ge != nil {
		conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{Code: ge.Code})
		return
	}

	// 采集成功，发送采集结果
	conn.Send(protocol.MsgIDCollectResult, &protocol.S2CCollectResult{
		Code:       errors.ErrSuccess.Code,
		ResourceID: cr.ResourceID,
		ItemID:     cr.ItemID,
		ItemName:   cr.ItemName,
		Count:      cr.Count,
	})
}
