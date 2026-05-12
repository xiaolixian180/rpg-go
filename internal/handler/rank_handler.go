package handler

import (
	"context"
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/pkg/errors"
)

// RankHandler 排行榜模块消息处理器
// 负责处理排行榜查询的网络消息
type RankHandler struct {
	rankSvc service.RankService // 排行榜服务接口
}

// NewRankHandler 创建排行榜模块处理器实例
func NewRankHandler(rankSvc service.RankService) *RankHandler {
	return &RankHandler{rankSvc: rankSvc}
}

// HandleRankingList 处理排行榜查询请求
// 流程：
//  1. 反序列化请求体（排行榜类型：等级/战力/荣誉）
//  2. 调用 RankService.GetRanking 执行查询逻辑
//  3. 根据 service 返回的 GameError 设置响应的 Code 字段
//  4. 通过 conn.Send 发送排行数据列表
func (h *RankHandler) HandleRankingList(conn *gateway.Conn, body []byte) {
	// 反序列化排行榜查询请求
	var req protocol.C2SRankingList
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDRankingListResp, &protocol.S2CRankingListResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}

	// 调用排行榜服务查询排行数据
	items, ge := h.rankSvc.GetRanking(context.Background(), req.Type)
	if ge != nil {
		conn.Send(protocol.MsgIDRankingListResp, &protocol.S2CRankingListResp{Code: ge.Code})
		return
	}

	// 查询成功，将 model.RankingItem 列表转换为协议层响应并发送
	conn.Send(protocol.MsgIDRankingListResp, &protocol.S2CRankingListResp{
		Code:     errors.ErrSuccess.Code,
		Type:     req.Type,
		Rankings: toRankingItemList(items),
	})
}