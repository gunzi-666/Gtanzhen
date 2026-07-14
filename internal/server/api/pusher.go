package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var browserUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// pusher 把精简后的公开状态以固定节流推送给所有浏览器订阅者。
// 数据构建复用 API.publicServers，与 REST 接口暴露的字段严格一致。
type pusher struct {
	api     *API
	mu      sync.RWMutex
	clients map[*browserClient]struct{}
}

type browserClient struct {
	ws   *websocket.Conn
	send chan []byte
}

func newPusher(a *API) *pusher {
	return &pusher{api: a, clients: make(map[*browserClient]struct{})}
}

// frame 构建一帧推送数据。
func (p *pusher) frame() ([]byte, error) {
	states, err := p.api.publicServers()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"type": "servers", "data": states})
}

// start 启动 3 秒节流的广播循环。
func (p *pusher) start() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			p.broadcast()
		}
	}()
}

func (p *pusher) broadcast() {
	p.mu.RLock()
	n := len(p.clients)
	p.mu.RUnlock()
	if n == 0 {
		return
	}
	data, err := p.frame()
	if err != nil {
		return
	}
	p.mu.RLock()
	for c := range p.clients {
		select {
		case c.send <- data:
		default:
			// 客户端积压，丢弃这一帧。
		}
	}
	p.mu.RUnlock()
}

func (p *pusher) handle(w http.ResponseWriter, r *http.Request) {
	ws, err := browserUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &browserClient{ws: ws, send: make(chan []byte, 8)}
	p.mu.Lock()
	p.clients[c] = struct{}{}
	p.mu.Unlock()

	// 连接建立后立即推一帧。
	if data, err := p.frame(); err == nil {
		c.send <- data
	}

	go p.writeLoop(c)
	p.readLoop(c)
}

func (p *pusher) writeLoop(c *browserClient) {
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ping.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (p *pusher) readLoop(c *browserClient) {
	defer func() {
		p.mu.Lock()
		delete(p.clients, c)
		p.mu.Unlock()
		close(c.send)
		c.ws.Close()
	}()
	for {
		if _, _, err := c.ws.ReadMessage(); err != nil {
			return
		}
	}
}
