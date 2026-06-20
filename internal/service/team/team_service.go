// Package team 提供组队系统：创建/邀请/加入/离开/解散/踢出/状态同步。
// 数据全部在内存中维护，玩家断线时由 World.OnLogout 触发 Leave 清理。
package team

import (
	"context"
	"sync"
	"sync/atomic"

	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// MaxTeamMembers 单支队伍最大人数（含队长）
const MaxTeamMembers = 5

// ==================== 数据结构 ====================

// Team 队伍数据
type Team struct {
	ID       uint64   // 队伍唯一ID
	LeaderID uint64   // 队长ID
	Members  []uint64 // 队员ID列表（LeaderID 始终位于 Members[0]）
}

// MemberIDs 返回队员ID列表的副本，避免外部修改
func (t *Team) MemberIDs() []uint64 {
	if t == nil {
		return nil
	}
	out := make([]uint64, len(t.Members))
	copy(out, t.Members)
	return out
}

// Contains 判断玩家是否在队中
func (t *Team) Contains(playerID uint64) bool {
	if t == nil {
		return false
	}
	for _, m := range t.Members {
		if m == playerID {
			return true
		}
	}
	return false
}

// ==================== 服务接口 ====================

// TeamService 组队服务接口
type TeamService interface {
	// Create 创建队伍，leader 成为队长。若已在队伍中则返回 ErrAlreadyInTeam
	Create(ctx context.Context, leaderID uint64) (*Team, *errors.GameError)
	// Query 查询玩家当前所在队伍，未在队中返回 (nil, ErrNotInTeam)
	Query(ctx context.Context, playerID uint64) (*Team, *errors.GameError)
	// MemberIDs 返回玩家所在队伍的全部队员ID（未在队中返回空切片）
	MemberIDs(playerID uint64) []uint64
	// Leave 离开队伍。队长离开若剩余队员 ≥1 则自动转移队长；若只剩自己则解散
	Leave(ctx context.Context, playerID uint64) (*Team, uint64, *errors.GameError)
	// Dismiss 解散队伍（仅队长）。返回旧队伍ID和原队员列表
	Dismiss(ctx context.Context, leaderID uint64) (uint64, []uint64, *errors.GameError)
	// AcceptInvite 接受邀请加入指定队伍
	AcceptInvite(ctx context.Context, playerID uint64, teamID uint64) (*Team, *errors.GameError)
	// Kick 踢出队员（仅队长）
	Kick(ctx context.Context, leaderID uint64, targetID uint64) (*Team, *errors.GameError)
	// Cleanup 玩家下线清理：从队伍中移除（语义同 Leave，但不返回转移后的队伍）
	Cleanup(playerID uint64)
}

// ==================== 服务实现 ====================

type teamService struct {
	mu       sync.RWMutex
	teams    map[uint64]*Team  // teamID → Team
	playerOf map[uint64]uint64 // playerID → teamID
	nextID   uint64            // 自增 teamID 生成器
}

// NewTeamService 创建组队服务实例
func NewTeamService() TeamService {
	return &teamService{
		teams:    make(map[uint64]*Team),
		playerOf: make(map[uint64]uint64),
	}
}

// Create 创建队伍
func (s *teamService) Create(ctx context.Context, leaderID uint64) (*Team, *errors.GameError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.playerOf[leaderID]; ok {
		return nil, errors.ErrAlreadyInTeam
	}

	id := atomic.AddUint64(&s.nextID, 1)
	t := &Team{
		ID:       id,
		LeaderID: leaderID,
		Members:  []uint64{leaderID},
	}
	s.teams[id] = t
	s.playerOf[leaderID] = id

	logger.TInfo(ctx, "创建队伍", "team_id", id, "leader_id", leaderID)
	return t, nil
}

// Query 查询玩家所在队伍
func (s *teamService) Query(ctx context.Context, playerID uint64) (*Team, *errors.GameError) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	teamID, ok := s.playerOf[playerID]
	if !ok {
		return nil, errors.ErrNotInTeam
	}
	return s.teams[teamID], nil
}

// MemberIDs 玩家所在队伍的队员ID
func (s *teamService) MemberIDs(playerID uint64) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	teamID, ok := s.playerOf[playerID]
	if !ok {
		return nil
	}
	t := s.teams[teamID]
	if t == nil {
		return nil
	}
	return t.MemberIDs()
}

