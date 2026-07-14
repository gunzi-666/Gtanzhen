package store

import "time"

// CronTask 计划任务：定时在指定服务器上执行命令。
// Schedule 为标准 5 段 cron 表达式。ServerIDs 为逗号分隔的目标服务器 id。
type CronTask struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Schedule  string `json:"schedule"`
	ServerIDs string `json:"server_ids"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
}

// ListCronTasks 返回全部计划任务。
func (s *Store) ListCronTasks() ([]CronTask, error) {
	rows, err := s.db.Query(`SELECT id,name,command,schedule,server_ids,enabled,created_at FROM cron_tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CronTask{}
	for rows.Next() {
		var c CronTask
		var enabled int
		if err := rows.Scan(&c.ID, &c.Name, &c.Command, &c.Schedule, &c.ServerIDs, &enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Enabled = enabled == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCronTask 新增计划任务。
func (s *Store) CreateCronTask(c CronTask) (uint64, error) {
	res, err := s.db.Exec(
		`INSERT INTO cron_tasks(name,command,schedule,server_ids,enabled,created_at) VALUES(?,?,?,?,?,?)`,
		c.Name, c.Command, c.Schedule, c.ServerIDs, boolInt(c.Enabled), time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

// UpdateCronTask 更新计划任务。
func (s *Store) UpdateCronTask(c CronTask) error {
	res, err := s.db.Exec(
		`UPDATE cron_tasks SET name=?,command=?,schedule=?,server_ids=?,enabled=? WHERE id=?`,
		c.Name, c.Command, c.Schedule, c.ServerIDs, boolInt(c.Enabled), c.ID,
	)
	if err != nil {
		return err
	}
	return affected(res)
}

// DeleteCronTask 删除计划任务。
func (s *Store) DeleteCronTask(id uint64) error {
	res, err := s.db.Exec(`DELETE FROM cron_tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	return affected(res)
}

// TaskLog 任务执行日志（手动执行或计划任务触发）。
type TaskLog struct {
	ID        uint64 `json:"id"`
	TaskID    string `json:"task_id"`
	CronID    uint64 `json:"cron_id"`
	ServerID  uint64 `json:"server_id"`
	Success   bool   `json:"success"`
	Output    string `json:"output"`
	CreatedAt int64  `json:"created_at"`
}

// AddTaskLog 写入一条任务执行日志。
func (s *Store) AddTaskLog(l TaskLog) error {
	output := l.Output
	if len(output) > 64*1024 {
		output = output[:64*1024] + "\n...[truncated]"
	}
	_, err := s.db.Exec(
		`INSERT INTO task_logs(task_id,cron_id,server_id,success,output,created_at) VALUES(?,?,?,?,?,?)`,
		l.TaskID, l.CronID, l.ServerID, boolInt(l.Success), output, time.Now().Unix(),
	)
	return err
}

// ListTaskLogs 返回最近的任务日志。
func (s *Store) ListTaskLogs(limit int) ([]TaskLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id,task_id,cron_id,server_id,success,output,created_at FROM task_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TaskLog{}
	for rows.Next() {
		var l TaskLog
		var success int
		if err := rows.Scan(&l.ID, &l.TaskID, &l.CronID, &l.ServerID, &success, &l.Output, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Success = success == 1
		out = append(out, l)
	}
	return out, rows.Err()
}
