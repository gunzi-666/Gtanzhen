package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// publicHost 是对公开状态页暴露的主机信息子集。
// 刻意不包含 hostname、内核版本、系统小版本、虚拟化等敏感/无用细节。
type publicHost struct {
	Platform  string   `json:"platform"`
	Arch      string   `json:"arch"`
	CPU       []string `json:"cpu,omitempty"`
	MemTotal  uint64   `json:"mem_total"`
	DiskTotal uint64   `json:"disk_total"`
}

// publicMetrics 是对公开状态页暴露的实时指标子集。
type publicMetrics struct {
	CPU          float64 `json:"cpu"`
	MemUsed      uint64  `json:"mem_used"`
	DiskUsed     uint64  `json:"disk_used"`
	NetInSpeed   uint64  `json:"net_in_speed"`
	NetOutSpeed  uint64  `json:"net_out_speed"`
	Load1        float64 `json:"load1"`
	Uptime       uint64  `json:"uptime"`
	ProcessCount uint64  `json:"process_count"`
	TCPConnCount uint64  `json:"tcp_conn_count"`
}

// publicServer 是状态页展示用的精简结构（不含 secret / note 等管理字段）。
type publicServer struct {
	ID         uint64         `json:"id"`
	Name       string         `json:"name"`
	Online     bool           `json:"online"`
	LastSeen   int64          `json:"last_seen"`
	Tags       []string       `json:"tags"`
	Group      string         `json:"group"`
	Host       *publicHost    `json:"host,omitempty"`
	Metrics    *publicMetrics `json:"metrics,omitempty"`
	TrafficIn  uint64         `json:"traffic_in"`  // 当月入站累计字节
	TrafficOut uint64         `json:"traffic_out"` // 当月出站累计字节
}

// publicServers 构建公开状态数据（合并 DB 登记与 Hub 实时态，过滤隐藏项）。
// REST 接口与浏览器 WebSocket 推送共用，保证两条通道暴露的字段一致。
func (a *API) publicServers() ([]publicServer, error) {
	servers, err := a.deps.Store.ListServers()
	if err != nil {
		return nil, err
	}
	metricsByID := map[uint64]*publicMetrics{}
	onlineByID := map[uint64]bool{}
	lastSeenByID := map[uint64]int64{}
	hostByID := map[uint64]*publicHost{}
	for _, st := range a.deps.Hub.Snapshot() {
		onlineByID[st.ServerID] = st.Online
		lastSeenByID[st.ServerID] = st.LastSeen.Unix()
		if m := st.Metrics; m != nil {
			metricsByID[st.ServerID] = &publicMetrics{
				CPU:          m.CPU,
				MemUsed:      m.MemUsed,
				DiskUsed:     m.DiskUsed,
				NetInSpeed:   m.NetInSpeed,
				NetOutSpeed:  m.NetOutSpeed,
				Load1:        m.Load1,
				Uptime:       m.Uptime,
				ProcessCount: m.ProcessCount,
				TCPConnCount: m.TCPConnCount,
			}
		}
		if h := st.Host; h != nil {
			hostByID[st.ServerID] = &publicHost{
				Platform:  h.Platform,
				Arch:      h.Arch,
				CPU:       h.CPU,
				MemTotal:  h.MemTotal,
				DiskTotal: h.DiskTotal,
			}
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
			Tags:       srv.Tags,
			Group:      srv.Group,
			Host:       hostByID[srv.ID],
			Metrics:    metricsByID[srv.ID],
			TrafficIn:  inB,
			TrafficOut: outB,
		})
	}
	return out, nil
}

// handlePublicServers 返回状态页数据。
func (a *API) handlePublicServers(w http.ResponseWriter, r *http.Request) {
	if !a.requireStatusAccess(w, r) {
		return
	}
	out, err := a.publicServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleHistory 返回单台服务器历史指标：/api/public/history?server_id=1&hours=6
func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	if !a.requireStatusAccess(w, r) {
		return
	}
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
		// 附带在线状态与 Agent 版本。
		online := map[uint64]bool{}
		version := map[uint64]string{}
		for _, st := range a.deps.Hub.Snapshot() {
			online[st.ServerID] = st.Online
			version[st.ServerID] = st.AgentVersion
		}
		type row struct {
			ID           uint64   `json:"id"`
			Name         string   `json:"name"`
			Secret       string   `json:"secret"`
			Note         string   `json:"note"`
			SortOrder    int      `json:"sort_order"`
			Hidden       bool     `json:"hidden"`
			Online       bool     `json:"online"`
			AgentVersion string   `json:"agent_version"`
			ExpiresAt    int64    `json:"expires_at"`
			Tags         []string `json:"tags"`
			Group        string   `json:"group"`
			CreatedAt    int64    `json:"created_at"`
		}
		out := make([]row, 0, len(servers))
		for _, s := range servers {
			out = append(out, row{s.ID, s.Name, s.Secret, s.Note, s.SortOrder, s.Hidden, online[s.ID], version[s.ID], s.ExpiresAt, s.Tags, s.Group, s.CreatedAt})
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

// handleServerItem 处理 /api/admin/servers/{id} 的 PUT(更新) 与 DELETE，
// 以及 /api/admin/servers/{id}/upgrade 的 POST(Agent 自升级)。
func (a *API) handleServerItem(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/servers/")
	if rest, found := strings.CutSuffix(idStr, "/upgrade"); found {
		id, err := strconv.ParseUint(rest, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		a.handleAgentUpgrade(w, r, id)
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body struct {
			Name      string   `json:"name"`
			Note      string   `json:"note"`
			SortOrder int      `json:"sort_order"`
			Hidden    bool     `json:"hidden"`
			ExpiresAt int64    `json:"expires_at"`
			Tags      []string `json:"tags"`
			Group     string   `json:"group"`
		}
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		if err := a.deps.Store.UpdateServer(id, body.Name, body.Note, body.SortOrder, body.Hidden, body.ExpiresAt, body.Tags, body.Group); err != nil {
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

// handleAgentUpgrade 向指定服务器下发 Agent 自升级任务。
// 下载地址根据设置中的 GitHub 仓库与该机上报的 os/arch 自动拼出（latest release）。
func (a *API) handleAgentUpgrade(w http.ResponseWriter, r *http.Request, id uint64) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	repo := a.deps.Store.GetSetting(settingRepo, "")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "请先在设置页配置 GitHub 仓库（owner/name）")
		return
	}
	st, ok := a.deps.Hub.StateOf(id)
	if !ok || !st.Online {
		writeError(w, http.StatusConflict, "服务器不在线，无法升级")
		return
	}
	if st.Host == nil {
		writeError(w, http.StatusConflict, "尚未收到该服务器的主机信息，请稍后重试")
		return
	}
	if latest := a.latestAgentVersion(); latest != "" && st.AgentVersion == latest {
		writeError(w, http.StatusConflict, "Agent 已是最新版本 "+latest+"，无需升级")
		return
	}
	bin := fmt.Sprintf("probe-agent-%s-%s", st.Host.OS, st.Host.Arch)
	if st.Host.OS == "windows" {
		bin += ".exe"
	}
	url := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, bin)

	srvName := st.Name
	a.notifyTG("Agent 升级提醒",
		"面板向服务器「"+srvName+"」下发了 Agent 升级任务。\n时间："+time.Now().Format("2006-01-02 15:04:05")+
			"\n来源 IP："+clientIP(r))

	output, err := a.deps.Dispatcher.Upgrade(id, url, 180)
	if err != nil {
		msg := err.Error()
		if output != "" {
			msg += "：" + output
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": output})
}
