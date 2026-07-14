package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// publicServer 是状态页展示用的精简结构（隐藏 secret）。
type publicServer struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Online      bool   `json:"online"`
	LastSeen    int64  `json:"last_seen"`
	Host        any    `json:"host,omitempty"`
	Metrics     any    `json:"metrics,omitempty"`
	Note        string `json:"note,omitempty"`
	TrafficIn   uint64 `json:"traffic_in"`  // 当月入站累计字节
	TrafficOut  uint64 `json:"traffic_out"` // 当月出站累计字节
}

// handlePublicServers 返回状态页数据（合并 DB 登记与 Hub 实时态，过滤隐藏项）。
func (a *API) handlePublicServers(w http.ResponseWriter, r *http.Request) {
	servers, err := a.deps.Store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stateByID := map[uint64]any{}
	onlineByID := map[uint64]bool{}
	lastSeenByID := map[uint64]int64{}
	hostByID := map[uint64]any{}
	for _, st := range a.deps.Hub.Snapshot() {
		onlineByID[st.ServerID] = st.Online
		lastSeenByID[st.ServerID] = st.LastSeen.Unix()
		if st.Metrics != nil {
			stateByID[st.ServerID] = st.Metrics
		}
		if st.Host != nil {
			hostByID[st.ServerID] = st.Host
		}
	}
	ym := time.Now().Format("2006-01")
	out := make([]publicServer, 0, len(servers))
	for _, srv := range servers {
		if srv.Hidden {
			continue
		}
		inB, outB := a.deps.Store.TrafficMonth(srv.ID, ym)
		out = append(out, publicServer{
			ID:         srv.ID,
			Name:       srv.Name,
			Online:     onlineByID[srv.ID],
			LastSeen:   lastSeenByID[srv.ID],
			Host:       hostByID[srv.ID],
			Metrics:    stateByID[srv.ID],
			Note:       srv.Note,
			TrafficIn:  inB,
			TrafficOut: outB,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleHistory 返回单台服务器历史指标：/api/public/history?server_id=1&hours=6
func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("server_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server_id")
		return
	}
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 1
	}
	to := time.Now().Unix()
	from := to - int64(hours)*3600
	points, err := a.deps.Store.History(id, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

// handleServers 处理 /api/admin/servers 的 GET(列表) 与 POST(新增)。
func (a *API) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers, err := a.deps.Store.ListServers()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// 附带在线状态。
		online := map[uint64]bool{}
		for _, st := range a.deps.Hub.Snapshot() {
			online[st.ServerID] = st.Online
		}
		type row struct {
			ID        uint64 `json:"id"`
			Name      string `json:"name"`
			Secret    string `json:"secret"`
			Note      string `json:"note"`
			SortOrder int    `json:"sort_order"`
			Hidden    bool   `json:"hidden"`
			Online    bool   `json:"online"`
			CreatedAt int64  `json:"created_at"`
		}
		out := make([]row, 0, len(servers))
		for _, s := range servers {
			out = append(out, row{s.ID, s.Name, s.Secret, s.Note, s.SortOrder, s.Hidden, online[s.ID], s.CreatedAt})
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
			Note string `json:"note"`
		}
		if err := readJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeError(w, http.StatusBadRequest, "name required")
			return
		}
		srv, err := a.deps.Store.CreateServer(body.Name, body.Note)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, srv)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleServerItem 处理 /api/admin/servers/{id} 的 PUT(更新) 与 DELETE。
func (a *API) handleServerItem(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/servers/")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Name      string `json:"name"`
			Note      string `json:"note"`
			SortOrder int    `json:"sort_order"`
			Hidden    bool   `json:"hidden"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.deps.Store.UpdateServer(id, body.Name, body.Note, body.SortOrder, body.Hidden); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := a.deps.Store.DeleteServer(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
