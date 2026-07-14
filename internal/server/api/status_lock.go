package api

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// 状态页访问密码相关设置键。
const (
	settingStatusPwEnabled = "status_password_enabled" // "1" 开启
	settingStatusPwHash    = "status_password_hash"    // bcrypt 哈希
)

const statusCookie = "probe_status"

// statusLimiter 状态页解锁的失败限速（与后台登录限速相互独立）。
var statusLimiter loginLimiter

// statusLockEnabled 状态页是否开启了密码访问。
func (a *API) statusLockEnabled() bool {
	return a.deps.Store.GetSetting(settingStatusPwEnabled, "0") == "1" &&
		a.deps.Store.GetSetting(settingStatusPwHash, "") != ""
}

// statusUnlocked 请求是否有权访问公开数据：未开启密码、持有解锁 cookie、或已登录后台。
func (a *API) statusUnlocked(r *http.Request) bool {
	if !a.statusLockEnabled() {
		return true
	}
	if c, err := r.Cookie(statusCookie); err == nil && a.statusSessions.valid(c.Value) {
		return true
	}
	if c, err := r.Cookie(sessionCookie); err == nil && a.sessions.valid(c.Value) {
		return true
	}
	return false
}

// requireStatusAccess 公开接口的守卫；未解锁时写 401 并返回 false。
func (a *API) requireStatusAccess(w http.ResponseWriter, r *http.Request) bool {
	if a.statusUnlocked(r) {
		return true
	}
	writeError(w, http.StatusUnauthorized, "status_locked")
	return false
}

// handleStatusUnlock 处理 POST /api/public/unlock：校验状态页密码并发放 cookie。
func (a *API) handleStatusUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.statusLockEnabled() {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if statusLimiter.blocked() {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	hash := a.deps.Store.GetSetting(settingStatusPwHash, "")
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		statusLimiter.fail()
		writeError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	statusLimiter.reset()
	tok := a.statusSessions.create()
	http.SetCookie(w, &http.Cookie{
		Name:     statusCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
