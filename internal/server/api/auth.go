package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "probe_session"
const sessionTTL = 7 * 24 * time.Hour

// sessionStore 内存会话表。
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token -> 过期时间
}

func newSessionStore() *sessionStore {
	s := &sessionStore{sessions: make(map[string]time.Time)}
	go s.gc()
	return s
}

func (s *sessionStore) gc() {
	t := time.NewTicker(time.Hour)
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for tok, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, tok)
			}
		}
		s.mu.Unlock()
	}
}

func (s *sessionStore) create() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[tok] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return tok
}

func (s *sessionStore) valid(tok string) bool {
	s.mu.RLock()
	exp, ok := s.sessions[tok]
	s.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

func (s *sessionStore) revoke(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
}

// revokeAll 吊销所有会话（改密码后强制全部重新登录）。
func (s *sessionStore) revokeAll() {
	s.mu.Lock()
	s.sessions = make(map[string]time.Time)
	s.mu.Unlock()
}

// loginLimiter 简单的登录失败限速（按无差别全局计数）。
type loginLimiter struct {
	mu       sync.Mutex
	fails    int
	blockTil time.Time
}

var limiter loginLimiter

func (l *loginLimiter) blocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Now().Before(l.blockTil)
}

func (l *loginLimiter) fail() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails++
	if l.fails >= 5 {
		l.blockTil = time.Now().Add(time.Minute)
		l.fails = 0
	}
}

func (l *loginLimiter) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails = 0
}

// effectiveUser 当前生效的管理员用户名：后台改过则用数据库里的，否则用启动参数。
func (a *API) effectiveUser() string {
	if u := a.deps.Store.GetSetting(settingAdminUser, ""); u != "" {
		return u
	}
	return a.adminUser
}

// checkPassword 校验管理员密码：后台改过密码则用数据库里的 bcrypt 哈希，
// 否则退回启动参数 / 环境变量里的初始密码。
func (a *API) checkPassword(pass string) bool {
	if hash := a.deps.Store.GetSetting(settingPassHash, ""); hash != "" {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil
	}
	return subtle.ConstantTimeCompare([]byte(pass), []byte(a.adminPass)) == 1
}

// auth 是要求登录的中间件。
func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !a.sessions.valid(c.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if limiter.blocked() {
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(a.effectiveUser())) == 1
	passOK := a.checkPassword(body.Password)
	if !userOK || !passOK {
		limiter.fail()
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	limiter.reset()
	tok := a.sessions.create()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	// 已绑定 TG 时发送登录提醒。
	a.notifyTG("面板登录提醒",
		"管理员 "+body.Username+" 登录了面板后台。\n时间："+time.Now().Format("2006-01-02 15:04:05")+
			"\n来源 IP："+clientIP(r)+"\n\n若非本人操作，请立即修改密码！")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.sessions.revoke(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookie)
	loggedIn := err == nil && a.sessions.valid(c.Value)
	writeJSON(w, http.StatusOK, map[string]bool{"logged_in": loggedIn})
}
