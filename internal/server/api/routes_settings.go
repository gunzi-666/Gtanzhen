package api

import (
	"net/http"
	"time"
)

// registerSettingsRoutes 装配站点设置相关路由。
func (a *API) registerSettingsRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/settings", a.auth(http.HandlerFunc(a.handleSettings)))
}

// 设置项的键名。
const (
	settingRepo          = "github_repo"           // GitHub 仓库 owner/name，用于生成一键安装命令
	settingPublicURL     = "public_ws_url"         // 面板对外 WS 地址，例如 ws://1.2.3.4:8008/api/agent
	settingAgentName     = "agent_name"            // Agent 实例名，多面板共存时用于区分 systemd 服务
	settingExpireEnabled = "expire_notify_enabled" // 到期 TG 提醒开关（"1"/"0"）
	settingExpireTime    = "expire_notify_time"    // 每日提醒时间，格式 HH:MM
)

func (a *API) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"github_repo":           a.deps.Store.GetSetting(settingRepo, ""),
			"public_ws_url":         a.deps.Store.GetSetting(settingPublicURL, ""),
			"agent_name":            a.deps.Store.GetSetting(settingAgentName, ""),
			"expire_notify_enabled": a.deps.Store.GetSetting(settingExpireEnabled, "0") == "1",
			"expire_notify_time":    a.deps.Store.GetSetting(settingExpireTime, "09:00"),
		})
	case http.MethodPut:
		var body struct {
			GithubRepo          string `json:"github_repo"`
			PublicWSURL         string `json:"public_ws_url"`
			AgentName           string `json:"agent_name"`
			ExpireNotifyEnabled bool   `json:"expire_notify_enabled"`
			ExpireNotifyTime    string `json:"expire_notify_time"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if body.ExpireNotifyEnabled && !a.tgBound() {
			writeError(w, http.StatusPreconditionFailed, "开启到期提醒前请先绑定 Telegram Bot")
			return
		}
		if body.ExpireNotifyTime == "" {
			body.ExpireNotifyTime = "09:00"
		}
		if _, err := time.Parse("15:04", body.ExpireNotifyTime); err != nil {
			writeError(w, http.StatusBadRequest, "提醒时间格式应为 HH:MM")
			return
		}
		enabled := "0"
		if body.ExpireNotifyEnabled {
			enabled = "1"
		}
		pairs := map[string]string{
			settingRepo:          body.GithubRepo,
			settingPublicURL:     body.PublicWSURL,
			settingAgentName:     body.AgentName,
			settingExpireEnabled: enabled,
			settingExpireTime:    body.ExpireNotifyTime,
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
