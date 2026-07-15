package store

import (
	"sync"
	"time"

	"probe/internal/protocol"
)

// minuteBucket 累积某台服务器当前分钟内的采样。
type minuteBucket struct {
	minuteTs int64 // 该分钟起始 Unix 秒
	count    int
	cpuSum   float64
	cpuMax   float64
	memUsed  uint64
	diskUsed uint64
	netIn    uint64
	netOut   uint64
	load1Sum float64
}

// Aggregator 把高频原始指标聚合为分钟级并落库，同时维护月流量。
type Aggregator struct {
	store   *Store
	mu      sync.Mutex
	buckets map[uint64]*minuteBucket
}

// NewAggregator 创建聚合器。
func NewAggregator(s *Store) *Aggregator {
	return &Aggregator{store: s, buckets: make(map[uint64]*minuteBucket)}
}

// Add 接收一条原始指标采样。
func (a *Aggregator) Add(serverID uint64, m protocol.Metrics) {
	now := time.Now()
	minuteTs := now.Truncate(time.Minute).Unix()

	a.mu.Lock()
	b, ok := a.buckets[serverID]
	if !ok || b.minuteTs != minuteTs {
		if ok && b.count > 0 {
			a.flush(serverID, b)
		}
		b = &minuteBucket{minuteTs: minuteTs}
		a.buckets[serverID] = b
	}
	b.count++
	b.cpuSum += m.CPU
	if m.CPU > b.cpuMax {
		b.cpuMax = m.CPU
	}
	b.memUsed = m.MemUsed
	b.diskUsed = m.DiskUsed
	b.netIn = m.NetInSpeed
	b.netOut = m.NetOutSpeed
	b.load1Sum += m.Load1
	a.mu.Unlock()

	a.updateTraffic(serverID, m.NetInTransfer, m.NetOutTransfer)
}

// flush 把一个分钟桶写入 metrics_minute（调用方须持锁）。
func (a *Aggregator) flush(serverID uint64, b *minuteBucket) {
	if b.count == 0 {
		return
	}
	_, _ = a.store.db.Exec(
		`INSERT OR REPLACE INTO metrics_minute(server_id,ts,cpu_avg,cpu_max,mem_used,disk_used,net_in,net_out,load1)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		serverID, b.minuteTs,
		b.cpuSum/float64(b.count), b.cpuMax,
		b.memUsed, b.diskUsed, b.netIn, b.netOut,
		b.load1Sum/float64(b.count),
	)
}

// FlushAll 立即落盘所有未满一分钟的桶（关机或定时调用）。
func (a *Aggregator) FlushAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, b := range a.buckets {
		a.flush(id, b)
	}
}

// updateTraffic 累计计费周期流量（按每台服务器的重置日划分周期），
// 处理 Agent 重启导致的计数器回退。
func (a *Aggregator) updateTraffic(serverID, curIn, curOut uint64) {
	tx, err := a.store.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	var resetDay int
	if err := tx.QueryRow(`SELECT reset_day FROM servers WHERE id=?`, serverID).Scan(&resetDay); err != nil {
		resetDay = 1
	}
	ym := TrafficPeriodKey(time.Now(), resetDay)

	var inBytes, outBytes, lastIn, lastOut uint64
	row := tx.QueryRow(`SELECT in_bytes,out_bytes,last_in,last_out FROM traffic_monthly WHERE server_id=? AND year_month=?`, serverID, ym)
	err = row.Scan(&inBytes, &outBytes, &lastIn, &lastOut)
	if err != nil {
		// 本月首条记录。
		_, _ = tx.Exec(`INSERT INTO traffic_monthly(server_id,year_month,in_bytes,out_bytes,last_in,last_out) VALUES(?,?,?,?,?,?)`,
			serverID, ym, 0, 0, curIn, curOut)
		_ = tx.Commit()
		return
	}

	// 计数器单调递增；若变小说明 Agent 重启，增量按当前值计。
	var dIn, dOut uint64
	if curIn >= lastIn {
		dIn = curIn - lastIn
	} else {
		dIn = curIn
	}
	if curOut >= lastOut {
		dOut = curOut - lastOut
	} else {
		dOut = curOut
	}
	_, _ = tx.Exec(`UPDATE traffic_monthly SET in_bytes=?, out_bytes=?, last_in=?, last_out=? WHERE server_id=? AND year_month=?`,
		inBytes+dIn, outBytes+dOut, curIn, curOut, serverID, ym)
	_ = tx.Commit()
}

// MetricPoint 是历史查询返回的一个数据点。
type MetricPoint struct {
	Ts      int64   `json:"ts"`
	CPU     float64 `json:"cpu"`
	MemUsed uint64  `json:"mem_used"`
	DiskUsed uint64 `json:"disk_used"`
	NetIn   uint64  `json:"net_in"`
	NetOut  uint64  `json:"net_out"`
	Load1   float64 `json:"load1"`
}

// History 返回指定服务器某时间范围的历史点。
// 范围超过 24 小时用小时表，否则用分钟表。
func (s *Store) History(serverID uint64, from, to int64) ([]MetricPoint, error) {
	table := "metrics_minute"
	if to-from > 24*3600 {
		table = "metrics_hour"
	}
	rows, err := s.db.Query(
		`SELECT ts,cpu_avg,mem_used,disk_used,net_in,net_out,load1 FROM `+table+
			` WHERE server_id=? AND ts>=? AND ts<=? ORDER BY ts`,
		serverID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MetricPoint{}
	for rows.Next() {
		var p MetricPoint
		if err := rows.Scan(&p.Ts, &p.CPU, &p.MemUsed, &p.DiskUsed, &p.NetIn, &p.NetOut, &p.Load1); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TrafficMonth 返回某服务器指定月份（YYYY-MM）的进出流量字节数。
func (s *Store) TrafficMonth(serverID uint64, ym string) (in, out uint64) {
	_ = s.db.QueryRow(`SELECT in_bytes,out_bytes FROM traffic_monthly WHERE server_id=? AND year_month=?`, serverID, ym).Scan(&in, &out)
	return
}
