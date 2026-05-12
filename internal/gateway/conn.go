package gateway

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/logger"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"
	"golang.org/x/time/rate"
)

// Conn 代表一个客户端的 WebSocket 连接，
// 封装了发送通道、限流器、连接状态等信息。
type Conn struct {
	ID       string          // 连接的唯一标识，使用 xid 生成
	PlayerID uint64          // 关联的玩家ID，认证后赋值；0 表示未认证
	ws       *websocket.Conn // 底层 WebSocket 连接
	send     chan []byte     // 待发送消息的缓冲通道，容量为256
	hub      *Hub            // 所属的连接管理中心
	closed   bool            // 连接是否已关闭的标记
	mu       sync.Mutex      // 保护 closed 标记和 send 操作的互斥锁
	limiter  *rate.Limiter   // 令牌桶限流器，控制该连接的消息频率
}

// Limiter 返回连接内置的令牌桶限流器，
// 供中间件等外部模块进行限流检查。
func (c *Conn) Limiter() *rate.Limiter {
	return c.limiter
}

// newConn 创建一个新的客户端连接实例。
// 参数 rateLimit 指定每秒允许的最大消息数，同时作为令牌桶的突发容量。
func newConn(ws *websocket.Conn, hub *Hub, rateLimit int) *Conn {
	return &Conn{
		ID:      xid.New().String(),
		ws:      ws,
		send:    make(chan []byte, 256),
		hub:     hub,
		limiter: rate.NewLimiter(rate.Limit(rateLimit), rateLimit),
	}
}

// Send 将指定消息ID和数据序列化后发送到客户端。
// 如果连接已关闭则返回 ErrClosedPipe；
// 如果发送缓冲区已满则丢弃该消息并记录警告日志（避免阻塞）。
func (c *Conn) Send(msgID uint16, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	msg := encodeMessage(msgID, body)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return io.ErrClosedPipe
	}
	select {
	case c.send <- msg:
	default:
		logger.Warn("send buffer full, dropping message", "conn_id", c.ID, "msg_id", msgID)
	}
	return nil
}

// readPump 是连接的读取泵，在独立协程中运行。
// 持续从 WebSocket 读取消息，进行限流检查、消息解码和路由分发。
// 读超时为90秒，每次收到 Pong 帧会重置超时时间。
// 连接断开时自动从 Hub 注销并关闭底层 WebSocket。
func (c *Conn) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.ws.Close()
	}()

	// 设置读超时，90秒内无数据则断开
	c.ws.SetReadDeadline(time.Now().Add(90 * time.Second))
	// 收到 Pong 帧时重置读超时，保持连接存活
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Error("read error", "conn_id", c.ID, "err", err)
			}
			return
		}

		// 限流：超出频率限制的消息直接丢弃
		if !c.limiter.Allow() {
			logger.Warn("rate limited", "conn_id", c.ID)
			continue
		}

		msgID, body, err := decodeMessage(data)
		if err != nil {
			logger.Warn("decode message failed", "conn_id", c.ID, "err", err)
			continue
		}

		// 心跳消息特殊处理：原样回传时间戳，不进入业务路由
		if msgID == protocol.MsgIDHeartbeat {
			var hb protocol.C2SHeartbeat
			json.Unmarshal(body, &hb)
			c.Send(protocol.MsgIDHeartbeat, &protocol.S2CHeartbeat{Timestamp: hb.Timestamp})
			continue
		}

		// 将业务消息分发给路由器
		if !c.hub.router.Handle(msgID, c, body) {
			logger.Warn("no handler for message", "msg_id", msgID, "conn_id", c.ID)
		}
	}
}

// writePump 是连接的写入泵，在独立协程中运行。
// 从 send 通道取出消息写入 WebSocket，同时每30秒发送一次 Ping 帧保活。
// 写超时为10秒，发送通道关闭或写入失败时退出并关闭连接。
func (c *Conn) writePump() {
	ticker := time.NewTicker(30 * time.Second) // Ping 保活定时器
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// send 通道已被关闭，发送 Close 帧通知客户端
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			// 定时发送 Ping 帧，用于保持连接和检测客户端是否存活
			c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
