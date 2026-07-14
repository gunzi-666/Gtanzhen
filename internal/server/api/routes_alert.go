package api

import (
	"net/http"
	"strconv"
	"strings"

	"probe/internal/server/alert"
	"probe/internal/server/store"
)

// registerAlertRoutes 装配告警规则、告警事件、通知渠道相关路由（均需登录）。
func (a *API) registerAlertRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/alerts", a.auth(http.HandlerFunc(a.handleAlerts)))
	mux.Handle("/api/admin/alerts/", a.auth(http.HandlerFunc(a.handleAlertItem)))
	mux.Handle("/api/admin/alert-events", a.auth(http.HandlerFunc(a.handleAlertEvents)))

	mux.Handle("/api/admin/notifications", a.auth(http.HandlerFunc(a.handleNotifications)))
	mux.Handle("/api/admin/notifications/", a.auth(http.HandlerFunc(a.handleNotificationItem)))
}

func (a *API) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := a.deps.Store.ListAlertRules()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rules)
	case http.MethodPost:
		var rule store.AlertRule
		if err := readJSON(r, &rule); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		id, err := a.deps.Store.CreateAlertRule(rule)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]uint64{"id": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleAlertItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/api/admin/alerts/"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var rule store.AlertRule
		if err := readJSON(r, &rule); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		rule.ID = id
		if err := a.deps.Store.UpdateAlertRule(rule); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := a.deps.Store.DeleteAlertRule(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := a.deps.Store.ListAlertEvents(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *API) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.deps.Store.ListNotifications()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var n store.Notification
		if err := readJSON(r, &n); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		id, err := a.deps.Store.CreateNotification(n)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]uint64{"id": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleNotificationItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/notifications/")
	// 支持 /api/admin/notifications/{id}/test 触发测试发送。
	if strings.HasSuffix(rest, "/test") {
		idStr := strings.TrimSuffix(rest, "/test")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		a.testNotification(w, id)
		return
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var n store.Notification
		if err := readJSON(r, &n); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		n.ID = id
		if err := a.deps.Store.UpdateNotification(n); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := a.deps.Store.DeleteNotification(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// testNotification 立即向指定渠道发送一条测试消息。
func (a *API) testNotification(w http.ResponseWriter, id uint64) {
	n, err := a.deps.Store.GetNotification(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	sender, err := alert.BuildSender(n)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sender.Send("[探针测试]", "这是一条来自探针面板的测试通知。"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
