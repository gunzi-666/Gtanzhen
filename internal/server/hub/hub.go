// Package hub 管理所有 Agent 的 WebSocket 连接、认证、心跳与实时状态。
package hub

import (
	"sync"
	"time"

	"probe/internal/protocol"
)

// State 是单台服务器在内存中的实时状态。
type State struct {
	ServerID     uint64             `json:"server_id"`
	Name         string             `json:"name"`
	Online       bool               `json:"online"`
	LastSeen     time.Time          `json:"last_seen"`
	AgentVersion string             `json:"agent_version,omitempty"`
	IP           string             `json:"-"` // Agent 连接来源 IP，仅供管理后台，不随公开接口输出
	Host         *protocol.HostInfo `json:"host,omitempty"`
	Metrics      *protocol.Metrics  `json:"metrics,omitempty"`
	connID       uint64             // 当前占用连接的自增 id，用于处理重复连接

	// notifiedOffline 标记是否已发过离线告警。
	// 断连后立即置 Online=false（界面即时反馈），但告警要等宽限期后
	// 仍未回连才发；快速重启（升级/网络抖动）不打扰。
	notifiedOffline bool
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
	onOnline      OnlineHandler
	onConnect     ConnectHandler
	onHostIP      HostIPHandler
	reportPeriod  int
	offlineAfter  time.Duration
}

// ConnectHandler 在每次 Agent 成功接入时回调（含重连），用于持久化来源 IP 等。
type ConnectHandler func(serverID uint64, ip string)

// HostIPHandler 在收到 Agent 自测的公网 IPv4/IPv6 时回调（空串表示对应协议不可用）。
type HostIPHandler func(serverID uint64, ipv4, ipv6 string)

// OnlineHandler 处理服务器上线事件。
// fresh 为 true 表示该机在面板内存中没有历史状态（首次连接或面板刚重启），
// 是否算“首次上线”由上层结合持久化记录判断。
type OnlineHandler func(serverID uint64, fresh bool)

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

// SetOnlineHandler 注册上线回调（离线→在线的边沿或首次连接时触发）。
func (h *Hub) SetOnlineHandler(fn OnlineHandler) { h.onOnline = fn }

// SetConnectHandler 注册接入回调（每次连接建立都触发）。
func (h *Hub) SetConnectHandler(fn ConnectHandler) { h.onConnect = fn }

// SetHostIPHandler 注册公网 IP 上报回调。
func (h *Hub) SetHostIPHandler(fn HostIPHandler) { h.onHostIP = fn }

// fireOnline 在异步 goroutine 中触发上线回调，调用方可持锁调用。
func (h *Hub) fireOnline(id uint64, fresh bool) {
	if h.onOnline != nil {
		go h.onOnline(id, fresh)
	}
}

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

// markOffline 后台巡检找出需要发离线告警的服务器：
// 无论是超时未上报（连接还挂着）还是已主动断连，只要距最后一次
// 活动超过宽限期且尚未告警过，就标记并返回。
func (h *Hub) markOffline() []uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var changed []uint64
	now := time.Now()
	for id, s := range h.states {
		if s.notifiedOffline || now.Sub(s.LastSeen) <= h.offlineAfter {
			continue
		}
		s.Online = false
		s.notifiedOffline = true
		changed = append(changed, id)
	}
	return changed
}

// OfflineWatcher 启动离线巡检，onOffline 在服务器确认离线（超过宽限期）时回调。
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
