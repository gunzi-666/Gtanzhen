package store

import "time"

// Notification 通知渠道配置。
// Type：telegram / email / webhook。Config 为该类型的 JSON 配置字符串。
type Notification struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Config    string `json:"config"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
}

// ListNotifications 返回全部通知渠道。
func (s *Store) ListNotifications() ([]Notification, error) {
	rows, err := s.db.Query(`SELECT id,name,type,config,enabled,created_at FROM notifications ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		var n Notification
		var enabled int
		if err := rows.Scan(&n.ID, &n.Name, &n.Type, &n.Config, &enabled, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Enabled = enabled == 1
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNotification 按 id 取通知渠道。
func (s *Store) GetNotification(id uint64) (*Notification, error) {
	var n Notification
	var enabled int
	err := s.db.QueryRow(`SELECT id,name,type,config,enabled,created_at FROM notifications WHERE id=?`, id).
		Scan(&n.ID, &n.Name, &n.Type, &n.Config, &enabled, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	n.Enabled = enabled == 1
	return &n, nil
}

// CreateNotification 新增通知渠道。
func (s *Store) CreateNotification(n Notification) (uint64, error) {
	res, err := s.db.Exec(
		`INSERT INTO notifications(name,type,config,enabled,created_at) VALUES(?,?,?,?,?)`,
		n.Name, n.Type, n.Config, boolInt(n.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

// UpdateNotification 更新通知渠道。
func (s *Store) UpdateNotification(n Notification) error {
	res, err := s.db.Exec(
		`UPDATE notifications SET name=?,type=?,config=?,enabled=? WHERE id=?`,
		n.Name, n.Type, n.Config, boolInt(n.Enabled), n.ID,
	)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteNotification 删除通知渠道。
func (s *Store) DeleteNotification(id uint64) error {
	res, err := s.db.Exec(`DELETE FROM notifications WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}
