package handler

import (
	"encoding/json"
	"fmt"

	"hero-quest/internal/gateway"
	"hero-quest/internal/protocol"
	"hero-quest/internal/service"
	"hero-quest/internal/service/team"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// TeamHandler 组队模块处理器
type TeamHandler struct {
	world   service.World
	teamSvc team.TeamService
}

// NewTeamHandler 创建组队处理器实例
func NewTeamHandler(world service.World, teamSvc team.TeamService) *TeamHandler {
	return &TeamHandler{world: world, teamSvc: teamSvc}
}

// snapshotMembers 将一组玩家ID解析为 TeamMember 协议结构
// 离线玩家也保留在列表中（Online=false），便于客户端展示状态。
func (h *TeamHandler) snapshotMembers(members []uint64, leaderID uint64) []protocol.TeamMember {
	out := make([]protocol.TeamMember, 0, len(members))
	for _, pid := range members {
		m := protocol.TeamMember{
			PlayerID: pid,
			IsLeader: pid == leaderID,
			Online:   false,
		}
		if p := h.world.GetOnlinePlayer(pid); p != nil {
			p.Mu().RLock()
			m.Name = p.Name
			m.Class = p.Class
			m.Level = p.Level
			m.Hp = p.Hp
			m.MaxHp = p.MaxHp
			m.Online = p.Online
			m.Layer = p.Layer
			p.Mu().RUnlock()
		} else {
			m.Name = fmt.Sprintf("玩家#%d", pid)
		}
		out = append(out, m)
	}
	return out
}

// buildTeamInfo 由内部 Team 构造可下发的 TeamInfo
func (h *TeamHandler) buildTeamInfo(t *team.Team) protocol.TeamInfo {
	if t == nil {
		return protocol.TeamInfo{}
	}
	return protocol.TeamInfo{
		TeamID:      t.ID,
		LeaderID:    t.LeaderID,
		MemberCount: int32(len(t.Members)),
		Members:     h.snapshotMembers(t.Members, t.LeaderID),
	}
}

// broadcastUpdate 向队伍全员推送状态变更
// action: 1=加入 2=离开 3=解散 4=踢出
func (h *TeamHandler) broadcastUpdate(t *team.Team, action uint32, reason string) {
	if t == nil {
		return
	}
	upd := &protocol.S2CTeamUpdate{
		Action:   action,
		TeamID:   t.ID,
		LeaderID: t.LeaderID,
		Members:  h.snapshotMembers(t.Members, t.LeaderID),
		Reason:   reason,
	}
	h.world.Hub().BroadcastToPlayers(t.Members, protocol.MsgIDTeamUpdate, upd)
}

// HandleTeamCreate 处理创建队伍
func (h *TeamHandler) HandleTeamCreate(conn *gateway.Conn, body []byte) {
	if onlinePlayer(h.world, conn) == nil {
		return
	}
	t, ge := h.teamSvc.Create(connCtx(conn), conn.PlayerID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{Code: ge.Code})
		return
	}
	conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{
		Code: errors.ErrSuccess.Code,
		Team: h.buildTeamInfo(t),
	})
}

// HandleTeamInvite 处理邀请请求
func (h *TeamHandler) HandleTeamInvite(conn *gateway.Conn, body []byte) {
	var req protocol.C2STeamInvite
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrParamInvalid.Code, TargetID: req.TargetID,
		})
		return
	}
	inviter := onlinePlayer(h.world, conn)
	if inviter == nil {
		return
	}
	if req.TargetID == conn.PlayerID {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrTeamInviteSelf.Code, TargetID: req.TargetID,
		})
		return
	}

	// 邀请者必须在队伍中且是队长
	t, ge := h.teamSvc.Query(connCtx(conn), conn.PlayerID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: ge.Code, TargetID: req.TargetID,
		})
		return
	}
	if t.LeaderID != conn.PlayerID {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrNotTeamLeader.Code, TargetID: req.TargetID,
		})
		return
	}
	if len(t.Members) >= team.MaxTeamMembers {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrTeamFull.Code, TargetID: req.TargetID,
		})
		return
	}

	// 校验目标在线 & 没在队伍中
	target := h.world.GetOnlinePlayer(req.TargetID)
	if target == nil {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrTeamInviteOff.Code, TargetID: req.TargetID,
		})
		return
	}
	if ids := h.teamSvc.MemberIDs(req.TargetID); len(ids) > 0 {
		conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
			Code: errors.ErrTargetInTeam.Code, TargetID: req.TargetID,
		})
		return
	}

	// 推送邀请通知给目标
	target.Mu().RLock()
	targetName := target.Name
	target.Mu().RUnlock()
	_ = h.world.Hub().SendToPlayer(req.TargetID, protocol.MsgIDTeamInvitePush, &protocol.S2CTeamInvitePush{
		TeamID:      t.ID,
		InviterID:   conn.PlayerID,
		InviterName: inviter.Name,
		MemberCount: int32(len(t.Members)),
	})
	logger.TInfo(connCtx(conn), "发送组队邀请", "inviter", conn.PlayerID, "target", req.TargetID, "team_id", t.ID)

	// 同时给邀请者一个 ACK（等待对方响应）
	conn.Send(protocol.MsgIDTeamInviteResult, &protocol.S2CTeamInviteResult{
		Code:       0,
		TargetID:   req.TargetID,
		TargetName: targetName,
		Accept:     false, // 表示邀请已发出，等待回复
	})
}

