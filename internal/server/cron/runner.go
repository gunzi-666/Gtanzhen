// Package cron 按 cron 表达式定时向目标服务器下发远程命令。
package cron

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"probe/internal/server/store"
	"probe/internal/server/task"

	robfig "github.com/robfig/cron/v3"
)

// Runner 计划任务运行器，把 DB 中的 cron 任务注册到调度器。
type Runner struct {
	store *store.Store
	tasks *task.Manager

	mu    sync.Mutex
	cron  *robfig.Cron
	byJob map[uint64]robfig.EntryID // cronTaskID -> 调度条目
}

// New 创建运行器。
func New(s *store.Store, tm *task.Manager) *Runner {
	return &Runner{
		store: s,
		tasks: tm,
		cron:  robfig.New(),
		byJob: make(map[uint64]robfig.EntryID),
	}
}

// Start 加载任务并启动调度。
func (r *Runner) Start() {
	r.Reload()
	r.cron.Start()
}

// Reload 重新从数据库加载所有启用的计划任务。
// 增删改计划任务后由 API 调用以即时生效。
func (r *Runner) Reload() {
	tasks, err := r.store.ListCronTasks()
	if err != nil {
		log.Printf("[cron] list tasks: %v", err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range r.byJob {
		r.cron.Remove(id)
	}
	r.byJob = make(map[uint64]robfig.EntryID)

	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		ct := t // 捕获副本
		id, err := r.cron.AddFunc(ct.Schedule, func() { r.run(ct) })
		if err != nil {
			log.Printf("[cron] invalid schedule for task %d (%q): %v", ct.ID, ct.Schedule, err)
			continue
		}
		r.byJob[ct.ID] = id
	}
}

// run 在任务的所有目标服务器上执行命令并记录日志。
func (r *Runner) run(t store.CronTask) {
	for _, serverID := range parseIDs(t.ServerIDs) {
		go func(sid uint64) {
			output, err := r.tasks.RunCommand(sid, t.Command, 60)
			logEntry := store.TaskLog{
				CronID:   t.ID,
				ServerID: sid,
				Success:  err == nil,
				Output:   output,
			}
			if err != nil {
				if logEntry.Output != "" {
					logEntry.Output += "\n"
				}
				logEntry.Output += "错误: " + err.Error()
			}
			_ = r.store.AddTaskLog(logEntry)
		}(serverID)
	}
}

func parseIDs(s string) []uint64 {
	var out []uint64
	for _, part := range strings.Split(s, ",") {
		if id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}
