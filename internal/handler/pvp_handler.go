package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/pvp"
	"hero-quest/pkg/errors"
)

// PvpHandler PvP模块消息处理器
type PvpHandler struct {
	world  service.World  // 游戏世界（获取在线玩家和配置）
	pvpSvc pvp.PvpService // PvP服务接口
}

// NewPvpHandler 创建PvP模块处理器实例
func NewPvpHandler(world service.World, pvpSvc pvp.PvpService) *PvpHandler {
	return &PvpHandler{world: world, pvpSvc: pvpSvc}
}

// HandlePvpAttack 处理PvP攻击请求
func (h *PvpHandler) HandlePvpAttack(conn *gateway.Conn, body []byte) {
	var req protocol.C2SPvpAttack
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	attacker := h.world.GetOnlinePlayer(conn.PlayerID)
	target := h.world.GetOnlinePlayer(req.TargetID)
	if attacker == nil || target == nil {
		conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
			Code: errors.ErrTargetNotFound.Code,
		})
		return
	}

	cfg := h.world.Config()
	pr, ge := h.pvpSvc.Attack(connCtx(conn), attacker, target, req.SkillID, cfg.PvpGoldPenalty, cfg.PvpHonorGain, cfg.RedNameThreshold)
	if ge != nil {
		conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
		AttackerID: pr.AttackerID, TargetID: pr.TargetID,
		Damage: pr.Damage, GoldGain: pr.GoldGain, HonorGain: pr.HonorGain, IsDead: pr.IsDead,
	})

	if pr.IsRedName {
		h.world.Hub().Broadcast(protocol.MsgIDBroadcast, &protocol.S2CBroadcast{
			Type: 2, Content: pr.RedMsg,
		})
	}
}

// HandleBountyHunt 处理悬赏追杀请求
func (h *PvpHandler) HandleBountyHunt(conn *gateway.Conn, body []byte) {
	var req protocol.C2SBountyHunt
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	hunter := h.world.GetOnlinePlayer(conn.PlayerID)
	target := h.world.GetOnlinePlayer(req.TargetID)
	if hunter == nil || target == nil {
		conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{
			Code: errors.ErrTargetNotFound.Code,
		})
		return
	}

	br, ge := h.pvpSvc.BountyHunt(connCtx(conn), hunter, target, h.world.Config().RedNameThreshold)
	if ge != nil {
		conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{
		TargetID: br.TargetID, GoldGain: br.GoldGain, HonorGain: br.HonorGain,
	})
}

// HandleRevenge 处理复仇请求
func (h *PvpHandler) HandleRevenge(conn *gateway.Conn, body []byte) {
	var req protocol.C2SRevenge
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取攻击者和目标在线实例
	attacker := h.world.GetOnlinePlayer(conn.PlayerID)
	target := h.world.GetOnlinePlayer(req.TargetID)
	if attacker == nil || target == nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{Code: errors.ErrTargetNotFound.Code})
		return
	}

	// 执行复仇攻击（使用普攻，skillID=0）
	pr, ge := h.pvpSvc.Revenge(connCtx(conn), attacker, target, 0)
	if ge != nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{
		Code: errors.ErrSuccess.Code, TargetID: req.TargetID,
	})

	// 如果目标死亡，通知攻击者和目标
	if pr.IsDead {
		conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
			AttackerID: pr.AttackerID, TargetID: pr.TargetID,
			Damage: pr.Damage, GoldGain: pr.GoldGain, HonorGain: pr.HonorGain, IsDead: pr.IsDead,
		})
	}
}
