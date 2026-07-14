package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"probe/internal/protocol"
)

// CleanupOldBinary 清理上次升级留下的旧二进制（启动时调用，失败忽略）。
func CleanupOldBinary() {
	if exe, err := selfPath(); err == nil {
		_ = os.Remove(exe + ".old")
	}
}

// selfPath 返回当前可执行文件的真实路径（解析符号链接）。
func selfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// upgrade 执行自升级：验签 -> 下载 -> 自检 -> 原子替换 -> 退出交给 systemd 重启。
func (e *Executor) upgrade(ctx context.Context, res *protocol.TaskResult, task protocol.TaskDispatch) {
	// 与 exec_command 相同的防重放校验，升级不受 disable-command 开关限制。
	if task.SignTs == 0 || time.Since(time.Unix(task.SignTs, 0)) > 5*time.Minute {
		res.Error = "upgrade signature expired"
		return
	}
	if !protocol.VerifyExec(e.secret, task.TaskID, task.Target, task.Nonce, task.SignTs, task.Sign) {
		res.Error = "invalid upgrade signature"
		return
	}
	if !strings.HasPrefix(task.Target, "https://") {
		res.Error = "upgrade url must be https"
		return
	}

	exe, err := selfPath()
	if err != nil {
		res.Error = "locate self: " + err.Error()
		return
	}

	tmp := exe + ".new"
	if err := download(ctx, task.Target, tmp); err != nil {
		res.Error = "download: " + err.Error()
		return
	}

	// 自检：新二进制能正常输出版本号才算有效，防止下到损坏文件。
	verOut, err := exec.CommandContext(ctx, tmp, "-version").Output()
	if err != nil {
		_ = os.Remove(tmp)
		res.Error = "new binary self-check failed: " + err.Error()
		return
	}
	newVer := strings.TrimSpace(string(verOut))

	// Windows 上运行中的 exe 不能覆盖但可以改名，先把自己挪开再放新文件。
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(tmp)
		res.Error = "backup current binary: " + err.Error()
		return
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Rename(old, exe) // 回滚
		res.Error = "replace binary: " + err.Error()
		return
	}

	res.Success = true
	res.Output = fmt.Sprintf("upgraded %s -> %s, restarting", Version, newVer)
	log.Printf("upgrade done (%s -> %s), exiting for restart", Version, newVer)

	// 留出时间把结果发回面板，再退出交给 systemd 拉起新版本。
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}

// download 把 url 内容写到 path，并赋予可执行权限。
func download(ctx context.Context, url, path string) error {
	dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(path)
		return err
	}
	return f.Close()
}
