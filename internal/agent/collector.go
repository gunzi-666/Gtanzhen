// Package agent 实现探针端：采集指标、维护 WS 连接、执行下发任务。
package agent

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"probe/internal/protocol"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// netCounter 记录上一次网络采样，用于计算速率。
type netCounter struct {
	lastIn   uint64
	lastOut  uint64
	lastTime time.Time
}

// Collector 负责采集主机静态信息与动态指标。
type Collector struct {
	net netCounter
}

// NewCollector 创建采集器。
func NewCollector() *Collector {
	return &Collector{}
}

// HostInfo 采集主机静态信息（上线时调用一次）。
func (c *Collector) HostInfo() protocol.HostInfo {
	info := protocol.HostInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	if hi, err := host.Info(); err == nil {
		info.Hostname = hi.Hostname
		info.Platform = hi.Platform
		info.PlatformVersion = hi.PlatformVersion
		info.KernelVersion = hi.KernelVersion
		info.Virtualization = hi.VirtualizationSystem
		info.BootTime = hi.BootTime
		if hi.OS != "" {
			info.OS = hi.OS
		}
	}

	if cpus, err := cpu.Info(); err == nil {
		for _, ci := range cpus {
			info.CPU = append(info.CPU, fmt.Sprintf("%s %d Cores", ci.ModelName, ci.Cores))
		}
	}
	if len(info.CPU) == 0 {
		if n, err := cpu.Counts(true); err == nil {
			info.CPU = append(info.CPU, fmt.Sprintf("%d Cores", n))
		}
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		info.MemTotal = vm.Total
	}
	if sm, err := mem.SwapMemory(); err == nil {
		info.SwapTotal = sm.Total
	}
	if usage, err := disk.Usage(rootPath()); err == nil {
		info.DiskTotal = usage.Total
	}

	// 自测公网 IPv4/IPv6：连接来源地址只有一个（双栈机器通常走 v6），
	// 两个都要就得 Agent 自己探测后上报。
	info.IPv4, info.IPv6 = PublicIPs()

	return info
}

// Metrics 采集一次动态指标。
func (c *Collector) Metrics(ctx context.Context) protocol.Metrics {
	var m protocol.Metrics

	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(percents) > 0 {
		m.CPU = percents[0]
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		m.MemUsed = vm.Used
	}
	if sm, err := mem.SwapMemory(); err == nil {
		m.SwapUsed = sm.Used
	}
	if usage, err := disk.Usage(rootPath()); err == nil {
		m.DiskUsed = usage.Used
	}

	if l, err := load.Avg(); err == nil {
		m.Load1 = l.Load1
		m.Load5 = l.Load5
		m.Load15 = l.Load15
	}

	c.fillNetwork(&m)

	if conns, err := net.Connections("tcp"); err == nil {
		m.TCPConnCount = uint64(len(conns))
	}
	if conns, err := net.Connections("udp"); err == nil {
		m.UDPConnCount = uint64(len(conns))
	}
	if pids, err := process.Pids(); err == nil {
		m.ProcessCount = uint64(len(pids))
	}
	if up, err := host.Uptime(); err == nil {
		m.Uptime = up
	}

	return m
}

// fillNetwork 计算网络累计流量与瞬时速率。
func (c *Collector) fillNetwork(m *protocol.Metrics) {
	counters, err := net.IOCounters(false)
	if err != nil || len(counters) == 0 {
		return
	}
	total := counters[0]
	m.NetInTransfer = total.BytesRecv
	m.NetOutTransfer = total.BytesSent

	now := time.Now()
	if !c.net.lastTime.IsZero() {
		elapsed := now.Sub(c.net.lastTime).Seconds()
		if elapsed > 0 {
			if total.BytesRecv >= c.net.lastIn {
				m.NetInSpeed = uint64(float64(total.BytesRecv-c.net.lastIn) / elapsed)
			}
			if total.BytesSent >= c.net.lastOut {
				m.NetOutSpeed = uint64(float64(total.BytesSent-c.net.lastOut) / elapsed)
			}
		}
	}
	c.net.lastIn = total.BytesRecv
	c.net.lastOut = total.BytesSent
	c.net.lastTime = now
}

// rootPath 返回统计磁盘用量的根路径，按平台区分。
func rootPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}