// HandleTeamInviteReply 处理被邀请方的回复
func (h *TeamHandler) HandleTeamInviteReply(conn *gateway.Conn, body []byte) {
	var req protocol.C2STeamInviteReply
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{Code: errors.ErrParamInvalid.Code})
		return
	}
	if onlinePlayer(h.world, conn) == nil {
		return
	}

	replier := h.world.GetOnlinePlayer(conn.PlayerID)
	if replier == nil {
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{Code: errors.ErrNotLogin.Code})
		return
	}

	if !req.Accept {
		// 拒绝：给邀请者一个通知（如有在线）。这里不知道邀请者ID，留作未来按team_id+player_id扩展
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{
			Code: errors.ErrSuccess.Code,
		})
		return
	}

	// 接受：加入队伍
	t, ge := h.teamSvc.AcceptInvite(connCtx(conn), conn.PlayerID, req.TeamID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{
		Code: errors.ErrSuccess.Code,
		Team: h.buildTeamInfo(t),
	})
	h.broadcastUpdate(t, 1, fmt.Sprintf("%s 加入了队伍", replier.Name))
}

// HandleTeamLeave 处理离开队伍
func (h *TeamHandler) HandleTeamLeave(conn *gateway.Conn, body []byte) {
	if onlinePlayer(h.world, conn) == nil {
		return
	}
	leaver := h.world.GetOnlinePlayer(conn.PlayerID)
	leaverName := ""
	if leaver != nil {
		leaver.Mu().RLock()
		leaverName = leaver.Name
		leaver.Mu().RUnlock()
	}

	remaining, oldTeamID, ge := h.teamSvc.Leave(connCtx(conn), conn.PlayerID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamLeaveResp, &protocol.S2CTeamLeaveResp{Code: ge.Code})
		return
	}

	conn.Send(protocol.MsgIDTeamLeaveResp, &protocol.S2CTeamLeaveResp{
		Code: errors.ErrSuccess.Code, TeamID: oldTeamID,
	})
	if remaining != nil {
		h.broadcastUpdate(remaining, 2, fmt.Sprintf("%s 离开了队伍", leaverName))
	} else {
		// 队伍已解散，给原队员推送一个解散通知（此时没有 team，直接给原 conn 一个轻量推送）
		// remaining==nil 表示只剩自己一人离开即解散，无需广播
	}
}

// HandleTeamDismiss 处理解散队伍
func (h *TeamHandler) HandleTeamDismiss(conn *gateway.Conn, body []byte) {
	if onlinePlayer(h.world, conn) == nil {
		return
	}
	teamID, oldMembers, ge := h.teamSvc.Dismiss(connCtx(conn), conn.PlayerID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamDismissResp, &protocol.S2CTeamDismissResp{Code: ge.Code})
		return
	}
	conn.Send(protocol.MsgIDTeamDismissResp, &protocol.S2CTeamDismissResp{
		Code: errors.ErrSuccess.Code, TeamID: teamID,
	})
	// 通知所有原队员队伍已解散
	upd := &protocol.S2CTeamUpdate{
		Action: 3,
		TeamID: teamID,
		Reason: "队伍已被解散",
	}
	h.world.Hub().BroadcastToPlayers(oldMembers, protocol.MsgIDTeamUpdate, upd)
}

// HandleTeamKick 处理踢出队员
func (h *TeamHandler) HandleTeamKick(conn *gateway.Conn, body []byte) {
	var req protocol.C2STeamKick
	if err := json.Unmarshal(body, &req); err != nil {
		conn.Send(protocol.MsgIDTeamKickResp, &protocol.S2CTeamKickResp{
			Code: errors.ErrParamInvalid.Code, TargetID: req.TargetID,
		})
		return
	}
	if onlinePlayer(h.world, conn) == nil {
		return
	}

	kickedName := ""
	if t := h.world.GetOnlinePlayer(req.TargetID); t != nil {
		t.Mu().RLock()
		kickedName = t.Name
		t.Mu().RUnlock()
	}

	remaining, ge := h.teamSvc.Kick(connCtx(conn), conn.PlayerID, req.TargetID)
	if ge != nil {
		conn.Send(protocol.MsgIDTeamKickResp, &protocol.S2CTeamKickResp{
			Code: ge.Code, TargetID: req.TargetID,
		})
		return
	}
	conn.Send(protocol.MsgIDTeamKickResp, &protocol.S2CTeamKickResp{
		Code: errors.ErrSuccess.Code, TargetID: req.TargetID,
	})
	if remaining != nil {
		h.broadcastUpdate(remaining, 4, fmt.Sprintf("%s 被请出了队伍", kickedName))
	}
}

// HandleTeamQuery 处理查询我的队伍
func (h *TeamHandler) HandleTeamQuery(conn *gateway.Conn, body []byte) {
	if onlinePlayer(h.world, conn) == nil {
		return
	}
	t, ge := h.teamSvc.Query(connCtx(conn), conn.PlayerID)
	if ge != nil {
		// 不在队伍中返回 code 但不是错误（前端可据此显示"未加入队伍"）
		conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{Code: ge.Code})
		return
	}
	conn.Send(protocol.MsgIDTeamInfoResp, &protocol.S2CTeamInfoResp{
		Code: errors.ErrSuccess.Code,
		Team: h.buildTeamInfo(t),
	})
}
