package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"probe/internal/protocol"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// conn 封装一条 Agent 连接。
type conn struct {
	ws       *websocket.Conn
	serverID uint64
	connID   uint64
	writeMu  sync.Mutex
}

// HandleWS 是 Agent 接入的 HTTP 处理器（挂在 /api/agent）。
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	// 第一条必须是认证。
	ws.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		return
	}
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type != protocol.TypeAuth {
		return
	}
	var auth protocol.AuthRequest
	if err := msg.Decode(&auth); err != nil {
		return
	}
	id, name, ok := h.auth(auth.Secret)
	result := protocol.AuthResult{OK: ok, ServerID: id, ReportPeriod: h.reportPeriod}
	if !ok {
		result.Message = "invalid secret"
	}
	c := &conn{ws: ws, serverID: id}
	_ = c.sendMsg(protocol.TypeAuthResult, "", result)
	if !ok {
		return
	}

	h.register(c, name)
	defer h.unregister(c)
	log.Printf("agent online: server_id=%d name=%s from %s", id, name, r.RemoteAddr)

	ws.SetReadDeadline(time.Time{})
	ws.SetReadLimit(2 << 20) // 2MB 上限，防超大消息

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		var m protocol.Message
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		h.dispatch(c, &m)
	}
}

// register 登记连接与初始状态；同一服务器的旧连接会被顶替。
func (h *Hub) register(c *conn, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connSeq++
	c.connID = h.connSeq

	if old, ok := h.conns[c.serverID]; ok {
		_ = old.ws.Close()
	}
	h.conns[c.serverID] = c

	s, ok := h.states[c.serverID]
	if !ok {
		s = &State{ServerID: c.serverID}
		h.states[c.serverID] = s
	}
	s.Name = name
	s.Online = true
	s.LastSeen = time.Now()
	s.connID = c.connID
}

// unregister 注销连接（仅当仍是当前连接时才置离线）。
func (h *Hub) unregister(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[c.serverID]; ok && cur.connID == c.connID {
		delete(h.conns, c.serverID)
		if s, ok := h.states[c.serverID]; ok {
			s.Online = false
		}
	}
}

// dispatch 处理 Agent 上报的消息。
func (h *Hub) dispatch(c *conn, m *protocol.Message) {
	switch m.Type {
	case protocol.TypeHeartbeat:
		h.touch(c.serverID)
	case protocol.TypeHostInfo:
		var hi protocol.HostInfo
		if err := m.Decode(&hi); err == nil {
			h.setHost(c.serverID, &hi)
		}
	case protocol.TypeMetrics:
		var mt protocol.Metrics
		if err := m.Decode(&mt); err == nil {
			h.setMetrics(c.serverID, &mt)
			if h.onMetrics != nil {
				h.onMetrics(c.serverID, mt)
			}
		}
	case protocol.TypeTaskResult:
		var res protocol.TaskResult
		if err := m.Decode(&res); err == nil && h.onTaskResult != nil {
			h.onTaskResult(c.serverID, res)
		}
	}
}

func (h *Hub) touch(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.states[id]; ok {
		s.Online = true
		s.LastSeen = time.Now()
	}
}

func (h *Hub) setHost(id uint64, hi *protocol.HostInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.states[id]; ok {
		s.Host = hi
		s.LastSeen = time.Now()
	}
}

func (h *Hub) setMetrics(id uint64, m *protocol.Metrics) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.states[id]; ok {
		s.Metrics = m
		s.Online = true
		s.LastSeen = time.Now()
	}
}

// sendMsg 发送任意消息。
func (c *conn) sendMsg(t protocol.MessageType, id string, payload any) error {
	msg, err := protocol.NewMessage(t, id, time.Now().Unix(), payload)
	if err != nil {
		return err
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

// sendTask 下发任务。
func (c *conn) sendTask(task protocol.TaskDispatch) error {
	return c.sendMsg(protocol.TypeTaskDispatch, task.TaskID, task)
}
