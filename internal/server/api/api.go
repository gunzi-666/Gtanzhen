// Package api 提供 REST 接口、浏览器实时推送 WebSocket，以及内嵌的前端静态资源。
package api

import (
	"net/http"

	"probe/internal/server/hub"
	"probe/internal/server/store"
)

// Deps 是 API 层依赖的其它组件集合。
type Deps struct {
	Hub   *hub.Hub
	Store *store.Store

	Dispatcher CommandDispatcher
	Cron       CronReloader
}

// CommandDispatcher 抽象“向某台服务器下发一次性命令并等待结果”的能力。
type CommandDispatcher interface {
	RunCommand(serverID uint64, command string, timeout int) (string, error)
}

// API 聚合所有 HTTP 处理器。
type API struct {
	deps      Deps
	sessions  *sessionStore
	pusher    *pusher
	adminUser string
	adminPass string
}

// New 创建 API。
func New(deps Deps, adminUser, adminPass string) *API {
	a := &API{
		deps:      deps,
		sessions:  newSessionStore(),
		adminUser: adminUser,
		adminPass: adminPass,
	}
	a.pusher = newPusher(deps.Hub)
	return a
}

// Routes 装配路由并返回 http.Handler。
// static 是内嵌前端的文件系统（可为 nil，此时只提供 API）。
func (a *API) Routes(static http.FileSystem) http.Handler {
	mux := http.NewServeMux()

	// Agent 接入。
	mux.HandleFunc("/api/agent", a.deps.Hub.HandleWS)

	// 浏览器实时推送。
	mux.HandleFunc("/api/ws", a.pusher.handle)

	// 公开接口（无需登录）。
	mux.HandleFunc("/api/public/servers", a.handlePublicServers)
	mux.HandleFunc("/api/public/history", a.handleHistory)

	// 认证。
	mux.HandleFunc("/api/login", a.handleLogin)
	mux.HandleFunc("/api/logout", a.handleLogout)
	mux.HandleFunc("/api/me", a.handleMe)

	// 管理接口（需登录）。
	mux.Handle("/api/admin/servers", a.auth(http.HandlerFunc(a.handleServers)))
	mux.Handle("/api/admin/servers/", a.auth(http.HandlerFunc(a.handleServerItem)))

	a.registerAlertRoutes(mux)
	a.registerMonitorRoutes(mux)
	a.registerCronRoutes(mux)
	a.registerSettingsRoutes(mux)
	a.registerSecurityRoutes(mux)

	// 前端静态资源（SPA fallback）。
	if static != nil {
		mux.Handle("/", spaHandler(static))
	}

	a.pusher.start()
	return withCORS(mux)
}
