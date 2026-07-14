package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// registerSettingsRoutes 装配站点设置相关路由。
func (a *API) registerSettingsRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/settings", a.auth(http.HandlerFunc(a.handleSettings)))
	// 公开：站点名与状态页背景（锁屏界面也需要，故不做密码拦截）。
	mux.HandleFunc("/api/public/site", a.handlePublicSite)
}

// 设置项的键名。
const (
	settingRepo          = "github_repo"           // GitHub 仓库 owner/name，用于生成一键安装命令
	settingPublicURL     = "public_ws_url"         // 面板对外 WS 地址，例如 ws://1.2.3.4:8008/api/agent
	settingAgentName     = "agent_name"            // Agent 实例名，多面板共存时用于区分 systemd 服务
	settingExpireEnabled = "expire_notify_enabled" // 到期 TG 提醒开关（"1"/"0"）
	settingExpireTime    = "expire_notify_time"    // 每日提醒时间，格式 HH:MM
	settingSiteName      = "site_name"             // 站点名称，显示在状态页标题
	settingStatusBG      = "status_background"     // 状态页背景图 URL，空 = 纯色背景
)

// handlePublicSite 返回状态页需要的站点外观信息。
func (a *API) handlePublicSite(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"site_name":         a.deps.Store.GetSetting(settingSiteName, "探针监控"),
		"status_background": a.deps.Store.GetSetting(settingStatusBG, ""),
	})
}

func (a *API) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"github_repo":             a.deps.Store.GetSetting(settingRepo, ""),
			"public_ws_url":           a.deps.Store.GetSetting(settingPublicURL, ""),
			"agent_name":              a.deps.Store.GetSetting(settingAgentName, ""),
			"expire_notify_enabled":   a.deps.Store.GetSetting(settingExpireEnabled, "0") == "1",
			"expire_notify_time":      a.deps.Store.GetSetting(settingExpireTime, "09:00"),
			"status_password_enabled": a.deps.Store.GetSetting(settingStatusPwEnabled, "0") == "1",
			"status_password_set":     a.deps.Store.GetSetting(settingStatusPwHash, "") != "",
			"site_name":               a.deps.Store.GetSetting(settingSiteName, ""),
			"status_background":       a.deps.Store.GetSetting(settingStatusBG, ""),
		})
	case http.MethodPut:
		var body struct {
			GithubRepo            string `json:"github_repo"`
			PublicWSURL           string `json:"public_ws_url"`
			AgentName             string `json:"agent_name"`
			ExpireNotifyEnabled   bool   `json:"expire_notify_enabled"`
			ExpireNotifyTime      string `json:"expire_notify_time"`
			StatusPasswordEnabled bool   `json:"status_password_enabled"`
			StatusPassword        string `json:"status_password"` // 仅在设置/修改时传，空 = 不改
			SiteName              string `json:"site_name"`
			StatusBackground      string `json:"status_background"`
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
		// 状态页密码：有新密码则更新哈希；开启时必须已有密码。
		if body.StatusPassword != "" {
			if len(body.StatusPassword) < 4 {
				writeError(w, http.StatusBadRequest, "状态页密码至少 4 位")
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(body.StatusPassword), bcrypt.DefaultCost)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := a.deps.Store.SetSetting(settingStatusPwHash, string(hash)); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if body.StatusPasswordEnabled && a.deps.Store.GetSetting(settingStatusPwHash, "") == "" {
			writeError(w, http.StatusBadRequest, "请先设置状态页访问密码再开启")
			return
		}
		boolStr := func(b bool) string {
			if b {
				return "1"
			}
			return "0"
		}
		pairs := map[string]string{
			settingRepo:            body.GithubRepo,
			settingPublicURL:       body.PublicWSURL,
			settingAgentName:       body.AgentName,
			settingExpireEnabled:   boolStr(body.ExpireNotifyEnabled),
			settingExpireTime:      body.ExpireNotifyTime,
			settingStatusPwEnabled: boolStr(body.StatusPasswordEnabled),
			settingSiteName:        body.SiteName,
			settingStatusBG:        body.StatusBackground,
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
