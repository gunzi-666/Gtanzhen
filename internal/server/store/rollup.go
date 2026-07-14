package store

import (
	"log"
	"time"
)

// 数据保留策略。
const (
	minuteRetention = 7 * 24 * time.Hour       // 分钟级保留 7 天
	hourRetention   = 365 * 24 * time.Hour     // 小时级保留 12 个月
)

// RollupWorker 周期性地把分钟数据聚合为小时，并清理过期数据。
// agg 用于在聚合前先落盘内存中的分钟桶。
func (s *Store) RollupWorker(agg *Aggregator) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		// 启动时先跑一次。
		s.rollupHours()
		s.cleanup()
		for range ticker.C {
			if agg != nil {
				agg.FlushAll()
			}
			s.rollupHours()
			s.cleanup()
		}
	}()
}

// rollupHours 把已完结的整点分钟数据聚合到 metrics_hour。
func (s *Store) rollupHours() {
	// 聚合上一个已结束的小时，避免聚合进行中的小时。
	hourStart := time.Now().Truncate(time.Hour).Add(-time.Hour).Unix()
	hourEnd := hourStart + 3600
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO metrics_hour(server_id,ts,cpu_avg,cpu_max,mem_used,disk_used,net_in,net_out,load1)
		 SELECT server_id, ?, AVG(cpu_avg), MAX(cpu_max), AVG(mem_used), AVG(disk_used), AVG(net_in), AVG(net_out), AVG(load1)
		 FROM metrics_minute WHERE ts>=? AND ts<? GROUP BY server_id`,
		hourStart, hourStart, hourEnd,
	)
	if err != nil {
		log.Printf("rollup hours: %v", err)
	}
}

// cleanup 删除超过保留期的历史数据。
func (s *Store) cleanup() {
	minuteCutoff := time.Now().Add(-minuteRetention).Unix()
	hourCutoff := time.Now().Add(-hourRetention).Unix()
	if _, err := s.db.Exec(`DELETE FROM metrics_minute WHERE ts<?`, minuteCutoff); err != nil {
		log.Printf("cleanup minute: %v", err)
	}
	if _, err := s.db.Exec(`DELETE FROM metrics_hour WHERE ts<?`, hourCutoff); err != nil {
		log.Printf("cleanup hour: %v", err)
	}
	// 监控结果保留 30 天。
	monCutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	if _, err := s.db.Exec(`DELETE FROM monitor_results WHERE ts<?`, monCutoff); err != nil {
		log.Printf("cleanup monitor: %v", err)
	}
}
