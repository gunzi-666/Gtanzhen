// Package monitor 周期性地向 Agent 下发服务探测任务并记录结果。
package monitor

import (
	"fmt"
	"log"
	"time"

	"probe/internal/protocol"
	"probe/internal/server/store"
	"probe/internal/server/task"
)

// Scheduler 服务监控调度器。
type Scheduler struct {
	store *store.Store
	tasks *task.Manager

	lastRun map[uint64]time.Time // monitorID -> 上次探测时间
}

// New 创建调度器。
func New(s *store.Store, tm *task.Manager) *Scheduler {
	return &Scheduler{store: s, tasks: tm, lastRun: make(map[uint64]time.Time)}
}

// Run 启动调度循环：每 10 秒检查一次哪些监控项到点该探测。
func (sc *Scheduler) Run() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			sc.tick()
		}
	}()
}

func (sc *Scheduler) tick() {
	monitors, err := sc.store.ListMonitors()
	if err != nil {
		return
	}
	now := time.Now()
	for _, mon := range monitors {
		if !mon.Enabled {
			continue
		}
		last := sc.lastRun[mon.ID]
		if now.Sub(last) < time.Duration(mon.Interval)*time.Second {
			continue
		}
		sc.lastRun[mon.ID] = now
		go sc.probe(mon)
	}
}

// probe 执行一次探测并保存结果。
func (sc *Scheduler) probe(mon store.Monitor) {
	typ := protocol.TaskType(mon.Type)
	res, err := sc.tasks.Probe(mon.ServerID, typ, mon.Target, 10)

	result := store.MonitorResult{MonitorID: mon.ID, Ts: time.Now().Unix()}
	if err != nil {
		result.Success = false
		result.Message = err.Error()
	} else {
		result.Success = res.Success
		result.Delay = res.Delay
		if res.Error != "" {
			result.Message = res.Error
		} else if res.HTTPCode > 0 {
			result.Message = fmt.Sprintf("HTTP %d", res.HTTPCode)
		}
	}
	if err := sc.store.AddMonitorResult(result); err != nil {
		log.Printf("[monitor] save result: %v", err)
	}
}