// Leave 离开队伍
// 返回值：
//
//	*Team: 离开后的剩余队伍（若解散则为 nil）
//	uint64: 离开者所属的原 teamID（无论是否解散都返回）
func (s *teamService) Leave(ctx context.Context, playerID uint64) (*Team, uint64, *errors.GameError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamID, ok := s.playerOf[playerID]
	if !ok {
		return nil, 0, errors.ErrNotInTeam
	}
	t := s.teams[teamID]
	if t == nil {
		delete(s.playerOf, playerID)
		return nil, teamID, errors.ErrTeamNotFound
	}

	// 从成员列表移除
	newMembers := make([]uint64, 0, len(t.Members))
	for _, m := range t.Members {
		if m != playerID {
			newMembers = append(newMembers, m)
		}
	}
	delete(s.playerOf, playerID)

	// 没有剩余成员 → 解散
	if len(newMembers) == 0 {
		delete(s.teams, teamID)
		logger.TInfo(ctx, "队伍解散（最后一人离开）", "team_id", teamID, "player_id", playerID)
		return nil, teamID, nil
	}

	// 队长离开 → 转移队长给第一个剩余成员
	if t.LeaderID == playerID {
		t.LeaderID = newMembers[0]
	}
	t.Members = newMembers

	logger.TInfo(ctx, "玩家离开队伍", "team_id", teamID, "player_id", playerID, "remaining", len(newMembers))
	return t, teamID, nil
}

// Dismiss 解散队伍（仅队长）
func (s *teamService) Dismiss(ctx context.Context, leaderID uint64) (uint64, []uint64, *errors.GameError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamID, ok := s.playerOf[leaderID]
	if !ok {
		return 0, nil, errors.ErrNotInTeam
	}
	t := s.teams[teamID]
	if t == nil {
		delete(s.playerOf, leaderID)
		return 0, nil, errors.ErrTeamNotFound
	}
	if t.LeaderID != leaderID {
		return 0, nil, errors.ErrNotTeamLeader
	}

	// 收集原成员，清空所有映射
	oldMembers := t.MemberIDs()
	for _, m := range oldMembers {
		delete(s.playerOf, m)
	}
	delete(s.teams, teamID)

	logger.TInfo(ctx, "队伍解散（队长操作）", "team_id", teamID, "leader_id", leaderID, "members", len(oldMembers))
	return teamID, oldMembers, nil
}

// AcceptInvite 接受邀请加入队伍
func (s *teamService) AcceptInvite(ctx context.Context, playerID uint64, teamID uint64) (*Team, *errors.GameError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 玩家已在其他队伍
	if _, ok := s.playerOf[playerID]; ok {
		return nil, errors.ErrAlreadyInTeam
	}
	t, ok := s.teams[teamID]
	if !ok {
		return nil, errors.ErrTeamNotFound
	}
	if len(t.Members) >= MaxTeamMembers {
		return nil, errors.ErrTeamFull
	}

	t.Members = append(t.Members, playerID)
	s.playerOf[playerID] = teamID

	logger.TInfo(ctx, "玩家加入队伍", "team_id", teamID, "player_id", playerID, "members", len(t.Members))
	return t, nil
}

// Kick 踢出队员
func (s *teamService) Kick(ctx context.Context, leaderID uint64, targetID uint64) (*Team, *errors.GameError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	teamID, ok := s.playerOf[leaderID]
	if !ok {
		return nil, errors.ErrNotInTeam
	}
	t := s.teams[teamID]
	if t == nil {
		delete(s.playerOf, leaderID)
		return nil, errors.ErrTeamNotFound
	}
	if t.LeaderID != leaderID {
		return nil, errors.ErrNotTeamLeader
	}
	if leaderID == targetID {
		return nil, errors.ErrTeamInviteSelf
	}
	if !t.Contains(targetID) {
		return nil, errors.ErrNotTeamMember
	}

	newMembers := make([]uint64, 0, len(t.Members)-1)
	for _, m := range t.Members {
		if m != targetID {
			newMembers = append(newMembers, m)
		}
	}
	t.Members = newMembers
	delete(s.playerOf, targetID)

	logger.TInfo(ctx, "队员被踢出", "team_id", teamID, "target_id", targetID, "leader_id", leaderID)
	return t, nil
}

// Cleanup 玩家下线时调用（与 Leave 等价，但只记录一条日志）
func (s *teamService) Cleanup(playerID uint64) {
	_, _, _ = s.Leave(context.Background(), playerID)
}
