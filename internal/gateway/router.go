package gateway

import (
	"sync"
)

// Router 是消息路由器，根据消息ID将消息分发到对应的处理函数。
// 支持全局中间件链，按注册顺序从外到内依次执行。
type Router struct {
	handlers    map[uint16]Handler // 消息ID → 处理函数的映射表
	middlewares []Middleware       // 全局中间件链
	mu          sync.RWMutex       // 保护 handlers 的读写锁
}

// NewRouter 创建并返回一个新的消息路由器实例。
func NewRouter() *Router {
	return &Router{
		handlers: make(map[uint16]Handler),
	}
}

// Use 注册全局中间件，中间件按注册顺序从外到内执行。
// 应在 Register 之前调用。
func (r *Router) Use(mw ...Middleware) {
	r.middlewares = append(r.middlewares, mw...)
}

// Register 注册指定消息ID的处理函数。
// 同一个消息ID重复注册会覆盖之前的处理函数。
func (r *Router) Register(msgID uint16, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[msgID] = h
}

// Handle 根据消息ID查找并执行对应的处理函数。
// 处理函数会被全局中间件链包装后执行。
// 返回 true 表示找到并执行了处理函数，false 表示没有注册该消息的处理函数。
func (r *Router) Handle(msgID uint16, conn *Conn, body []byte) bool {
	r.mu.RLock()
	h, ok := r.handlers[msgID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	// 从后往前包装中间件，使得先注册的中间件最先执行
	// 执行顺序：middlewares[0] → middlewares[1] → ... → handler
	wrapped := h
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		mw := r.middlewares[i]
		next := wrapped
		wrapped = func(c *Conn, b []byte) {
			mw(msgID, c, b, next)
		}
	}
	wrapped(conn, body)
	return true
}
