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
)

func (a *API) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{
			"github_repo":   a.deps.Store.GetSetting(settingRepo, ""),
			"public_ws_url": a.deps.Store.GetSetting(settingPublicURL, ""),
		})
	case http.MethodPut:
		var body struct {
			GithubRepo  string `json:"github_repo"`
			PublicWSURL string `json:"public_ws_url"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.deps.Store.SetSetting(settingRepo, body.GithubRepo); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := a.deps.Store.SetSetting(settingPublicURL, body.PublicWSURL); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
