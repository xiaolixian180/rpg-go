package handler

import (
	"encoding/json"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/chat"
	"hero-quest/internal/service/team"
	"hero-quest/pkg/errors"
)

// ChatHandler 聊天模块处理器
type ChatHandler struct {
	world   service.World
	chatSvc chat.ChatService
	teamSvc team.TeamService // 队伍频道需要查询队员ID
}

// NewChatHandler 创建聊天处理器实例
func NewChatHandler(world service.World, chatSvc chat.ChatService, teamSvc team.TeamService) *ChatHandler {
	return &ChatHandler{world: world, chatSvc: chatSvc, teamSvc: teamSvc}
}

// HandleChatSend 处理发送聊天消息
func (h *ChatHandler) HandleChatSend(conn *gateway.Conn, body []byte) {
	var req protocol.C2SChatSend
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
			Code: errors.ErrParamInvalid.Code, Channel: req.Channel,
		})
		return
	}

	sender := onlinePlayer(h.world, conn)
	if sender == nil {
		return
	}

	sender.Mu().RLock()
	senderName := sender.Name
	sender.Mu().RUnlock()

	ctx := connCtx(conn)
	var (
		msg *protocol.S2CChatMessage
		ge  *errors.GameError
	)
	switch req.Channel {
	case protocol.ChatChannelWorld:
		msg, ge = h.chatSvc.BuildWorldMessage(ctx, conn.PlayerID, senderName, req.Content)
	case protocol.ChatChannelPrivate:
		msg, ge = h.chatSvc.BuildPrivateMessage(ctx, conn.PlayerID, senderName, req.TargetID, req.Content)
	case protocol.ChatChannelTeam:
		// 队伍频道需校验是否在队中
		if ids := teamMemberIDs(h, conn.PlayerID); len(ids) == 0 {
			conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
				Code: errors.ErrChatNoTeam.Code, Channel: req.Channel,
			})
			return
		}
		msg, ge = h.chatSvc.BuildTeamMessage(ctx, conn.PlayerID, senderName, req.Content)
	default:
		conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
			Code: errors.ErrChatChannelInvalid.Code, Channel: req.Channel,
		})
		return
	}
	if ge != nil {
		conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
			Code: ge.Code, Channel: req.Channel, TargetID: req.TargetID,
		})
		return
	}

	// 分发到对应频道
	switch req.Channel {
	case protocol.ChatChannelWorld:
		h.chatSvc.AppendHistory(msg)
		h.world.Hub().Broadcast(protocol.MsgIDChatMessage, msg)
	case protocol.ChatChannelPrivate:
		// 校验目标在线
		if h.world.Hub().GetConnByPlayerID(req.TargetID) == nil {
			conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
				Code: errors.ErrChatTargetOff.Code, Channel: req.Channel, TargetID: req.TargetID,
			})
			return
		}
		// 推给对方 + 自己（自己也看到对话）
		_ = h.world.Hub().SendToPlayer(req.TargetID, protocol.MsgIDChatMessage, msg)
		_ = h.world.Hub().SendToPlayer(conn.PlayerID, protocol.MsgIDChatMessage, msg)
	case protocol.ChatChannelTeam:
		ids := teamMemberIDs(h, conn.PlayerID)
		h.world.Hub().BroadcastToPlayers(ids, protocol.MsgIDChatMessage, msg)
	}

	conn.Send(protocol.MsgIDChatSendResp, &protocol.S2CChatSendResp{
		Code:      errors.ErrSuccess.Code,
		Channel:   req.Channel,
		TargetID:  req.TargetID,
		Timestamp: msg.Timestamp,
	})
}

// HandleChatHistory 处理查询历史消息
func (h *ChatHandler) HandleChatHistory(conn *gateway.Conn, body []byte) {
	var req protocol.C2SChatHistory
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDChatHistoryResp, &protocol.S2CChatHistoryResp{
			Code: errors.ErrParamInvalid.Code,
		})
		return
	}
	if onlinePlayer(h.world, conn) == nil {
		return
	}
	if req.Channel != protocol.ChatChannelWorld {
		// 当前仅世界频道支持历史
		conn.Send(protocol.MsgIDChatHistoryResp, &protocol.S2CChatHistoryResp{
			Code: errors.ErrChatChannelInvalid.Code, Channel: req.Channel,
		})
		return
	}
	msgs := h.chatSvc.GetHistory(int(req.Count))
	if msgs == nil {
		msgs = []protocol.S2CChatMessage{}
	}
	conn.Send(protocol.MsgIDChatHistoryResp, &protocol.S2CChatHistoryResp{
		Code:     errors.ErrSuccess.Code,
		Channel:  req.Channel,
		Messages: msgs,
	})
}

// teamMemberIDs 队伍频道用：返回玩家所在队伍的全部队员ID
func teamMemberIDs(h *ChatHandler, playerID uint64) []uint64 {
	if h.teamSvc == nil {
		return nil
	}
	return h.teamSvc.MemberIDs(playerID)
}
