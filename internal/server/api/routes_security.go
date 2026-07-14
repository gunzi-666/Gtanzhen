package api

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"probe/internal/server/alert"
)

// 安全相关设置项的键名。
const (
	settingPassHash = "admin_pass_hash" // bcrypt 哈希，存在则覆盖启动参数密码
	settingTGToken  = "tg_bot_token"    // 绑定的 Telegram Bot Token
	settingTGChat   = "tg_chat_id"      // 绑定的 Telegram Chat ID
)

const codeTTL = 10 * time.Minute

// verifyCode 一条待校验的验证码。
type verifyCode struct {
	code     string
	expires  time.Time
	attempts int
	// 绑定场景暂存待绑定的 bot 信息，验证通过后才落库。
	pendingToken string
	pendingChat  string
}

// codeStore 按用途（bind / password）存放验证码，全局单管理员场景下无需按用户区分。
type codeStore struct {
	mu    sync.Mutex
	codes map[string]*verifyCode
}

var vcodes = codeStore{codes: map[string]*verifyCode{}}

func (cs *codeStore) put(purpose string, vc *verifyCode) {
	cs.mu.Lock()
	cs.codes[purpose] = vc
	cs.mu.Unlock()
}

// check 校验并在成功时消费验证码；连续 5 次失败或过期即作废。
func (cs *codeStore) check(purpose, code string) (*verifyCode, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	vc, ok := cs.codes[purpose]
	if !ok || time.Now().After(vc.expires) {
		delete(cs.codes, purpose)
		return nil, false
	}
	if subtle.ConstantTimeCompare([]byte(vc.code), []byte(code)) != 1 {
		vc.attempts++
		if vc.attempts >= 5 {
			delete(cs.codes, purpose)
		}
		return nil, false
	}
	delete(cs.codes, purpose)
	return vc, true
}

func genCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// registerSecurityRoutes 装配 TG 绑定与修改密码相关路由（均需登录）。
func (a *API) registerSecurityRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/security", a.auth(http.HandlerFunc(a.handleSecurityInfo)))
	mux.Handle("/api/admin/tg/bind/code", a.auth(http.HandlerFunc(a.handleTGBindCode)))
	mux.Handle("/api/admin/tg/bind", a.auth(http.HandlerFunc(a.handleTGBind)))
	mux.Handle("/api/admin/tg/unbind", a.auth(http.HandlerFunc(a.handleTGUnbind)))
	mux.Handle("/api/admin/password/code", a.auth(http.HandlerFunc(a.handlePasswordCode)))
	mux.Handle("/api/admin/password", a.auth(http.HandlerFunc(a.handlePasswordChange)))
}

func (a *API) tgBound() bool {
	return a.deps.Store.GetSetting(settingTGToken, "") != "" && a.deps.Store.GetSetting(settingTGChat, "") != ""
}

// GET /api/admin/security：当前安全状态。
func (a *API) handleSecurityInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := a.deps.Store.GetSetting(settingTGToken, "")
	masked := ""
	if len(token) > 8 {
		masked = token[:6] + "..." + token[len(token)-4:]
	} else if token != "" {
		masked = "已设置"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tg_bound":         a.tgBound(),
		"tg_chat_id":       a.deps.Store.GetSetting(settingTGChat, ""),
		"tg_token_masked":  masked,
		"password_changed": a.deps.Store.GetSetting(settingPassHash, "") != "",
	})
}

// POST /api/admin/tg/bind/code：用待绑定的 bot 发送验证码，验证通过才算绑定成功。
func (a *API) handleTGBindCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	body.BotToken = strings.TrimSpace(body.BotToken)
	body.ChatID = strings.TrimSpace(body.ChatID)
	if body.BotToken == "" || body.ChatID == "" {
		writeError(w, http.StatusBadRequest, "bot_token 和 chat_id 不能为空")
		return
	}
	code := genCode()
	if err := alert.SendTelegram(body.BotToken, body.ChatID,
		"探针面板绑定验证", "验证码："+code+"\n10 分钟内有效。若非本人操作请忽略。"); err != nil {
		writeError(w, http.StatusBadGateway, "发送失败，请检查 Token/Chat ID："+err.Error())
		return
	}
	vcodes.put("bind", &verifyCode{
		code: code, expires: time.Now().Add(codeTTL),
		pendingToken: body.BotToken, pendingChat: body.ChatID,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/admin/tg/bind：校验验证码并落库完成绑定。
func (a *API) handleTGBind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	vc, ok := vcodes.check("bind", strings.TrimSpace(body.Code))
	if !ok {
		writeError(w, http.StatusUnauthorized, "验证码错误或已过期")
		return
	}
	if err := a.deps.Store.SetSetting(settingTGToken, vc.pendingToken); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.deps.Store.SetSetting(settingTGChat, vc.pendingChat); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/admin/tg/unbind：解除绑定。
func (a *API) handleTGUnbind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_ = a.deps.Store.SetSetting(settingTGToken, "")
	_ = a.deps.Store.SetSetting(settingTGChat, "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/admin/password/code：向已绑定的 TG 发送改密验证码。
func (a *API) handlePasswordCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.tgBound() {
		writeError(w, http.StatusPreconditionFailed, "请先绑定 Telegram Bot")
		return
	}
	code := genCode()
	err := alert.SendTelegram(
		a.deps.Store.GetSetting(settingTGToken, ""),
		a.deps.Store.GetSetting(settingTGChat, ""),
		"探针面板修改密码验证", "验证码："+code+"\n10 分钟内有效。若非本人操作，请立即检查面板安全！")
	if err != nil {
		writeError(w, http.StatusBadGateway, "发送失败："+err.Error())
		return
	}
	vcodes.put("password", &verifyCode{code: code, expires: time.Now().Add(codeTTL)})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/admin/password：校验旧密码 + TG 验证码后更新密码，并吊销所有会话。
func (a *API) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.tgBound() {
		writeError(w, http.StatusPreconditionFailed, "请先绑定 Telegram Bot")
		return
	}
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
		Code        string `json:"code"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if len(body.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	if !a.checkPassword(body.OldPassword) {
		writeError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}
	if _, ok := vcodes.check("password", strings.TrimSpace(body.Code)); !ok {
		writeError(w, http.StatusUnauthorized, "验证码错误或已过期")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.deps.Store.SetSetting(settingPassHash, string(hash)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 改密后所有会话作废，强制重新登录。
	a.sessions.revokeAll()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
