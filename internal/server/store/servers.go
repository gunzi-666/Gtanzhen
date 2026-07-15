package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Server 是一台被监控服务器的登记信息。
type Server struct {
	ID        uint64   `json:"id"`
	Name      string   `json:"name"`
	Secret    string   `json:"secret"`
	SortOrder int      `json:"sort_order"`
	Hidden    bool     `json:"hidden"`
	Note      string   `json:"note"`
	ExpiresAt int64    `json:"expires_at"` // 到期时间（unix 秒），0 表示未设置
	Tags      []string `json:"tags"`       // 个性标签，DB 内以逗号分隔存储
	Group     string   `json:"group"`      // 分组名，空表示未分组（DB 列名 grp，避开保留字）
	LastIP    string   `json:"last_ip"`    // Agent 最近一次接入的来源 IP（仅管理后台可见）
	CreatedAt int64    `json:"created_at"`
}

// joinTags / splitTags 在 []string 与 DB 逗号分隔文本之间转换。
func joinTags(tags []string) string {
	var out []string
	for _, t := range tags {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, ",")
}

func splitTags(s string) []string {
	out := []string{}
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("not found")

// MarkOnline 记录服务器本次上线时间，返回是否为该机首次上线。
// 用于区分「新装 Agent 首次上线」与「面板重启后的全量回连」。
func (s *Store) MarkOnline(id uint64) (first bool, err error) {
	var last int64
	if err := s.db.QueryRow(`SELECT last_online_at FROM servers WHERE id=?`, id).Scan(&last); err != nil {
		return false, err
	}
	if _, err := s.db.Exec(`UPDATE servers SET last_online_at=? WHERE id=?`, time.Now().Unix(), id); err != nil {
		return false, err
	}
	return last == 0, nil
}

// SaveLastIP 记录 Agent 最近一次接入的来源 IP。
func (s *Store) SaveLastIP(id uint64, ip string) error {
	_, err := s.db.Exec(`UPDATE servers SET last_ip=? WHERE id=?`, ip, id)
	return err
}

// genSecret 生成随机 secret。
func genSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateServer 新增一台服务器，自动生成 secret。
func (s *Store) CreateServer(name, note string) (*Server, error) {
	srv := &Server{
		Name:      name,
		Secret:    genSecret(),
		Note:      note,
		Tags:      []string{},
		CreatedAt: time.Now().Unix(),
	}
	res, err := s.db.Exec(
		`INSERT INTO servers(name, secret, note, created_at) VALUES(?,?,?,?)`,
		srv.Name, srv.Secret, srv.Note, srv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	srv.ID = uint64(id)
	return srv, nil
}

// ListServers 返回所有服务器（按 sort_order, id）。
func (s *Store) ListServers() ([]Server, error) {
	rows, err := s.db.Query(`SELECT id,name,secret,sort_order,hidden,note,expires_at,tags,grp,last_ip,created_at FROM servers ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		var srv Server
		var hidden int
		var tags string
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Secret, &srv.SortOrder, &hidden, &srv.Note, &srv.ExpiresAt, &tags, &srv.Group, &srv.LastIP, &srv.CreatedAt); err != nil {
			return nil, err
		}
		srv.Hidden = hidden == 1
		srv.Tags = splitTags(tags)
		out = append(out, srv)
	}
	return out, rows.Err()
}

// UpdateServer 更新名称、备注、排序、隐藏、到期时间、标签、分组。
func (s *Store) UpdateServer(id uint64, name, note string, sortOrder int, hidden bool, expiresAt int64, tags []string, group string) error {
	h := 0
	if hidden {
		h = 1
	}
	res, err := s.db.Exec(
		`UPDATE servers SET name=?, note=?, sort_order=?, hidden=?, expires_at=?, tags=?, grp=? WHERE id=?`,
		name, note, sortOrder, h, expiresAt, joinTags(tags), strings.TrimSpace(group), id,
	)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteServer 删除服务器及其历史数据。
func (s *Store) DeleteServer(id uint64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM servers WHERE id=?`,
		`DELETE FROM metrics_minute WHERE server_id=?`,
		`DELETE FROM metrics_hour WHERE server_id=?`,
		`DELETE FROM traffic_monthly WHERE server_id=?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AuthBySecret 通过 secret 查服务器 id 与名称，用于 Agent 认证。
func (s *Store) AuthBySecret(secret string) (uint64, string, bool) {
	var id uint64
	var name string
	err := s.db.QueryRow(`SELECT id,name FROM servers WHERE secret=?`, secret).Scan(&id, &name)
	if err != nil {
		return 0, "", false
	}
	return id, name, true
}

// SecretOf 返回指定服务器的 secret（用于命令签名）。
func (s *Store) SecretOf(id uint64) (string, error) {
	var secret string
	err := s.db.QueryRow(`SELECT secret FROM servers WHERE id=?`, id).Scan(&secret)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return secret, err
}

// affected 把 0 行影响转成 ErrNotFound。
func affected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
