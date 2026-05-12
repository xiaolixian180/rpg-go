package gateway

import (
	"sync"
)

// Router 是消息路由器，根据消息ID将消息分发到对应的处理函数。
// 使用读写锁保证并发安全。
type Router struct {
	handlers map[uint16]Handler // 消息ID → 处理函数的映射表
	mu       sync.RWMutex       // 保护 handlers 的读写锁
}

// NewRouter 创建并返回一个新的消息路由器实例。
func NewRouter() *Router {
	return &Router{
		handlers: make(map[uint16]Handler),
	}
}

// Register 注册指定消息ID的处理函数。
// 同一个消息ID重复注册会覆盖之前的处理函数。
func (r *Router) Register(msgID uint16, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[msgID] = h
}

// Handle 根据消息ID查找并执行对应的处理函数。
// 返回 true 表示找到并执行了处理函数，false 表示没有注册该消息的处理函数。
func (r *Router) Handle(msgID uint16, conn *Conn, body []byte) bool {
	r.mu.RLock()
	h, ok := r.handlers[msgID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	h(conn, body)
	return true
}
