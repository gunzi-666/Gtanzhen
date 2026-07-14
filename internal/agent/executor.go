package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"probe/internal/protocol"
)

// maxOutputBytes 限制远程执行命令回传的输出大小，防止打爆面板。
const maxOutputBytes = 1 << 20 // 1MB

// Executor 执行面板下发的任务。
type Executor struct {
	secret         string // 用于校验 exec 命令签名
	disableCommand bool   // Agent 侧单方面禁用远程执行
}

// NewExecutor 创建任务执行器。
func NewExecutor(secret string, disableCommand bool) *Executor {
	return &Executor{secret: secret, disableCommand: disableCommand}
}

// Run 执行一个任务并返回结果。
func (e *Executor) Run(ctx context.Context, task protocol.TaskDispatch) protocol.TaskResult {
	res := protocol.TaskResult{TaskID: task.TaskID, Type: task.Type}
	timeout := time.Duration(task.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	switch task.Type {
	case protocol.TaskTCPing:
		e.tcping(&res, task.Target, timeout)
	case protocol.TaskHTTPGet:
		e.httpGet(ctx, &res, task.Target, timeout)
	case protocol.TaskPing:
		e.ping(&res, task.Target, timeout)
	case protocol.TaskExecCommand:
		e.exec(ctx, &res, task, timeout)
	default:
		res.Error = "unknown task type"
	}
	return res
}

// tcping 测试 TCP 端口连通性并记录建连耗时。
func (e *Executor) tcping(res *protocol.TaskResult, target string, timeout time.Duration) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		res.Error = err.Error()
		return
	}
	_ = conn.Close()
	res.Success = true
	res.Delay = float64(time.Since(start).Microseconds()) / 1000.0
}

// httpGet 探测 HTTP(S) 可用性。
func (e *Executor) httpGet(ctx context.Context, res *protocol.TaskResult, target string, timeout time.Duration) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		res.Error = err.Error()
		return
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	res.Delay = float64(time.Since(start).Microseconds()) / 1000.0
	res.HTTPCode = resp.StatusCode
	res.Success = resp.StatusCode < 400
	if !res.Success {
		res.Error = fmt.Sprintf("status %d", resp.StatusCode)
	}
}

// ping 用系统 ping 命令做 ICMP 探测（免 root，跨平台）。
func (e *Executor) ping(res *protocol.TaskResult, target string, timeout time.Duration) {
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", fmt.Sprintf("%d", timeout.Milliseconds()), target}
	} else {
		args = []string{"-c", "1", "-W", fmt.Sprintf("%d", int(timeout.Seconds())+1), target}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()
	start := time.Now()
	out, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	if err != nil {
		res.Error = string(out)
		if res.Error == "" {
			res.Error = err.Error()
		}
		return
	}
	res.Success = true
	res.Delay = float64(time.Since(start).Microseconds()) / 1000.0
}

// exec 执行远程命令，先做开关、签名与新鲜度校验。
func (e *Executor) exec(ctx context.Context, res *protocol.TaskResult, task protocol.TaskDispatch, timeout time.Duration) {
	if e.disableCommand {
		res.Error = "command execution disabled on this agent"
		return
	}
	// 拒绝超过 5 分钟的旧签名，抵御重放。
	if task.SignTs == 0 || time.Since(time.Unix(task.SignTs, 0)) > 5*time.Minute {
		res.Error = "command signature expired"
		return
	}
	if !protocol.VerifyExec(e.secret, task.TaskID, task.Target, task.Nonce, task.SignTs, task.Sign) {
		res.Error = "invalid command signature"
		return
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "powershell", "-NonInteractive", "-Command", task.Target)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", task.Target)
	}
	out, err := cmd.CombinedOutput()
	if len(out) > maxOutputBytes {
		out = append(out[:maxOutputBytes], []byte("\n...[output truncated]")...)
	}
	res.Output = string(out)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Success = true
}
