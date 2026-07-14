// Package hub 管理所有 Agent 的 WebSocket 连接、认证、心跳与实时状态。
package hub

import (
	"sync"
	"time"

	"probe/internal/protocol"
)

// State 是单台服务器在内存中的实时状态。
type State struct {
	ServerID   uint64             `json:"server_id"`
	Name       string             `json:"name"`
	Online     bool               `json:"online"`
	LastSeen   time.Time          `json:"last_seen"`
	Host       *protocol.HostInfo `json:"host,omitempty"`
	Metrics    *protocol.Metrics  `json:"metrics,omitempty"`
	connID     uint64             // 当前占用连接的自增 id，用于处理重复连接
}

// TaskResultHandler 处理 Agent 回报的任务结果。
type TaskResultHandler func(serverID uint64, res protocol.TaskResult)

// MetricsHandler 处理 Agent 上报的指标（用于落库/告警）。
type MetricsHandler func(serverID uint64, m protocol.Metrics)

// Authenticator 根据 secret 返回服务器 id 与名称，失败返回 ok=false。
type Authenticator func(secret string) (id uint64, name string, ok bool)

// Hub 是连接中枢。
type Hub struct {
	mu       sync.RWMutex
	conns    map[uint64]*conn // serverID -> 连接
	states   map[uint64]*State
	connSeq  uint64

	auth          Authenticator
	onMetrics     MetricsHandler
	onTaskResult  TaskResultHandler
	reportPeriod  int
	offlineAfter  time.Duration
}

// New 创建 Hub。
func New(auth Authenticator, reportPeriod int) *Hub {
	return &Hub{
		conns:        make(map[uint64]*conn),
		states:       make(map[uint64]*State),
		auth:         auth,
		reportPeriod: reportPeriod,
		offlineAfter: 30 * time.Second,
	}
}

// SetMetricsHandler 注册指标回调。
func (h *Hub) SetMetricsHandler(fn MetricsHandler) { h.onMetrics = fn }

// SetTaskResultHandler 注册任务结果回调。
func (h *Hub) SetTaskResultHandler(fn TaskResultHandler) { h.onTaskResult = fn }

// Snapshot 返回所有服务器当前状态的拷贝，供 API 读取。
func (h *Hub) Snapshot() []State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]State, 0, len(h.states))
	for _, s := range h.states {
		cp := *s
		out = append(out, cp)
	}
	return out
}

// StateOf 返回单台服务器状态拷贝。
func (h *Hub) StateOf(id uint64) (State, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.states[id]
	if !ok {
		return State{}, false
	}
	return *s, true
}

// Dispatch 向指定服务器下发任务，返回是否成功送达。
func (h *Hub) Dispatch(serverID uint64, task protocol.TaskDispatch) bool {
	h.mu.RLock()
	c, ok := h.conns[serverID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	return c.sendTask(task) == nil
}

// IsOnline 报告服务器是否在线。
func (h *Hub) IsOnline(serverID uint64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.states[serverID]
	return ok && s.Online
}

// markOffline 后台巡检把超时未上报的服务器标记为离线。
// 返回本轮新转为离线的服务器 id 列表。
func (h *Hub) markOffline() []uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var changed []uint64
	now := time.Now()
	for id, s := range h.states {
		if s.Online && now.Sub(s.LastSeen) > h.offlineAfter {
			s.Online = false
			changed = append(changed, id)
		}
	}
	return changed
}

// OfflineWatcher 启动离线巡检，onOffline 在服务器转离线时回调。
func (h *Hub) OfflineWatcher(onOffline func(serverID uint64)) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			for _, id := range h.markOffline() {
				if onOffline != nil {
					onOffline(id)
				}
			}
		}
	}()
}
