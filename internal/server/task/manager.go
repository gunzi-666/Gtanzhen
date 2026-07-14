// Package task 负责向 Agent 下发一次性任务（探测/命令）并把结果路由回调用方。
package task

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"probe/internal/protocol"
)

// Dispatcher 抽象“把任务送达某台 Agent”的能力（由 hub 实现）。
type Dispatcher interface {
	Dispatch(serverID uint64, task protocol.TaskDispatch) bool
}

// SecretFunc 返回某服务器的 secret（用于命令签名）。
type SecretFunc func(serverID uint64) (string, error)

// Manager 管理待回报的任务。
type Manager struct {
	dispatcher Dispatcher
	secretOf   SecretFunc

	mu      sync.Mutex
	pending map[string]chan protocol.TaskResult
}

// NewManager 创建任务管理器。
func NewManager(d Dispatcher, secretOf SecretFunc) *Manager {
	return &Manager{
		dispatcher: d,
		secretOf:   secretOf,
		pending:    make(map[string]chan protocol.TaskResult),
	}
}

// OnResult 是注册到 hub 的任务结果回调。
func (m *Manager) OnResult(serverID uint64, res protocol.TaskResult) {
	m.mu.Lock()
	ch, ok := m.pending[res.TaskID]
	if ok {
		delete(m.pending, res.TaskID)
	}
	m.mu.Unlock()
	if ok {
		select {
		case ch <- res:
		default:
		}
	}
}

var (
	// ErrOffline 目标服务器不在线，无法下发。
	ErrOffline = errors.New("server offline")
	// ErrTimeout 等待结果超时。
	ErrTimeout = errors.New("task timeout")
)

func newTaskID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Probe 下发一个探测任务（ping/tcping/http_get）并等待结果。
func (m *Manager) Probe(serverID uint64, typ protocol.TaskType, target string, timeout int) (protocol.TaskResult, error) {
	task := protocol.TaskDispatch{
		TaskID:  newTaskID(),
		Type:    typ,
		Target:  target,
		Timeout: timeout,
	}
	return m.dispatchAndWait(serverID, task, timeout)
}

// RunCommand 下发一条已签名的远程执行命令并等待输出。
func (m *Manager) RunCommand(serverID uint64, command string, timeout int) (string, error) {
	secret, err := m.secretOf(serverID)
	if err != nil {
		return "", err
	}
	if timeout <= 0 {
		timeout = 60
	}
	taskID := newTaskID()
	nonce := newTaskID()
	signTs := time.Now().Unix()
	task := protocol.TaskDispatch{
		TaskID:  taskID,
		Type:    protocol.TaskExecCommand,
		Target:  command,
		Timeout: timeout,
		Nonce:   nonce,
		SignTs:  signTs,
		Sign:    protocol.SignExec(secret, taskID, command, nonce, signTs),
	}
	res, err := m.dispatchAndWait(serverID, task, timeout)
	if err != nil {
		return "", err
	}
	if !res.Success && res.Error != "" {
		return res.Output, errors.New(res.Error)
	}
	return res.Output, nil
}

// Upgrade 下发一条已签名的 Agent 自升级任务并等待结果。
// url 是新二进制的下载地址，Agent 替换自身后退出由 systemd 拉起。
func (m *Manager) Upgrade(serverID uint64, url string, timeout int) (string, error) {
	secret, err := m.secretOf(serverID)
	if err != nil {
		return "", err
	}
	if timeout <= 0 {
		timeout = 180
	}
	taskID := newTaskID()
	nonce := newTaskID()
	signTs := time.Now().Unix()
	task := protocol.TaskDispatch{
		TaskID:  taskID,
		Type:    protocol.TaskUpgrade,
		Target:  url,
		Timeout: timeout,
		Nonce:   nonce,
		SignTs:  signTs,
		Sign:    protocol.SignExec(secret, taskID, url, nonce, signTs),
	}
	res, err := m.dispatchAndWait(serverID, task, timeout)
	if err != nil {
		return "", err
	}
	if !res.Success {
		return res.Output, errors.New(res.Error)
	}
	return res.Output, nil
}

// dispatchAndWait 送达任务并阻塞等待结果。
func (m *Manager) dispatchAndWait(serverID uint64, task protocol.TaskDispatch, timeout int) (protocol.TaskResult, error) {
	ch := make(chan protocol.TaskResult, 1)
	m.mu.Lock()
	m.pending[task.TaskID] = ch
	m.mu.Unlock()

	if !m.dispatcher.Dispatch(serverID, task) {
		m.mu.Lock()
		delete(m.pending, task.TaskID)
		m.mu.Unlock()
		return protocol.TaskResult{}, ErrOffline
	}

	wait := time.Duration(timeout)*time.Second + 10*time.Second
	select {
	case res := <-ch:
		return res, nil
	case <-time.After(wait):
		m.mu.Lock()
		delete(m.pending, task.TaskID)
		m.mu.Unlock()
		return protocol.TaskResult{}, ErrTimeout
	}
}
