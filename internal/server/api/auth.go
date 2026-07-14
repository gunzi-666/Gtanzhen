package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
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
	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(a.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(body.Password), []byte(a.adminPass)) == 1
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
