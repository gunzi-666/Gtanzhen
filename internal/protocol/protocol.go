// Package protocol 定义 Agent 与面板之间通过 WebSocket 传输的消息格式。
// 两端共用，是整个系统的通信契约。
package protocol

import "encoding/json"

// MessageType 是消息信封的类型标识。
type MessageType string

const (
	// TypeAuth Agent 上线后发送的认证消息（携带 secret）。
	TypeAuth MessageType = "auth"
	// TypeAuthResult 面板回复认证结果。
	TypeAuthResult MessageType = "auth_result"
	// TypeHeartbeat 心跳，双向保活。
	TypeHeartbeat MessageType = "heartbeat"
	// TypeHostInfo 主机静态信息，Agent 上线时上报一次。
	TypeHostInfo MessageType = "host_info"
	// TypeMetrics 动态指标，Agent 按固定间隔上报。
	TypeMetrics MessageType = "metrics"
	// TypeTaskDispatch 面板向 Agent 下发任务。
	TypeTaskDispatch MessageType = "task_dispatch"
	// TypeTaskResult Agent 回报任务执行结果。
	TypeTaskResult MessageType = "task_result"
)

// Message 是所有通信的统一信封。
// Payload 使用延迟解码，接收方根据 Type 再解析具体结构。
type Message struct {
	Type    MessageType     `json:"type"`
	ID      string          `json:"id,omitempty"`
	Ts      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// NewMessage 构造一个消息，payload 会被序列化进信封。
func NewMessage(t MessageType, id string, ts int64, payload any) (*Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{Type: t, ID: id, Ts: ts, Payload: raw}, nil
}

// Decode 将信封中的 payload 解析到目标结构。
func (m *Message) Decode(v any) error {
	return json.Unmarshal(m.Payload, v)
}

// AuthRequest Agent -> 面板：认证请求。
type AuthRequest struct {
	Secret       string `json:"secret"`
	AgentVersion string `json:"agent_version"`
}

// AuthResult 面板 -> Agent：认证结果。
type AuthResult struct {
	OK           bool   `json:"ok"`
	Message      string `json:"message,omitempty"`
	ServerID     uint64 `json:"server_id,omitempty"`
	ReportPeriod int    `json:"report_period,omitempty"` // 指标上报间隔（秒）
}

// HostInfo 主机静态信息。
type HostInfo struct {
	Hostname        string   `json:"hostname"`
	OS              string   `json:"os"`       // 例如 linux
	Platform        string   `json:"platform"` // 例如 ubuntu
	PlatformVersion string   `json:"platform_version"`
	KernelVersion   string   `json:"kernel_version"`
	Arch            string   `json:"arch"`
	Virtualization  string   `json:"virtualization"`
	CPU             []string `json:"cpu"`       // 每个型号一条，含核数描述
	MemTotal        uint64   `json:"mem_total"` // 字节
	SwapTotal       uint64   `json:"swap_total"`
	DiskTotal       uint64   `json:"disk_total"`
	BootTime        uint64   `json:"boot_time"` // Unix 秒
	CountryCode     string   `json:"country_code,omitempty"`
}

// Metrics 动态运行时指标。
type Metrics struct {
	CPU            float64 `json:"cpu"`      // 使用率百分比 0-100
	MemUsed        uint64  `json:"mem_used"` // 字节
	SwapUsed       uint64  `json:"swap_used"`
	DiskUsed       uint64  `json:"disk_used"`
	NetInSpeed     uint64  `json:"net_in_speed"`  // 字节/秒
	NetOutSpeed    uint64  `json:"net_out_speed"` // 字节/秒
	NetInTransfer  uint64  `json:"net_in_transfer"`  // 累计入向字节
	NetOutTransfer uint64  `json:"net_out_transfer"` // 累计出向字节
	Load1          float64 `json:"load1"`
	Load5          float64 `json:"load5"`
	Load15         float64 `json:"load15"`
	TCPConnCount   uint64  `json:"tcp_conn_count"`
	UDPConnCount   uint64  `json:"udp_conn_count"`
	ProcessCount   uint64  `json:"process_count"`
	Uptime         uint64  `json:"uptime"` // 秒
}

// TaskType 任务类型。
type TaskType string

const (
	// TaskPing ICMP ping 探测。
	TaskPing TaskType = "ping"
	// TaskTCPing TCP 端口连通性探测。
	TaskTCPing TaskType = "tcping"
	// TaskHTTPGet HTTP(S) 可用性探测。
	TaskHTTPGet TaskType = "http_get"
	// TaskExecCommand 远程执行命令（高危通道）。
	TaskExecCommand TaskType = "exec_command"
	// TaskUpgrade Agent 自升级：下载新二进制替换自身后退出，由 systemd 拉起。
	// 与 exec_command 一样使用 HMAC 签名防伪造，Target 为下载 URL。
	TaskUpgrade TaskType = "upgrade"
)

// TaskDispatch 面板 -> Agent：下发的任务。
// 对于 exec_command，Sign / Nonce / SignTs 共同用于防重放。
type TaskDispatch struct {
	TaskID  string   `json:"task_id"`
	Type    TaskType `json:"type"`
	Target  string   `json:"target"`            // ping/tcping/http 的目标；exec 时为命令
	Timeout int      `json:"timeout,omitempty"` // 秒
	Sign    string   `json:"sign,omitempty"`    // exec_command 的 HMAC 签名
	Nonce   string   `json:"nonce,omitempty"`   // 防重放随机串
	SignTs  int64    `json:"sign_ts,omitempty"` // 签名时间戳（Unix 秒），用于新鲜度校验
}

// TaskResult Agent -> 面板：任务执行结果。
type TaskResult struct {
	TaskID   string   `json:"task_id"`
	Type     TaskType `json:"type"`
	Success  bool     `json:"success"`
	Delay    float64  `json:"delay,omitempty"`     // 探测延迟（毫秒）
	Output   string   `json:"output,omitempty"`    // exec 输出（截断后）
	HTTPCode int      `json:"http_code,omitempty"` // http_get 的响应码
	Error    string   `json:"error,omitempty"`
}
