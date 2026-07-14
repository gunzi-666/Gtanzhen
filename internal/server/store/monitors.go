package store

import "time"

// Monitor 服务监控项。
// Type：ping / tcping / http_get。Target：探测目标（地址或 URL）。
// ServerID：由哪台 Agent 发起探测。Interval：探测间隔（秒）。
type Monitor struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Target    string `json:"target"`
	ServerID  uint64 `json:"server_id"`
	Interval  int    `json:"interval"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
}

// ListMonitors 返回全部监控项。
func (s *Store) ListMonitors() ([]Monitor, error) {
	rows, err := s.db.Query(`SELECT id,name,type,target,server_id,interval,enabled,created_at FROM monitors ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Monitor{}
	for rows.Next() {
		var m Monitor
		var enabled int
		if err := rows.Scan(&m.ID, &m.Name, &m.Type, &m.Target, &m.ServerID, &m.Interval, &enabled, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Enabled = enabled == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateMonitor 新增监控项。
func (s *Store) CreateMonitor(m Monitor) (uint64, error) {
	if m.Interval <= 0 {
		m.Interval = 60
	}
	res, err := s.db.Exec(
		`INSERT INTO monitors(name,type,target,server_id,interval,enabled,created_at) VALUES(?,?,?,?,?,?,?)`,
		m.Name, m.Type, m.Target, m.ServerID, m.Interval, boolInt(m.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

// UpdateMonitor 更新监控项。
func (s *Store) UpdateMonitor(m Monitor) error {
	if m.Interval <= 0 {
		m.Interval = 60
	}
	res, err := s.db.Exec(
		`UPDATE monitors SET name=?,type=?,target=?,server_id=?,interval=?,enabled=? WHERE id=?`,
		m.Name, m.Type, m.Target, m.ServerID, m.Interval, boolInt(m.Enabled), m.ID,
	)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteMonitor 删除监控项及其结果。
func (s *Store) DeleteMonitor(id uint64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM monitors WHERE id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM monitor_results WHERE monitor_id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// MonitorResult 一次探测结果。
type MonitorResult struct {
	MonitorID uint64  `json:"monitor_id"`
	Ts        int64   `json:"ts"`
	Success   bool    `json:"success"`
	Delay     float64 `json:"delay"`
	Message   string  `json:"message"`
}

// AddMonitorResult 写入一次探测结果。
func (s *Store) AddMonitorResult(r MonitorResult) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO monitor_results(monitor_id,ts,success,delay,message) VALUES(?,?,?,?,?)`,
		r.MonitorID, r.Ts, boolInt(r.Success), r.Delay, r.Message,
	)
	return err
}

// MonitorResults 返回某监控项最近 limit 条结果（按时间升序）。
func (s *Store) MonitorResults(monitorID uint64, limit int) ([]MonitorResult, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT ts,success,delay,message FROM (
			SELECT ts,success,delay,message FROM monitor_results WHERE monitor_id=? ORDER BY ts DESC LIMIT ?
		) ORDER BY ts ASC`,
		monitorID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MonitorResult{}
	for rows.Next() {
		r := MonitorResult{MonitorID: monitorID}
		var success int
		if err := rows.Scan(&r.Ts, &success, &r.Delay, &r.Message); err != nil {
			return nil, err
		}
		r.Success = success == 1
		out = append(out, r)
	}
	return out, rows.Err()
}
