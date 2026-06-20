package chat

import (
	"context"
	"testing"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/errors"
)

func TestValidateContent(t *testing.T) {
	cases := []struct {
		content string
		want    *errors.GameError
	}{
		{"", errors.ErrChatMsgEmpty},
		{"hi", nil},
		{string(make([]rune, MaxContentLen+1)), errors.ErrChatMsgTooLong},
		{string(make([]rune, MaxContentLen)), nil},
	}
	for _, c := range cases {
		if ge := validateContent(c.content); (ge == nil) != (c.want == nil) {
			t.Fatalf("validateContent(%q) ge=%v want=%v", c.content, ge, c.want)
		}
	}
}

func TestBuildMessages(t *testing.T) {
	s := NewChatService()
	ctx := context.Background()

	// 世界消息
	w, ge := s.BuildWorldMessage(ctx, 1, "Alice", "hello world")
	if ge != nil || w.Channel != protocol.ChatChannelWorld || w.Content != "hello world" {
		t.Fatalf("world msg wrong: ge=%v w=%+v", ge, w)
	}

	// 私聊
	p, ge := s.BuildPrivateMessage(ctx, 1, "Alice", 2, "psst")
	if ge != nil || p.Channel != protocol.ChatChannelPrivate || p.TargetID != 2 {
		t.Fatalf("private msg wrong: ge=%v p=%+v", ge, p)
	}

	// 私聊自己 → 错误
	if _, ge := s.BuildPrivateMessage(ctx, 1, "A", 1, "x"); ge == nil {
		t.Fatalf("expected error for private to self")
	}

	// 队伍消息
	tm, ge := s.BuildTeamMessage(ctx, 1, "Alice", "team hi")
	if ge != nil || tm.Channel != protocol.ChatChannelTeam {
		t.Fatalf("team msg wrong: ge=%v tm=%+v", ge, tm)
	}

	// 空消息 → 错误
	if _, ge := s.BuildWorldMessage(ctx, 1, "A", ""); ge == nil {
		t.Fatalf("expected ErrChatMsgEmpty")
	}
}

func TestWorldHistoryRingBuffer(t *testing.T) {
	s := NewChatService()
	ctx := context.Background()

	// 写入 60 条，应只保留最近 50 条
	for i := 0; i < 60; i++ {
		m, _ := s.BuildWorldMessage(ctx, uint64(i), "u", "msg")
		s.AppendHistory(m)
	}
	got := s.GetHistory(50)
	if len(got) != 50 {
		t.Fatalf("expected 50 messages, got %d", len(got))
	}
	// 第一条应是序号10对应的消息（最早保留）
	if got[0].SenderID != 10 {
		t.Fatalf("oldest kept should be sender 10, got %d", got[0].SenderID)
	}
	// 最后一条应是序号59
	if got[49].SenderID != 59 {
		t.Fatalf("newest should be sender 59, got %d", got[49].SenderID)
	}

	// 拉取超过容量
	got = s.GetHistory(100)
	if len(got) != 50 {
		t.Fatalf("GetHistory(100) should clamp to 50, got %d", len(got))
	}
}
