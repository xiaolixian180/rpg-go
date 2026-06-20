// Package chat 提供聊天系统：世界频道、私聊、队伍频道。
// 世界频道维护一个固定容量的环形缓冲历史；私聊和队伍频道为实时推送不保存历史。
package chat

import (
	"context"
	"sync"
	"time"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/errors"
	"hero-quest/pkg/logger"
)

// MaxContentLen 单条消息最大长度（字符数）
const MaxContentLen = 256

// WorldHistorySize 世界频道历史保留条数
const WorldHistorySize = 50

// ==================== 服务接口 ====================

// ChatService 聊天服务接口
type ChatService interface {
	// BuildWorldMessage 构造世界频道消息（不发送），返回时间戳
	BuildWorldMessage(ctx context.Context, senderID uint64, senderName string, content string) (*protocol.S2CChatMessage, *errors.GameError)
	// BuildPrivateMessage 构造私聊消息（不发送）
	BuildPrivateMessage(ctx context.Context, senderID uint64, senderName string, targetID uint64, content string) (*protocol.S2CChatMessage, *errors.GameError)
	// BuildTeamMessage 构造队伍频道消息（不发送）
	BuildTeamMessage(ctx context.Context, senderID uint64, senderName string, content string) (*protocol.S2CChatMessage, *errors.GameError)
	// AppendHistory 追加到世界历史（仅世界频道调用）
	AppendHistory(msg *protocol.S2CChatMessage)
	// GetHistory 返回最近 count 条世界历史（按时间升序）
	GetHistory(count int) []protocol.S2CChatMessage
}

// ==================== 服务实现 ====================

type chatService struct {
	mu      sync.Mutex
	history []protocol.S2CChatMessage // 环形缓冲
	head    int                       // 下一个写入位置
	full    bool                      // 是否已写满（决定读取顺序）
}

// NewChatService 创建聊天服务实例
func NewChatService() ChatService {
	return &chatService{
		history: make([]protocol.S2CChatMessage, WorldHistorySize),
	}
}

// validateContent 校验消息内容
func validateContent(content string) *errors.GameError {
	if content == "" {
		return errors.ErrChatMsgEmpty
	}
	// 按字节长度粗略限制（UTF-8 中文一字占3字节，按字符数限制更严格但需 rune 转换）
	if len([]rune(content)) > MaxContentLen {
		return errors.ErrChatMsgTooLong
	}
	return nil
}

// BuildWorldMessage 构造世界频道消息
func (s *chatService) BuildWorldMessage(ctx context.Context, senderID uint64, senderName string, content string) (*protocol.S2CChatMessage, *errors.GameError) {
	if ge := validateContent(content); ge != nil {
		return nil, ge
	}
	msg := &protocol.S2CChatMessage{
		Channel:    protocol.ChatChannelWorld,
		SenderID:   senderID,
		SenderName: senderName,
		Content:    content,
		Timestamp:  time.Now().UnixMilli(),
	}
	logger.TInfo(ctx, "世界频道消息", "sender_id", senderID, "len", len(content))
	return msg, nil
}

// BuildPrivateMessage 构造私聊消息
func (s *chatService) BuildPrivateMessage(ctx context.Context, senderID uint64, senderName string, targetID uint64, content string) (*protocol.S2CChatMessage, *errors.GameError) {
	if ge := validateContent(content); ge != nil {
		return nil, ge
	}
	if senderID == targetID {
		return nil, errors.ErrTeamInviteSelf // 复用：不能对自己私聊
	}
	msg := &protocol.S2CChatMessage{
		Channel:    protocol.ChatChannelPrivate,
		SenderID:   senderID,
		SenderName: senderName,
		TargetID:   targetID,
		Content:    content,
		Timestamp:  time.Now().UnixMilli(),
	}
	logger.TInfo(ctx, "私聊消息", "sender_id", senderID, "target_id", targetID, "len", len(content))
	return msg, nil
}

// BuildTeamMessage 构造队伍频道消息
func (s *chatService) BuildTeamMessage(ctx context.Context, senderID uint64, senderName string, content string) (*protocol.S2CChatMessage, *errors.GameError) {
	if ge := validateContent(content); ge != nil {
		return nil, ge
	}
	msg := &protocol.S2CChatMessage{
		Channel:    protocol.ChatChannelTeam,
		SenderID:   senderID,
		SenderName: senderName,
		Content:    content,
		Timestamp:  time.Now().UnixMilli(),
	}
	logger.TInfo(ctx, "队伍频道消息", "sender_id", senderID, "len", len(content))
	return msg, nil
}

// AppendHistory 写入环形缓冲
func (s *chatService) AppendHistory(msg *protocol.S2CChatMessage) {
	if msg == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[s.head] = *msg
	s.head = (s.head + 1) % WorldHistorySize
	if s.head == 0 {
		s.full = true
	}
}

// GetHistory 返回最近 count 条历史（升序）
func (s *chatService) GetHistory(count int) []protocol.S2CChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count <= 0 {
		return nil
	}
	if count > WorldHistorySize {
		count = WorldHistorySize
	}

	var available int
	if s.full {
		available = WorldHistorySize
	} else {
		available = s.head
	}
	if count > available {
		count = available
	}
	if count == 0 {
		return nil
	}

	out := make([]protocol.S2CChatMessage, 0, count)
	// 升序：先写最早的
	start := s.head - count
	if start < 0 {
		start += WorldHistorySize
	}
	for i := 0; i < count; i++ {
		idx := (start + i) % WorldHistorySize
		out = append(out, s.history[idx])
	}
	return out
}
