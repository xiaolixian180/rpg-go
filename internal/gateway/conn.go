package gateway

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/logger"

	"github.com/coder/websocket"
	"github.com/rs/xid"
	"golang.org/x/time/rate"
)

// Conn 代表一个客户端的 WebSocket 连接
type Conn struct {
	ID       string          // 连接唯一标识
	PlayerID uint64          // 关联玩家ID
	ws       *websocket.Conn // 底层 WebSocket 连接
	send     chan []byte     // 发送缓冲通道
	hub      *Hub            // 连接管理中心
	closed   bool            // 是否已关闭
	mu       sync.Mutex      // 保护 closed 和 send
	limiter  *rate.Limiter   // 令牌桶限流器
	traceCtx context.Context // 追踪上下文，携带 trace_id
}

// Limiter 返回连接的令牌桶限流器
func (c *Conn) Limiter() *rate.Limiter {
	return c.limiter
}

// Context 返回连接的追踪上下文
func (c *Conn) Context() context.Context {
	return c.traceCtx
}

// BindPlayer 将认证后的玩家ID绑定到当前连接。
func (c *Conn) BindPlayer(playerID uint64) {
	c.hub.BindPlayer(c, playerID)
}

// newConn 创建新的客户端连接实例
func newConn(ws *websocket.Conn, hub *Hub, rateLimit int) *Conn {
	return &Conn{
		ID:       xid.New().String(),
		ws:       ws,
		send:     make(chan []byte, 256),
		hub:      hub,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), rateLimit),
		traceCtx: context.Background(),
	}
}

// Send 将指定消息ID和数据序列化后发送到客户端
func (c *Conn) Send(msgID uint16, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.SendRaw(encodeMessage(msgID, body))
}

// SendRaw 将预序列化的二进制消息写入发送缓冲区
func (c *Conn) SendRaw(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return io.ErrClosedPipe
	}
	select {
	case c.send <- msg:
	default:
		logger.Warn("发送缓冲区满，丢弃消息", "conn_id", c.ID, "trace_id", logger.GetTrace(c.traceCtx))
	}
	return nil
}

// readPump 连接的读取泵
func (c *Conn) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.ws.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		_, data, err := c.ws.Read(ctx)
		cancel()
		if err != nil {
			closeStatus := websocket.CloseStatus(err)
			if closeStatus != websocket.StatusNormalClosure && closeStatus != websocket.StatusGoingAway {
				logger.Error("读取错误", "conn_id", c.ID, "player_id", c.PlayerID, "err", err)
			}
			return
		}

		msgID, body, err := decodeMessage(data)
		if err != nil {
			logger.Warn("消息解码失败", "conn_id", c.ID, "err", err)
			continue
		}

		// 心跳消息特殊处理
		if msgID == protocol.MsgIDHeartbeat {
			var hb protocol.C2SHeartbeat
			if err := json.Unmarshal(body, &hb); err != nil {
				logger.Debug("心跳消息解析失败", "conn_id", c.ID, "err", err)
			} else {
				c.Send(protocol.MsgIDHeartbeat, &protocol.S2CHeartbeat{Timestamp: hb.Timestamp})
			}
			continue
		}

		// 为每条消息重置追踪上下文
		c.traceCtx = logger.WithTrace(context.Background(), logger.NewTraceID())

		// 分发业务消息
		if !c.hub.router.Handle(msgID, c, body) {
			logger.Warn("未注册消息处理", "msg_id", msgID, "conn_id", c.ID, "trace_id", logger.GetTrace(c.traceCtx))
		}
	}
}

// writePump 连接的写入泵
func (c *Conn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, writeCancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.ws.Write(writeCtx, websocket.MessageBinary, msg)
			writeCancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.ws.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return
			}
		}
	}
}
