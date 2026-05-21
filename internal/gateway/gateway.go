// Package gateway 实现了游戏服务器的 WebSocket 网关层，
// 负责客户端连接管理、消息路由、限流、认证以及广播等功能。
package gateway

import (
	"context"
	"expvar"
	"fmt"
	"net/http"

	"hero-quest/internal/protocol"
	"hero-quest/pkg/logger"

	"github.com/coder/websocket"
)

// Handler 定义消息处理函数的类型，
// 接收客户端连接和消息体，由业务层注册具体处理逻辑。
type Handler func(conn *Conn, body []byte)

// Gateway 是网关服务的主结构体，
// 封装了 Hub、Router 及 HTTP 服务器等配置，提供 WebSocket 服务的启动和优雅关闭入口。
type Gateway struct {
	hub            *Hub         // 连接管理中心
	router         *Router      // 消息路由器
	addr           string       // 监听地址（如 ":8080"）
	allowedOrigins []string     // 允许的来源列表，非空时仅允许匹配的 Origin 连接
	rateLimit      int          // 每个连接的每秒消息频率限制
	server         *http.Server // HTTP 服务器，支持优雅关闭
}

// New 创建并返回一个新的网关实例。
// 默认限流为每秒30条消息，注册以下中间件链（从外到内）：
//
//	Recovery → Trace → AccessLog → RateLimit → MaxBodySize(4KB) → AuthGuard → Metrics → Handler
func New(addr string, router *Router) *Gateway {
	// 注册默认中间件（从外到内执行）
	router.Use(
		RecoveryMiddleware,          // 最外层：兜底 panic
		TraceMiddleware,             // 生成 trace_id
		AccessLogMiddleware,         // 请求日志（含耗时）
		RateLimitMiddleware,         // 限流
		MaxBodySizeMiddleware(4096), // 最大包大小 4KB
		AuthGuardMiddleware( // 认证守卫
			protocol.MsgIDLogin,        // 登录免认证
			protocol.MsgIDCreatePlayer, // 创建角色免认证
		),
		MetricsMiddleware, // 最内层：指标采集
	)

	gw := &Gateway{
		hub:            NewHub(router),
		router:         router,
		addr:           addr,
		rateLimit:      30,
		server:         &http.Server{Addr: addr},
		allowedOrigins: []string{}, // 空列表表示允许所有来源，生产环境应配置具体域名
	}

	// 注册 HTTP 路由处理器
	mux := http.NewServeMux()

	// WebSocket 升级端点
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// 来源检查：如果配置了白名单，则验证请求 Origin
		if len(gw.allowedOrigins) > 0 {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range gw.allowedOrigins {
				if o == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				logger.Warn("WebSocket连接来源被拒绝", "origin", origin)
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}

		// 将 HTTP 连接升级为 WebSocket
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
		if err != nil {
			logger.Error("WebSocket升级失败", "err", err)
			return
		}

		// 从 URL 参数中获取认证 token
		token := r.URL.Query().Get("token")

		// 认证检查必须在 Register 之前，否则认证失败的连接会泄漏在 Hub 中
		var playerID uint64
		if gw.hub.onAuth != nil {
			pid, ok := gw.hub.onAuth(token)
			if !ok {
				logger.Warn("认证失败")
				ws.Close(websocket.StatusNormalClosure, "auth failed")
				return
			}
			playerID = pid
		}

		// 认证通过后才创建连接并注册
		conn := newConn(ws, gw.hub, gw.rateLimit)
		conn.PlayerID = playerID

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

	// 指标端点（Go 内置 expvar，零依赖）
	mux.Handle("/debug/vars", expvar.Handler())

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
	logger.Info("网关启动中", "addr", g.addr)
	return g.server.ListenAndServe()
}

// Shutdown 优雅关闭网关服务。
// 使用 http.Server.Shutdown 方法，在指定超时时间内等待所有连接处理完成，
// 超时后强制关闭。关闭后 Run 方法会返回 http.ErrServerClosed。
func (g *Gateway) Shutdown(ctx context.Context) error {
	logger.Info("网关正在关闭...")
	return g.server.Shutdown(ctx)
}
