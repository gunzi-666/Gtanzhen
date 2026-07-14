package alert

import (
	"fmt"
	"log"
	"sync"
	"time"

	"probe/internal/protocol"
	"probe/internal/server/hub"
	"probe/internal/server/store"
)

// ruleState 记录单个 (规则,服务器) 的运行状态。
// 一次持续突破只在跨过 Duration 阈值时通知一次，恢复时再通知一次，
// triggered 标志天然避免了重复轰炸，无需额外静默期。
type ruleState struct {
	condSince time.Time // 条件持续为真的起始时刻；零值表示当前为假
	triggered bool      // 是否处于已触发状态
}

// Engine 是告警引擎。
type Engine struct {
	store *store.Store
	hub   *hub.Hub

	mu     sync.Mutex
	states map[string]*ruleState // key: ruleID:serverID
}

// NewEngine 创建告警引擎。
func NewEngine(s *store.Store, h *hub.Hub) *Engine {
	return &Engine{store: s, hub: h, states: make(map[string]*ruleState)}
}

func key(ruleID, serverID uint64) string {
	return fmt.Sprintf("%d:%d", ruleID, serverID)
}

// OnMetrics 每次收到某服务器指标时评估其相关规则。
func (e *Engine) OnMetrics(serverID uint64, m protocol.Metrics) {
	rules, err := e.store.ListAlertRules()
	if err != nil {
		return
	}
	st, _ := e.hub.StateOf(serverID)
	for _, r := range rules {
		if !r.Enabled || r.Metric == "offline" || r.Metric == "online" || !r.Matches(serverID) {
			continue
		}
		value, ok := metricValue(r.Metric, m, st.Host)
		if !ok {
			continue
		}
		e.evaluate(r, serverID, st.Name, condTrue(r.Operator, value, r.Threshold), value)
	}
}

// OnOffline 服务器转离线时评估 offline 规则。
func (e *Engine) OnOffline(serverID uint64) {
	rules, err := e.store.ListAlertRules()
	if err != nil {
		return
	}
	st, _ := e.hub.StateOf(serverID)
	for _, r := range rules {
		if !r.Enabled || r.Metric != "offline" || !r.Matches(serverID) {
			continue
		}
		msg := fmt.Sprintf("服务器 %s 已离线", nameOr(st.Name, serverID))
		e.fire(r, serverID, "triggered", msg)
	}
}

// OnOnline 服务器由离线转在线时评估 online 规则。
func (e *Engine) OnOnline(serverID uint64) {
	rules, err := e.store.ListAlertRules()
	if err != nil {
		return
	}
	st, _ := e.hub.StateOf(serverID)
	for _, r := range rules {
		if !r.Enabled || r.Metric != "online" || !r.Matches(serverID) {
			continue
		}
		msg := fmt.Sprintf("服务器 %s 已上线", nameOr(st.Name, serverID))
		e.fire(r, serverID, "resolved", msg)
	}
}

// evaluate 推进单条规则的状态机。
func (e *Engine) evaluate(r store.AlertRule, serverID uint64, name string, condMet bool, value float64) {
	k := key(r.ID, serverID)
	e.mu.Lock()
	s, ok := e.states[k]
	if !ok {
		s = &ruleState{}
		e.states[k] = s
	}
	now := time.Now()

	if condMet {
		if s.condSince.IsZero() {
			s.condSince = now
		}
		held := now.Sub(s.condSince)
		if !s.triggered && held >= time.Duration(r.Duration)*time.Second {
			s.triggered = true
			e.mu.Unlock()
			msg := fmt.Sprintf("服务器 %s 的 %s 达到 %.1f（阈值 %s %.1f，持续 %d 秒）",
				nameOr(name, serverID), metricLabel(r.Metric), value, r.Operator, r.Threshold, r.Duration)
			e.fire(r, serverID, "triggered", msg)
			return
		}
	} else {
		wasTriggered := s.triggered
		s.condSince = time.Time{}
		s.triggered = false
		if wasTriggered {
			e.mu.Unlock()
			msg := fmt.Sprintf("服务器 %s 的 %s 已恢复（当前 %.1f）", nameOr(name, serverID), metricLabel(r.Metric), value)
			e.fire(r, serverID, "resolved", msg)
			return
		}
	}
	e.mu.Unlock()
}

// fire 记录事件并按需发送通知。
func (e *Engine) fire(r store.AlertRule, serverID uint64, state, msg string) {
	_ = e.store.AddAlertEvent(store.AlertEvent{RuleID: r.ID, ServerID: serverID, State: state, Message: msg})
	log.Printf("[alert] %s: %s", state, msg)

	if r.NotificationID == 0 {
		return
	}
	n, err := e.store.GetNotification(r.NotificationID)
	if err != nil || !n.Enabled {
		return
	}
	sender, err := BuildSender(n)
	if err != nil {
		log.Printf("[alert] build sender: %v", err)
		return
	}
	title := "[探针告警] " + r.Name
	if state == "resolved" {
		title = "[探针恢复] " + r.Name
	}
	go func() {
		if err := sender.Send(title, msg); err != nil {
			log.Printf("[alert] send notify: %v", err)
		}
	}()
}

// metricValue 从指标中提取规则关注的数值。百分比类返回 0-100。
func metricValue(metric string, m protocol.Metrics, host *protocol.HostInfo) (float64, bool) {
	switch metric {
	case "cpu":
		return m.CPU, true
	case "mem":
		if host == nil || host.MemTotal == 0 {
			return 0, false
		}
		return float64(m.MemUsed) / float64(host.MemTotal) * 100, true
	case "disk":
		if host == nil || host.DiskTotal == 0 {
			return 0, false
		}
		return float64(m.DiskUsed) / float64(host.DiskTotal) * 100, true
	case "load1":
		return m.Load1, true
	case "net_in":
		return float64(m.NetInSpeed), true
	case "net_out":
		return float64(m.NetOutSpeed), true
	}
	return 0, false
}

func metricLabel(metric string) string {
	switch metric {
	case "cpu":
		return "CPU 使用率(%)"
	case "mem":
		return "内存使用率(%)"
	case "disk":
		return "磁盘使用率(%)"
	case "load1":
		return "1 分钟负载"
	case "net_in":
		return "入站速率(B/s)"
	case "net_out":
		return "出站速率(B/s)"
	}
	return metric
}

func condTrue(op string, value, threshold float64) bool {
	if op == "lt" {
		return value < threshold
	}
	return value > threshold
}

func nameOr(name string, id uint64) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("#%d", id)
}
