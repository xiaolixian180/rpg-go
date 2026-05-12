package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// PetHandler 宠物模块消息处理器
// 负责处理宠物召唤、收回、升级、进阶、探险和合成的网络消息
type PetHandler struct {
	petSvc service.PetService // 宠物服务接口
}

// NewPetHandler 创建宠物模块处理器实例
func NewPetHandler(petSvc service.PetService) *PetHandler {
	return &PetHandler{petSvc: petSvc}
}

// HandleSummon 处理宠物召唤请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（宠物实例UID）
//  2. 调用 PetService.Summon 执行召唤逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送召唤结果（宠物数据）
func (h *PetHandler) HandleSummon(conn *gateway.Conn, body []byte) {
	// 反序列化宠物召唤请求
	var req protocol.C2SPetSummon
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPetSummonResp, &protocol.S2CPetSummonResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用宠物服务执行召唤逻辑
	pet, ge := h.petSvc.Summon(context.Background(), conn.PlayerID, req.PetUID)
	if ge != nil {
		conn.Send(protocol.MsgIDPetSummonResp, &protocol.S2CPetSummonResp{Code: ge.Code})
		return
	}

	// 召唤成功，将 model.Pet 转换为协议层 PetData 并发送
	conn.Send(protocol.MsgIDPetSummonResp, &protocol.S2CPetSummonResp{
		Code: errors.ErrSuccess.Code,
		Pet:  *toPetData(pet),
	})
}

// HandleRecall 处理宠物收回请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（宠物实例UID）
//  2. 调用 PetService.Recall 执行收回逻辑
//  3. 收回成功无独立响应体，直接发送成功
func (h *PetHandler) HandleRecall(conn *gateway.Conn, body []byte) {
	// 反序列化宠物收回请求
	var req protocol.C2SPetRecall
	if err := json.Unmarshal(body, &req); err != nil {
		// 反序列化失败，记录日志
		return
	}

	// 调用宠物服务执行收回逻辑
	ge := h.petSvc.Recall(context.Background(), conn.PlayerID, req.PetUID)
	if ge != nil {
		// 收回失败，不发送错误响应
		_ = ge
		return
	}

	// 收回成功（协议中无 PetRecall 响应结构体和消息ID，此处静默处理）
}

// HandleLevelUp 处理宠物升级请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（宠物实例UID）
//  2. 调用 PetService.LevelUp 执行升级逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送升级结果（新等级）
func (h *PetHandler) HandleLevelUp(conn *gateway.Conn, body []byte) {
	// 反序列化宠物升级请求
	var req protocol.C2SPetLevelUp
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPetLevelUp, &protocol.S2CPetLevelUp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用宠物服务执行升级逻辑（goldCost=100，升级费用由调用方决定）
	lr, ge := h.petSvc.LevelUp(context.Background(), conn.PlayerID, req.PetUID, 100)
	if ge != nil {
		conn.Send(protocol.MsgIDPetLevelUp, &protocol.S2CPetLevelUp{Code: ge.Code})
		return
	}

	// 升级成功，发送新等级
	conn.Send(protocol.MsgIDPetLevelUp, &protocol.S2CPetLevelUp{
		Code:   errors.ErrSuccess.Code,
		PetUID: lr.PetUID,
		Level:  lr.Level,
	})
}

// HandleEvolve 处理宠物进阶请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（宠物实例UID）
//  2. 调用 PetService.Evolve 执行进阶逻辑（需等级>=10）
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送进阶结果（新模板ID和品质）
func (h *PetHandler) HandleEvolve(conn *gateway.Conn, body []byte) {
	// 反序列化宠物进阶请求
	var req protocol.C2SPetEvolve
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPetEvolveResp, &protocol.S2CPetEvolveResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用宠物服务执行进阶逻辑
	er, ge := h.petSvc.Evolve(context.Background(), conn.PlayerID, req.PetUID)
	if ge != nil {
		conn.Send(protocol.MsgIDPetEvolveResp, &protocol.S2CPetEvolveResp{Code: ge.Code})
		return
	}

	// 进阶成功，发送新模板ID和品质
	conn.Send(protocol.MsgIDPetEvolveResp, &protocol.S2CPetEvolveResp{
		Code:       errors.ErrSuccess.Code,
		PetUID:     er.PetUID,
		NewPetID:   er.NewPetID,
		NewQuality: er.NewQuality,
	})
}

// HandleExplore 处理宠物探险请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（宠物实例UID+探险时长）
//  2. 调用 PetService.Explore 执行探险派遣逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送探险结果（结束时间）
func (h *PetHandler) HandleExplore(conn *gateway.Conn, body []byte) {
	// 反序列化宠物探险请求
	var req protocol.C2SPetExplore
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPetExploreResp, &protocol.S2CPetExploreResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用宠物服务执行探险派遣逻辑
	er, ge := h.petSvc.Explore(context.Background(), conn.PlayerID, req.PetUID, req.Duration)
	if ge != nil {
		conn.Send(protocol.MsgIDPetExploreResp, &protocol.S2CPetExploreResp{Code: ge.Code})
		return
	}

	// 探险派遣成功，发送结束时间
	conn.Send(protocol.MsgIDPetExploreResp, &protocol.S2CPetExploreResp{
		Code:    errors.ErrSuccess.Code,
		PetUID:  er.PetUID,
		EndTime: er.EndTime,
	})
}

// HandleCompose 处理宠物合成请求
// 流程：
//  1. 从连接中获取玩家ID，反序列化请求体（3只宠物实例UID列表）
//  2. 调用 PetService.Compose 执行合成逻辑（3只同品质合成升阶）
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送合成结果（新宠物ID和品质）
func (h *PetHandler) HandleCompose(conn *gateway.Conn, body []byte) {
	// 反序列化宠物合成请求
	var req protocol.C2SPetCompose
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDPetComposeResp, &protocol.S2CPetComposeResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用宠物服务执行合成逻辑
	cr, ge := h.petSvc.Compose(context.Background(), conn.PlayerID, req.PetUIDs)
	if ge != nil {
		conn.Send(protocol.MsgIDPetComposeResp, &protocol.S2CPetComposeResp{Code: ge.Code})
		return
	}

	// 合成成功，发送新宠物ID和品质
	conn.Send(protocol.MsgIDPetComposeResp, &protocol.S2CPetComposeResp{
		Code:     errors.ErrSuccess.Code,
		ResultID: cr.ResultID,
		PetID:    cr.PetID,
		Quality:  cr.Quality,
	})
}
