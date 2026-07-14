package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"probe/internal/protocol"

	"github.com/gorilla/websocket"
)

// Version 是 Agent 版本号，由 cmd/agent 在启动时用构建注入的版本覆盖。
var Version = "dev"

// Config 是 Agent 运行配置。
type Config struct {
	Server         string // 面板 WS 地址，例如 ws://127.0.0.1:8008/api/agent
	Secret         string
	ReportPeriod   int  // 上报间隔（秒），面板可在认证时覆盖
	DisableCommand bool // 禁用远程执行
}

// Client 是 Agent 主体，管理连接生命周期。
type Client struct {
	cfg       Config
	collector *Collector
	executor  *Executor

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewClient 创建 Agent 客户端。
func NewClient(cfg Config) *Client {
	if cfg.ReportPeriod <= 0 {
		cfg.ReportPeriod = 2
	}
	return &Client{
		cfg:       cfg,
		collector: NewCollector(),
		executor:  NewExecutor(cfg.Secret, cfg.DisableCommand),
	}
}

// Run 启动 Agent，断线后自动重连，直到 ctx 取消。
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.connectOnce(ctx)
		if err != nil {
			log.Printf("connection closed: %v; retry in %s", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
		// 成功连过一段时间后重置退避在 connectOnce 内处理。
	}
}

// connectOnce 建立一次连接，完成认证与收发循环。
func (c *Client) connectOnce(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.cfg.Server, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// ReadMessage 不感知 ctx，收到退出信号时主动关连接让它立即返回，
	// 否则 systemctl stop 会一直等到超时被 SIGKILL。
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-watchDone:
		}
	}()

	// 认证。
	auth := protocol.AuthRequest{Secret: c.cfg.Secret, AgentVersion: Version}
	if err := c.send(protocol.TypeAuth, "", auth); err != nil {
		return err
	}

	// 等待认证结果。
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	var msg protocol.Message
	if err := unmarshal(data, &msg); err != nil {
		return err
	}
	if msg.Type != protocol.TypeAuthResult {
		return errUnexpected
	}
	var ar protocol.AuthResult
	if err := msg.Decode(&ar); err != nil {
		return err
	}
	if !ar.OK {
		log.Printf("auth rejected: %s", ar.Message)
		return errAuthRejected
	}
	if ar.ReportPeriod > 0 {
		c.cfg.ReportPeriod = ar.ReportPeriod
	}
	log.Printf("authenticated, server_id=%d, report every %ds", ar.ServerID, c.cfg.ReportPeriod)

	// 上报主机静态信息。
	if err := c.send(protocol.TypeHostInfo, "", c.collector.HostInfo()); err != nil {
		return err
	}

	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.reportLoop(loopCtx)

	// 读循环：处理下发任务与心跳。
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var m protocol.Message
		if err := unmarshal(data, &m); err != nil {
			continue
		}
		c.handle(loopCtx, &m)
	}
}

// reportLoop 周期性采集并上报指标，同时发送心跳。
func (c *Client) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.ReportPeriod) * time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m := c.collector.Metrics(ctx)
			if err := c.send(protocol.TypeMetrics, "", m); err != nil {
				return
			}
		case <-heartbeat.C:
			_ = c.send(protocol.TypeHeartbeat, "", struct{}{})
		}
	}
}

// handle 处理面板下发的消息。
func (c *Client) handle(ctx context.Context, m *protocol.Message) {
	switch m.Type {
	case protocol.TypeHeartbeat:
		// 忽略，仅保活。
	case protocol.TypeTaskDispatch:
		var task protocol.TaskDispatch
		if err := m.Decode(&task); err != nil {
			return
		}
		go func() {
			res := c.executor.Run(ctx, task)
			_ = c.send(protocol.TypeTaskResult, task.TaskID, res)
		}()
	}
}

// send 发送一条消息（加锁保证并发写安全）。
func (c *Client) send(t protocol.MessageType, id string, payload any) error {
	msg, err := protocol.NewMessage(t, id, time.Now().Unix(), payload)
	if err != nil {
		return err
	}
	data, err := marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errNoConn
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}
