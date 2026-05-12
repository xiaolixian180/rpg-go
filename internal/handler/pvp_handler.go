package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// PvpHandler PvP模块消息处理器
// 负责处理PvP攻击、悬赏追杀和复仇的网络消息
type PvpHandler struct {
	pvpSvc        service.PvpService    // PvP服务接口
	playerSvc     service.PlayerService // 玩家服务接口（用于获取玩家数据）
	gm            *pvpConfig            // PvP配置参数
	enemiesLookup func(playerID uint64) []uint64 // 仇人列表查找回调（由 GameManager 提供）
}

// pvpConfig PvP配置参数，从 GameManager 配置中提取
type pvpConfig struct {
	pvpPenalty       float64 // PvP金币掠夺比例
	pvpHonor         int32   // PvP击杀荣誉值奖励
	redNameThreshold int32   // 红名杀戮值阈值
}

// NewPvpHandler 创建PvP模块处理器实例
func NewPvpHandler(pvpSvc service.PvpService, playerSvc service.PlayerService, pvpPenalty float64, pvpHonor int32, redNameThreshold int32, enemiesLookup func(playerID uint64) []uint64) *PvpHandler {
	return &PvpHandler{
		pvpSvc:        pvpSvc,
		playerSvc:     playerSvc,
		gm: &pvpConfig{
			pvpPenalty:       pvpPenalty,
			pvpHonor:         pvpHonor,
			redNameThreshold: redNameThreshold,
		},
		enemiesLookup: enemiesLookup,
	}
}

// HandlePvpAttack 处理PvP攻击请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（目标玩家ID+技能ID）
//  2. 获取攻击者和目标玩家数据
//  3. 调用 PvpService.Attack 执行PvP攻击逻辑
//  4. 通过 conn.Send 发送PvP结算结果
func (h *PvpHandler) HandlePvpAttack(conn *gateway.Conn, body []byte) {
	// 反序列化PvP攻击请求
	var req protocol.C2SPvpAttack
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，直接丢弃消息
		return
	}

	// 获取攻击者玩家数据
	attacker, ge := h.playerSvc.GetPlayer(context.Background(), conn.PlayerID)
	if ge != nil || attacker == nil {
		return
	}

	// 获取目标玩家数据
	target, ge := h.playerSvc.GetPlayer(context.Background(), req.TargetID)
	if ge != nil || target == nil {
		// 目标不存在，不发送错误响应（PvP攻击无独立错误码）
		return
	}

	// 调用PvP服务执行攻击逻辑
	pr, ge := h.pvpSvc.Attack(context.Background(), attacker, target, req.SkillID, h.gm.pvpPenalty, h.gm.pvpHonor, h.gm.redNameThreshold)
	if ge != nil {
		// PvP攻击失败，不发送错误响应
		_ = ge
		return
	}

	// PvP攻击成功，将 PvpResult 转换为协议层消息并发送
	conn.Send(protocol.MsgIDPvpResult, &protocol.S2CPvpResult{
		AttackerID: pr.AttackerID,
		TargetID:   pr.TargetID,
		Damage:     pr.Damage,
		GoldGain:   pr.GoldGain,
		HonorGain:  pr.HonorGain,
		IsDead:     pr.IsDead,
	})
}

// HandleBountyHunt 处理悬赏追杀请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（目标红名玩家ID）
//  2. 获取悬赏者和目标玩家数据
//  3. 调用 PvpService.BountyHunt 执行悬赏逻辑
//  4. 通过 conn.Send 发送悬赏奖励（金币和荣誉）
func (h *PvpHandler) HandleBountyHunt(conn *gateway.Conn, body []byte) {
	// 反序列化悬赏追杀请求
	var req protocol.C2SBountyHunt
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，直接丢弃消息
		return
	}

	// 获取悬赏者玩家数据
	hunter, ge := h.playerSvc.GetPlayer(context.Background(), conn.PlayerID)
	if ge != nil || hunter == nil {
		return
	}

	// 获取目标玩家数据
	target, ge := h.playerSvc.GetPlayer(context.Background(), req.TargetID)
	if ge != nil || target == nil {
		// 目标不存在
		return
	}

	// 调用PvP服务执行悬赏逻辑
	br, ge := h.pvpSvc.BountyHunt(context.Background(), hunter, target)
	if ge != nil {
		// 悬赏失败，不发送错误响应
		_ = ge
		return
	}

	// 悬赏成功，发送悬赏奖励
	conn.Send(protocol.MsgIDBountyReward, &protocol.S2CBountyReward{
		TargetID:  br.TargetID,
		GoldGain:  br.GoldGain,
		HonorGain: br.HonorGain,
	})
}

// HandleRevenge 处理复仇请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（仇人ID）
//  2. 构建仇人列表映射，调用 PvpService.Revenge 执行复仇校验逻辑
//  3. 通过 conn.Send 发送复仇响应
func (h *PvpHandler) HandleRevenge(conn *gateway.Conn, body []byte) {
	// 反序列化复仇请求
	var req protocol.C2SRevenge
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 获取仇人列表并构建映射供复仇服务校验
	enemies := h.enemiesLookup(conn.PlayerID)
	enemiesMap := map[uint64][]uint64{
		conn.PlayerID: enemies,
	}

	// 调用PvP服务执行复仇校验逻辑
	ge := h.pvpSvc.Revenge(context.Background(), conn.PlayerID, req.TargetID, enemiesMap)
	if ge != nil {
		conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{Code: ge.Code})
		return
	}

	// 复仇校验成功
	conn.Send(protocol.MsgIDRevengeResp, &protocol.S2CRevengeResp{
		Code:     errors.ErrSuccess.Code,
		TargetID: req.TargetID,
	})
}
