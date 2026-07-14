package store

import (
	"strconv"
	"strings"
	"time"
)

// AlertRule 告警规则。
// Metric 可选：cpu / mem / disk / load1 / net_in / net_out / offline。
// Operator：gt / lt。Duration：持续多少秒才触发（滑动窗口）。
// ServerIDs 为空表示对所有服务器生效，否则为逗号分隔的 id 列表。
type AlertRule struct {
	ID             uint64  `json:"id"`
	Name           string  `json:"name"`
	Metric         string  `json:"metric"`
	Operator       string  `json:"operator"`
	Threshold      float64 `json:"threshold"`
	Duration       int     `json:"duration"`
	ServerIDs      string  `json:"server_ids"`
	NotificationID uint64  `json:"notification_id"`
	Enabled        bool    `json:"enabled"`
	CreatedAt      int64   `json:"created_at"`
}

// Matches 判断规则是否作用于给定服务器。
func (r *AlertRule) Matches(serverID uint64) bool {
	if strings.TrimSpace(r.ServerIDs) == "" {
		return true
	}
	target := strconv.FormatUint(serverID, 10)
	for _, part := range strings.Split(r.ServerIDs, ",") {
		if strings.TrimSpace(part) == target {
			return true
		}
	}
	return false
}

// ListAlertRules 返回全部规则。
func (s *Store) ListAlertRules() ([]AlertRule, error) {
	rows, err := s.db.Query(`SELECT id,name,metric,operator,threshold,duration,server_ids,notification_id,enabled,created_at FROM alert_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertRule{}
	for rows.Next() {
		var r AlertRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Metric, &r.Operator, &r.Threshold, &r.Duration, &r.ServerIDs, &r.NotificationID, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateAlertRule 新增规则。
func (s *Store) CreateAlertRule(r AlertRule) (uint64, error) {
	res, err := s.db.Exec(
		`INSERT INTO alert_rules(name,metric,operator,threshold,duration,server_ids,notification_id,enabled,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		r.Name, r.Metric, r.Operator, r.Threshold, r.Duration, r.ServerIDs, r.NotificationID, boolInt(r.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

// UpdateAlertRule 更新规则。
func (s *Store) UpdateAlertRule(r AlertRule) error {
	res, err := s.db.Exec(
		`UPDATE alert_rules SET name=?,metric=?,operator=?,threshold=?,duration=?,server_ids=?,notification_id=?,enabled=? WHERE id=?`,
		r.Name, r.Metric, r.Operator, r.Threshold, r.Duration, r.ServerIDs, r.NotificationID, boolInt(r.Enabled), r.ID,
	)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteAlertRule 删除规则。
func (s *Store) DeleteAlertRule(id uint64) error {
	res, err := s.db.Exec(`DELETE FROM alert_rules WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

// AlertEvent 告警事件记录。
type AlertEvent struct {
	ID        uint64 `json:"id"`
	RuleID    uint64 `json:"rule_id"`
	ServerID  uint64 `json:"server_id"`
	State     string `json:"state"` // triggered / resolved
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

// AddAlertEvent 写入一条告警事件。
func (s *Store) AddAlertEvent(e AlertEvent) error {
	_, err := s.db.Exec(
		`INSERT INTO alert_events(rule_id,server_id,state,message,created_at) VALUES(?,?,?,?,?)`,
		e.RuleID, e.ServerID, e.State, e.Message, time.Now().Unix(),
	)
	return err
}

// ListAlertEvents 返回最近的告警事件。
func (s *Store) ListAlertEvents(limit int) ([]AlertEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id,rule_id,server_id,state,message,created_at FROM alert_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlertEvent{}
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.RuleID, &e.ServerID, &e.State, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
