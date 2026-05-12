package gateway

import (
	"sync"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/logger"
)

// Hub 是连接管理中心，维护所有活跃的客户端连接，
// 支持按连接ID和玩家ID查找，以及重连时的顶号逻辑。
type Hub struct {
	router      *Router                  // 消息路由器，用于分发业务消息
	conns       map[string]*Conn         // 连接ID → 连接实例的映射
	playerConns map[uint64]*Conn         // 玩家ID → 连接实例的映射，支持重连时顶号
	mu          sync.RWMutex             // 保护 conns 和 playerConns 的读写锁
	onAuth      func(token string) (uint64, bool) // 认证回调，根据 token 返回玩家ID和是否合法
	onClose     func(conn *Conn)         // 连接关闭回调，用于业务层清理玩家状态等
}

// NewHub 创建并返回一个新的连接管理中心实例。
func NewHub(router *Router) *Hub {
	return &Hub{
		router:      router,
		conns:       make(map[string]*Conn),
		playerConns: make(map[uint64]*Conn),
	}
}

// SetAuthHandler 设置认证回调函数。
// 回调接收 token 字符串，返回玩家ID和认证是否成功。
func (h *Hub) SetAuthHandler(fn func(token string) (uint64, bool)) {
	h.onAuth = fn
}

// SetCloseHandler 设置连接关闭回调函数。
// 当连接断开并被注销时调用，用于业务层执行清理操作（如保存玩家数据）。
func (h *Hub) SetCloseHandler(fn func(conn *Conn)) {
	h.onClose = fn
}

// Register 将连接注册到 Hub 中。
// 如果该连接已绑定玩家ID，会先检查是否已有同玩家的旧连接，
// 若存在则踢掉旧连接（顶号逻辑），再建立新映射。
func (h *Hub) Register(conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn.ID] = conn
	if conn.PlayerID > 0 {
		// 重连：踢掉旧连接，发送踢出通知后关闭旧连接的发送通道
		if old, ok := h.playerConns[conn.PlayerID]; ok {
			old.Send(protocol.MsgIDKick, &protocol.S2CKick{Reason: "账号在其他地方登录"})
			close(old.send)
			old.mu.Lock()
			old.closed = true
			old.mu.Unlock()
			delete(h.conns, old.ID)
		}
		h.playerConns[conn.PlayerID] = conn
	}
	logger.Info("client connected", "conn_id", conn.ID, "player_id", conn.PlayerID)
}

// Unregister 从 Hub 中注销连接，清理相关映射。
// 关闭发送通道、标记连接为已关闭，并触发 onClose 回调通知业务层。
func (h *Hub) Unregister(conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[conn.ID]; ok {
		delete(h.conns, conn.ID)
		if conn.PlayerID > 0 {
			delete(h.playerConns, conn.PlayerID)
		}
		close(conn.send)
		conn.mu.Lock()
		conn.closed = true
		conn.mu.Unlock()
		if h.onClose != nil {
			h.onClose(conn)
		}
		logger.Info("client disconnected", "conn_id", conn.ID, "player_id", conn.PlayerID)
	}
}

// Broadcast 向所有活跃连接广播消息。
func (h *Hub) Broadcast(msgID uint16, v any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.conns {
		c.Send(msgID, v)
	}
}

// BroadcastToLayer 向指定层级的所有连接广播消息，排除指定连接ID。
// 注意：当前实现未使用 layer 参数，所有非排除连接都会收到消息，
// 后续可根据层级字段做更精确的过滤。
func (h *Hub) BroadcastToLayer(layer int32, msgID uint16, v any, excludeID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.conns {
		if c.ID != excludeID {
			c.Send(msgID, v)
		}
	}
}

// GetConnByPlayerID 根据玩家ID查找对应的活跃连接。
// 用于向指定玩家发送私聊消息等场景。
func (h *Hub) GetConnByPlayerID(playerID uint64) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.playerConns[playerID]
}

// OnlineCount 返回当前在线连接数。
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
