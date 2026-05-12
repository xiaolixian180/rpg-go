// Package gateway 实现了游戏服务器的 WebSocket 网关层，
// 负责客户端连接管理、消息路由、限流、认证以及广播等功能。
package gateway

import (
	"context"
	"fmt"
	"net/http"

	"hero-quest/pkg/logger"

	"github.com/gorilla/websocket"
)

// upgrader 是 HTTP 到 WebSocket 的升级器，
// CheckOrigin 返回 true 表示允许所有来源的跨域请求（生产环境应按需限制）。
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler 定义消息处理函数的类型，
// 接收客户端连接和消息体，由业务层注册具体处理逻辑。
type Handler func(conn *Conn, body []byte)

// Gateway 是网关服务的主结构体，
// 封装了 Hub、Router 及 HTTP 服务器等配置，提供 WebSocket 服务的启动和优雅关闭入口。
type Gateway struct {
	hub       *Hub        // 连接管理中心
	router    *Router     // 消息路由器
	addr      string      // 监听地址（如 ":8080"）
	rateLimit int         // 每个连接的每秒消息频率限制
	server    *http.Server // HTTP 服务器，支持优雅关闭
}

// New 创建并返回一个新的网关实例。
// 默认限流为每秒30条消息。
func New(addr string, router *Router) *Gateway {
	gw := &Gateway{
		hub:       NewHub(router),
		router:    router,
		addr:      addr,
		rateLimit: 30,
		server:    &http.Server{Addr: addr},
	}

	// 注册 HTTP 路由处理器
	mux := http.NewServeMux()

	// WebSocket 升级端点
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// 将 HTTP 连接升级为 WebSocket
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("upgrade failed", "err", err)
			return
		}

		// 从 URL 参数中获取认证 token
		token := r.URL.Query().Get("token")
		conn := newConn(ws, gw.hub, gw.rateLimit)

		// 如果设置了认证回调，验证 token；认证失败则直接关闭连接
		if gw.hub.onAuth != nil {
			playerID, ok := gw.hub.onAuth(token)
			if !ok {
				logger.Warn("auth failed", "conn_id", conn.ID)
				ws.Close()
				return
			}
			conn.PlayerID = playerID
		}

		// 将连接注册到 Hub，然后启动读写泵
		gw.hub.Register(conn)
		go conn.writePump()
		go conn.readPump()
	})

	// 健康检查接口
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 在线人数接口
	mux.HandleFunc("/online", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%d", gw.hub.OnlineCount())))
	})

	gw.server.Handler = mux
	return gw
}

// SetRateLimit 设置每个连接的每秒消息频率限制。
func (g *Gateway) SetRateLimit(rps int) {
	g.rateLimit = rps
}

// Hub 返回网关内部的连接管理中心，供外部访问。
func (g *Gateway) Hub() *Hub {
	return g.hub
}

// Router 返回网关内部的消息路由器，供外部注册消息处理函数。
func (g *Gateway) Router() *Router {
	return g.router
}

// Run 启动网关服务，开始监听 HTTP 请求。
// 该方法会阻塞，直到服务器关闭或出错。
func (g *Gateway) Run() error {
	logger.Info("gateway starting", "addr", g.addr)
	return g.server.ListenAndServe()
}

// Shutdown 优雅关闭网关服务。
// 使用 http.Server.Shutdown 方法，在指定超时时间内等待所有连接处理完成，
// 超时后强制关闭。关闭后 Run 方法会返回 http.ErrServerClosed。
func (g *Gateway) Shutdown(ctx context.Context) error {
	logger.Info("gateway shutting down...")
	return g.server.Shutdown(ctx)
}