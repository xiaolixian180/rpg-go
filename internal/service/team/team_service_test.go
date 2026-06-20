package team

import (
	"context"
	"testing"

	"hero-quest/pkg/errors"
)

func TestCreateAndQuery(t *testing.T) {
	s := NewTeamService()
	team, ge := s.Create(context.Background(), 1001)
	if ge != nil {
		t.Fatalf("Create failed: %v", ge)
	}
	if team.LeaderID != 1001 || len(team.Members) != 1 {
		t.Fatalf("unexpected team state: %+v", team)
	}

	// 同一玩家不能重复创建
	if _, ge := s.Create(context.Background(), 1001); ge == nil || ge.Code != errors.ErrAlreadyInTeam.Code {
		t.Fatalf("expected ErrAlreadyInTeam, got %v", ge)
	}

	// 查询
	got, ge := s.Query(context.Background(), 1001)
	if ge != nil || got.ID != team.ID {
		t.Fatalf("Query mismatch: ge=%v got=%+v", ge, got)
	}

	// 没在队中的玩家查询
	if _, ge := s.Query(context.Background(), 9999); ge == nil || ge.Code != errors.ErrNotInTeam.Code {
		t.Fatalf("expected ErrNotInTeam, got %v", ge)
	}
}

func TestInviteAcceptAndFull(t *testing.T) {
	s := NewTeamService()
	team, _ := s.Create(context.Background(), 1)

	// 加入直到满
	for i := uint64(2); i <= uint64(MaxTeamMembers); i++ {
		if _, ge := s.AcceptInvite(context.Background(), i, team.ID); ge != nil {
			t.Fatalf("AcceptInvite %d failed: %v", i, ge)
		}
	}
	// 第6人 → ErrTeamFull
	if _, ge := s.AcceptInvite(context.Background(), 99, team.ID); ge == nil || ge.Code != errors.ErrTeamFull.Code {
		t.Fatalf("expected ErrTeamFull, got %v", ge)
	}
	// 已在队中再加入 → ErrAlreadyInTeam
	if _, ge := s.AcceptInvite(context.Background(), 1, team.ID); ge == nil || ge.Code != errors.ErrAlreadyInTeam.Code {
		t.Fatalf("expected ErrAlreadyInTeam, got %v", ge)
	}
	// 不存在的 team
	if _, ge := s.AcceptInvite(context.Background(), 100, 99999); ge == nil || ge.Code != errors.ErrTeamNotFound.Code {
		t.Fatalf("expected ErrTeamNotFound, got %v", ge)
	}
}

func TestLeaveLeaderTransfer(t *testing.T) {
	s := NewTeamService()
	team, _ := s.Create(context.Background(), 1)
	_, _ = s.AcceptInvite(context.Background(), 2, team.ID)
	_, _ = s.AcceptInvite(context.Background(), 3, team.ID)

	// 队长离开 → 队长应转移
	remaining, _, ge := s.Leave(context.Background(), 1)
	if ge != nil {
		t.Fatalf("Leave leader failed: %v", ge)
	}
	if remaining.LeaderID == 1 {
		t.Fatalf("leader should be transferred, still 1")
	}
	if !remaining.Contains(2) || !remaining.Contains(3) {
		t.Fatalf("remaining members wrong: %+v", remaining.Members)
	}

	// 最后一人离开 → 解散
	lastID := remaining.LeaderID
	// 让另一个人先离开
	s.Leave(context.Background(), other(remaining.Members, lastID))
	got, _ := s.Query(context.Background(), lastID)
	otherID := other(got.Members, lastID)
	if otherID != 0 {
		s.Leave(context.Background(), otherID)
	}
	if _, _, ge := s.Leave(context.Background(), lastID); ge != nil {
		t.Fatalf("final Leave failed: %v", ge)
	}
	// 队伍应已不存在
	if _, ge := s.Query(context.Background(), lastID); ge == nil {
		t.Fatalf("team should be dismissed after last member left")
	}
}

func TestDismissAndKick(t *testing.T) {
	s := NewTeamService()
	team, _ := s.Create(context.Background(), 1)
	_, _ = s.AcceptInvite(context.Background(), 2, team.ID)
	_, _ = s.AcceptInvite(context.Background(), 3, team.ID)

	// 非队长不能解散
	if _, _, ge := s.Dismiss(context.Background(), 2); ge == nil || ge.Code != errors.ErrNotTeamLeader.Code {
		t.Fatalf("expected ErrNotTeamLeader, got %v", ge)
	}
	// 队长解散
	teamID, members, ge := s.Dismiss(context.Background(), 1)
	if ge != nil || teamID != team.ID || len(members) != 3 {
		t.Fatalf("Dismiss failed: ge=%v teamID=%d members=%v", ge, teamID, members)
	}
	// 解散后所有人都已不在队
	for _, m := range members {
		if _, ge := s.Query(context.Background(), m); ge == nil {
			t.Fatalf("member %d still in team after Dismiss", m)
		}
	}

	// 踢人
	team2, _ := s.Create(context.Background(), 10)
	_, _ = s.AcceptInvite(context.Background(), 11, team2.ID)
	remaining, ge := s.Kick(context.Background(), 10, 11)
	if ge != nil || remaining.Contains(11) {
		t.Fatalf("Kick failed: ge=%v remaining=%+v", ge, remaining)
	}
	if _, ge := s.Query(context.Background(), 11); ge == nil {
		t.Fatalf("kicked player still in team")
	}

	// 踢自己 → 错误
	if _, ge := s.Kick(context.Background(), 10, 10); ge == nil || ge.Code != errors.ErrTeamInviteSelf.Code {
		t.Fatalf("expected ErrTeamInviteSelf, got %v", ge)
	}
}

// other 返回 members 中除 skip 外的第一个ID；没有则返回 0
func other(members []uint64, skip uint64) uint64 {
	for _, m := range members {
		if m != skip {
			return m
		}
	}
	return 0
}
