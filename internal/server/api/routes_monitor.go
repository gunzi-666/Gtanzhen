package api

import (
	"net/http"
	"strconv"
	"strings"

	"probe/internal/server/store"
)

// registerMonitorRoutes 装配服务监控相关路由。
func (a *API) registerMonitorRoutes(mux *http.ServeMux) {
	// 公开：状态页展示监控可用性。
	mux.HandleFunc("/api/public/monitors", a.handlePublicMonitors)
	mux.HandleFunc("/api/public/monitor-results", a.handleMonitorResults)

	mux.Handle("/api/admin/monitors", a.auth(http.HandlerFunc(a.handleMonitors)))
	mux.Handle("/api/admin/monitors/", a.auth(http.HandlerFunc(a.handleMonitorItem)))
}

// handlePublicMonitors 返回监控项及其最近一次结果，供状态页展示。
func (a *API) handlePublicMonitors(w http.ResponseWriter, r *http.Request) {
	if !a.requireStatusAccess(w, r) {
		return
	}
	monitors, err := a.deps.Store.ListMonitors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type item struct {
		store.Monitor
		LastSuccess bool    `json:"last_success"`
		LastDelay   float64 `json:"last_delay"`
		LastMessage string  `json:"last_message"`
		HasResult   bool    `json:"has_result"`
	}
	out := make([]item, 0, len(monitors))
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}
		it := item{Monitor: m}
		if results, _ := a.deps.Store.MonitorResults(m.ID, 1); len(results) > 0 {
			last := results[len(results)-1]
			it.LastSuccess = last.Success
			it.LastDelay = last.Delay
			it.LastMessage = last.Message
			it.HasResult = true
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMonitorResults 返回单个监控项的历史结果。
func (a *API) handleMonitorResults(w http.ResponseWriter, r *http.Request) {
	if !a.requireStatusAccess(w, r) {
		return
	}
	id, err := strconv.ParseUint(r.URL.Query().Get("monitor_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid monitor_id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	results, err := a.deps.Store.MonitorResults(id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *API) handleMonitors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.deps.Store.ListMonitors()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var m store.Monitor
		if err := readJSON(r, &m); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		id, err := a.deps.Store.CreateMonitor(m)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]uint64{"id": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleMonitorItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/api/admin/monitors/"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var m store.Monitor
		if err := readJSON(r, &m); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		m.ID = id
		if err := a.deps.Store.UpdateMonitor(m); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := a.deps.Store.DeleteMonitor(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
