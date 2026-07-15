// Package store 负责持久化：服务器登记、指标降采样历史、告警、监控、计划任务等。
// 使用 SQLite（WAL 模式），所有 SQL 集中在本包，便于以后替换为 PostgreSQL。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store 封装数据库连接。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）数据库并初始化表结构。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 写入是串行的，限制连接数避免 "database is locked"。
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接（仅供本包内其他文件使用）。
func (s *Store) DB() *sql.DB { return s.db }

// migrate 创建所有表。使用 IF NOT EXISTS，可重复执行。
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			secret       TEXT NOT NULL UNIQUE,
			sort_order   INTEGER NOT NULL DEFAULT 0,
			hidden       INTEGER NOT NULL DEFAULT 0,
			note         TEXT NOT NULL DEFAULT '',
			expires_at   INTEGER NOT NULL DEFAULT 0,
			tags         TEXT NOT NULL DEFAULT '',
			grp          TEXT NOT NULL DEFAULT '',
			last_online_at INTEGER NOT NULL DEFAULT 0,
			last_ip      TEXT NOT NULL DEFAULT '',
			last_ipv4    TEXT NOT NULL DEFAULT '',
			last_ipv6    TEXT NOT NULL DEFAULT '',
			reset_day    INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS metrics_minute (
			server_id    INTEGER NOT NULL,
			ts           INTEGER NOT NULL,
			cpu_avg      REAL NOT NULL,
			cpu_max      REAL NOT NULL,
			mem_used     INTEGER NOT NULL,
			disk_used    INTEGER NOT NULL,
			net_in       INTEGER NOT NULL,
			net_out      INTEGER NOT NULL,
			load1        REAL NOT NULL,
			PRIMARY KEY (server_id, ts)
		)`,
		`CREATE TABLE IF NOT EXISTS metrics_hour (
			server_id    INTEGER NOT NULL,
			ts           INTEGER NOT NULL,
			cpu_avg      REAL NOT NULL,
			cpu_max      REAL NOT NULL,
			mem_used     INTEGER NOT NULL,
			disk_used    INTEGER NOT NULL,
			net_in       INTEGER NOT NULL,
			net_out      INTEGER NOT NULL,
			load1        REAL NOT NULL,
			PRIMARY KEY (server_id, ts)
		)`,
		`CREATE TABLE IF NOT EXISTS traffic_monthly (
			server_id    INTEGER NOT NULL,
			year_month   TEXT NOT NULL,
			in_bytes     INTEGER NOT NULL DEFAULT 0,
			out_bytes    INTEGER NOT NULL DEFAULT 0,
			last_in      INTEGER NOT NULL DEFAULT 0,
			last_out     INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (server_id, year_month)
		)`,
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			metric       TEXT NOT NULL,
			operator     TEXT NOT NULL,
			threshold    REAL NOT NULL,
			duration     INTEGER NOT NULL,
			server_ids   TEXT NOT NULL DEFAULT '',
			notification_id INTEGER NOT NULL DEFAULT 0,
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS alert_events (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id      INTEGER NOT NULL,
			server_id    INTEGER NOT NULL,
			state        TEXT NOT NULL,
			message      TEXT NOT NULL,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			type         TEXT NOT NULL,
			config       TEXT NOT NULL,
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS monitors (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			type         TEXT NOT NULL,
			target       TEXT NOT NULL,
			server_id    INTEGER NOT NULL,
			interval     INTEGER NOT NULL DEFAULT 60,
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS monitor_results (
			monitor_id   INTEGER NOT NULL,
			ts           INTEGER NOT NULL,
			success      INTEGER NOT NULL,
			delay        REAL NOT NULL,
			message      TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (monitor_id, ts)
		)`,
		`CREATE TABLE IF NOT EXISTS cron_tasks (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT NOT NULL,
			command      TEXT NOT NULL,
			schedule     TEXT NOT NULL,
			server_ids   TEXT NOT NULL DEFAULT '',
			enabled      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS task_logs (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id      TEXT NOT NULL,
			cron_id      INTEGER NOT NULL DEFAULT 0,
			server_id    INTEGER NOT NULL,
			success      INTEGER NOT NULL,
			output       TEXT NOT NULL DEFAULT '',
			created_at   INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key          TEXT PRIMARY KEY,
			value        TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// 旧库补列：列已存在时报错可安全忽略。
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN tags TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN grp TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN last_online_at INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN last_ip TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN last_ipv4 TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN last_ipv6 TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE servers ADD COLUMN reset_day INTEGER NOT NULL DEFAULT 1`)
	return nil
}
