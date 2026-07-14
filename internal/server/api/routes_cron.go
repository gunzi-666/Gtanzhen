package api

import (
	"net/http"
	"strconv"
	"strings"

	"probe/internal/server/store"
)

// CronReloader 允许在计划任务变更后即时重载调度。
type CronReloader interface {
	Reload()
}

// registerCronRoutes 装配计划任务与远程执行相关路由。
func (a *API) registerCronRoutes(mux *http.ServeMux) {
	mux.Handle("/api/admin/crons", a.auth(http.HandlerFunc(a.handleCrons)))
	mux.Handle("/api/admin/crons/", a.auth(http.HandlerFunc(a.handleCronItem)))
	mux.Handle("/api/admin/task-logs", a.auth(http.HandlerFunc(a.handleTaskLogs)))
	mux.Handle("/api/admin/exec", a.auth(http.HandlerFunc(a.handleExec)))
}

func (a *API) handleCrons(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.deps.Store.ListCronTasks()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var c store.CronTask
		if err := readJSON(r, &c); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		id, err := a.deps.Store.CreateCronTask(c)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.reloadCron()
		writeJSON(w, http.StatusOK, map[string]uint64{"id": id})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleCronItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/api/admin/crons/"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var c store.CronTask
		if err := readJSON(r, &c); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		c.ID = id
		if err := a.deps.Store.UpdateCronTask(c); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.reloadCron()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := a.deps.Store.DeleteCronTask(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.reloadCron()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	logs, err := a.deps.Store.ListTaskLogs(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// handleExec 立即在指定服务器执行一次命令并返回输出。
func (a *API) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if a.deps.Dispatcher == nil {
		writeError(w, http.StatusServiceUnavailable, "command dispatch unavailable")
		return
	}
	var body struct {
		ServerID uint64 `json:"server_id"`
		Command  string `json:"command"`
		Timeout  int    `json:"timeout"`
	}
	if err := readJSON(r, &body); err != nil || strings.TrimSpace(body.Command) == "" {
		writeError(w, http.StatusBadRequest, "server_id and command required")
		return
	}
	output, err := a.deps.Dispatcher.RunCommand(body.ServerID, body.Command, body.Timeout)
	logEntry := store.TaskLog{ServerID: body.ServerID, Success: err == nil, Output: output}
	resp := map[string]any{"output": output, "success": err == nil}
	if err != nil {
		resp["error"] = err.Error()
		if logEntry.Output != "" {
			logEntry.Output += "\n"
		}
		logEntry.Output += "错误: " + err.Error()
	}
	_ = a.deps.Store.AddTaskLog(logEntry)
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) reloadCron() {
	if a.deps.Cron != nil {
		a.deps.Cron.Reload()
	}
}
