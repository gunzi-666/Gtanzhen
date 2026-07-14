package api

import "net/http"

// registerSettingsRoutes 装配站点设置相关路由。
func (a *API) registerSettingsRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/settings", a.auth(http.HandlerFunc(a.handleSettings)))
}

// 设置项的键名。
const (
	settingRepo      = "github_repo"   // GitHub 仓库 owner/name，用于生成一键安装命令
	settingPublicURL = "public_ws_url" // 面板对外 WS 地址，例如 ws://1.2.3.4:8008/api/agent
	settingAgentName = "agent_name"    // Agent 实例名，多面板共存时用于区分 systemd 服务
)

func (a *API) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{
			"github_repo":   a.deps.Store.GetSetting(settingRepo, ""),
			"public_ws_url": a.deps.Store.GetSetting(settingPublicURL, ""),
			"agent_name":    a.deps.Store.GetSetting(settingAgentName, ""),
		})
	case http.MethodPut:
		var body struct {
			GithubRepo  string `json:"github_repo"`
			PublicWSURL string `json:"public_ws_url"`
			AgentName   string `json:"agent_name"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		pairs := map[string]string{
			settingRepo:      body.GithubRepo,
			settingPublicURL: body.PublicWSURL,
			settingAgentName: body.AgentName,
		}
		for k, v := range pairs {
			if err := a.deps.Store.SetSetting(k, v); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
