// Package tgbot 实现 Telegram Bot 命令查询：长轮询已绑定的 Bot，
// 支持 /overview 总览与 /server 单台状态，仅响应绑定的 Chat。
package tgbot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"probe/internal/protocol"
	"probe/internal/server/hub"
	"probe/internal/server/store"
)

const (
	keyTGToken = "tg_bot_token"
	keyTGChat  = "tg_chat_id"
)

var pollClient = &http.Client{Timeout: 40 * time.Second}

// Run 启动 Bot 命令轮询。未绑定时空转等待，绑定后立即生效，无需重启。
func Run(st *store.Store, h *hub.Hub) {
	go loop(st, h)
}

func loop(st *store.Store, h *hub.Hub) {
	var offset int64
	lastToken := ""
	for {
		token := st.GetSetting(keyTGToken, "")
		chat := st.GetSetting(keyTGChat, "")
		if token == "" || chat == "" {
			time.Sleep(10 * time.Second)
			continue
		}
		if token != lastToken {
			offset = 0
			lastToken = token
		}
		updates, err := getUpdates(token, offset)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			// 只响应绑定的会话，其他人发消息一律忽略。
			if strconv.FormatInt(u.Message.Chat.ID, 10) != chat {
				continue
			}
			reply := handleCommand(st, h, u.Message.Text)
			if reply != "" {
				_ = sendMessage(token, chat, reply)
			}
		}
	}
}

// sendMessage 直接发送纯文本回复（不带标题行）。
func sendMessage(token, chat, text string) error {
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	form := url.Values{}
	form.Set("chat_id", chat)
	form.Set("text", text)
	resp, err := pollClient.PostForm(api, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram status %d", resp.StatusCode)
	}
	return nil
}

// ==== Telegram getUpdates ====

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func getUpdates(token string, offset int64) ([]tgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=25&allowed_updates=[\"message\"]", token, offset)
	resp, err := pollClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		OK     bool       `json:"ok"`
		Result []tgUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram getUpdates not ok")
	}
	return out.Result, nil
}

// ==== 命令处理 ====

func handleCommand(st *store.Store, h *hub.Hub, text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ""
	}
	// 去掉群里 @botname 后缀。
	cmd := strings.ToLower(strings.SplitN(fields[0], "@", 2)[0])
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), fields[0]))

	switch cmd {
	case "/start", "/help", "帮助":
		return helpText()
	case "/overview", "/all", "总览":
		return overview(st, h)
	case "/server", "/status", "状态":
		if arg == "" {
			return "用法：/server 名称或ID\n例如 /server 1 或 /server hk-1\n\n" + serverNames(st)
		}
		return serverDetail(st, h, arg)
	default:
		if strings.HasPrefix(cmd, "/") {
			return "未知命令。\n\n" + helpText()
		}
		return "" // 普通聊天消息不回复
	}
}

func helpText() string {
	return "可用命令：\n" +
		"/overview - 所有服务器总览\n" +
		"/server 名称或ID - 单台服务器详细状态\n" +
		"/help - 显示本帮助"
}

func serverNames(st *store.Store) string {
	servers, err := st.ListServers()
	if err != nil || len(servers) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("已登记的服务器：\n")
	for _, s := range servers {
		fmt.Fprintf(&b, "%d. %s\n", s.ID, s.Name)
	}
	return b.String()
}

// overview 汇总所有在线服务器的资源占用，不逐台罗列。
func overview(st *store.Store, h *hub.Hub) string {
	servers, err := st.ListServers()
	if err != nil {
		return "查询失败：" + err.Error()
	}
	if len(servers) == 0 {
		return "还没有登记任何服务器。"
	}
	states := map[uint64]hub.State{}
	for _, s := range h.Snapshot() {
		states[s.ServerID] = s
	}

	var (
		online              int
		totalCores          int
		usedCores           float64 // 各服务器 cpu% × 核数 的累加，用于算加权总占用
		memTotal, memUsed   uint64
		diskTotal, diskUsed uint64
		netIn, netOut       uint64 // 实时速率总和 B/s
		trafIn, trafOut     uint64 // 本月流量总和
	)
	ym := time.Now().Format("2006-01")
	for _, s := range servers {
		in, out := st.TrafficMonth(s.ID, ym)
		trafIn += in
		trafOut += out

		stt, ok := states[s.ID]
		if !ok || !stt.Online {
			continue
		}
		online++
		cores := hostCores(stt.Host)
		totalCores += cores
		if stt.Host != nil {
			memTotal += stt.Host.MemTotal
			diskTotal += stt.Host.DiskTotal
		}
		if m := stt.Metrics; m != nil {
			usedCores += m.CPU / 100 * float64(cores)
			memUsed += m.MemUsed
			diskUsed += m.DiskUsed
			netIn += m.NetInSpeed
			netOut += m.NetOutSpeed
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "服务器总览\n\n")
	fmt.Fprintf(&b, "服务器：%d 台（🟢 %d 在线 / 🔴 %d 离线）\n", len(servers), online, len(servers)-online)
	if totalCores > 0 {
		fmt.Fprintf(&b, "CPU：%d 核，总占用 %.1f%%\n", totalCores, usedCores/float64(totalCores)*100)
	}
	if memTotal > 0 {
		fmt.Fprintf(&b, "内存：%s / %s（%.1f%%）\n", fmtBytes(memUsed), fmtBytes(memTotal),
			float64(memUsed)/float64(memTotal)*100)
	}
	if diskTotal > 0 {
		fmt.Fprintf(&b, "磁盘：%s / %s（%.1f%%）\n", fmtBytes(diskUsed), fmtBytes(diskTotal),
			float64(diskUsed)/float64(diskTotal)*100)
	}
	fmt.Fprintf(&b, "网速：↓%s/s ↑%s/s\n", fmtBytes(netIn), fmtBytes(netOut))
	fmt.Fprintf(&b, "本月流量：↓%s ↑%s", fmtBytes(trafIn), fmtBytes(trafOut))
	return b.String()
}

// hostCores 从上报的 CPU 描述（"型号 N Cores" xM 条）里累加核心数。
func hostCores(hi *protocol.HostInfo) int {
	if hi == nil {
		return 0
	}
	total := 0
	for _, s := range hi.CPU {
		fields := strings.Fields(s)
		// 末尾两段固定是 "N Cores"。
		if len(fields) >= 2 && strings.HasPrefix(strings.ToLower(fields[len(fields)-1]), "core") {
			if n, err := strconv.Atoi(fields[len(fields)-2]); err == nil {
				total += n
			}
		}
	}
	return total
}

func serverDetail(st *store.Store, h *hub.Hub, arg string) string {
	servers, err := st.ListServers()
	if err != nil {
		return "查询失败：" + err.Error()
	}
	var target *store.Server
	if id, e := strconv.ParseUint(arg, 10, 64); e == nil {
		for i := range servers {
			if servers[i].ID == id {
				target = &servers[i]
				break
			}
		}
	}
	if target == nil {
		low := strings.ToLower(arg)
		for i := range servers {
			if strings.ToLower(servers[i].Name) == low {
				target = &servers[i]
				break
			}
		}
	}
	if target == nil {
		return "找不到服务器「" + arg + "」。\n\n" + serverNames(st)
	}

	stt, ok := h.StateOf(target.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "名称：%s (ID %d)\n", target.Name, target.ID)
	if !ok || !stt.Online {
		b.WriteString("状态：🔴 离线\n")
		if ok && !stt.LastSeen.IsZero() {
			fmt.Fprintf(&b, "最后上报：%s\n", stt.LastSeen.Format("2006-01-02 15:04:05"))
		}
		return b.String()
	}
	b.WriteString("状态：🟢 在线\n")
	if m := stt.Metrics; m != nil {
		if cores := hostCores(stt.Host); cores > 0 {
			fmt.Fprintf(&b, "CPU：%.1f%%（%d 核）\n", m.CPU, cores)
		} else {
			fmt.Fprintf(&b, "CPU：%.1f%%\n", m.CPU)
		}
		if stt.Host != nil && stt.Host.MemTotal > 0 {
			fmt.Fprintf(&b, "内存：%s / %s（%.0f%%）\n", fmtBytes(m.MemUsed), fmtBytes(stt.Host.MemTotal),
				float64(m.MemUsed)/float64(stt.Host.MemTotal)*100)
		}
		fmt.Fprintf(&b, "网络：↓%s/s ↑%s/s\n", fmtBytes(m.NetInSpeed), fmtBytes(m.NetOutSpeed))
		if stt.Host != nil && stt.Host.DiskTotal > 0 {
			fmt.Fprintf(&b, "磁盘：%s / %s（%.0f%%）\n", fmtBytes(m.DiskUsed), fmtBytes(stt.Host.DiskTotal),
				float64(m.DiskUsed)/float64(stt.Host.DiskTotal)*100)
		}
		fmt.Fprintf(&b, "开机：%s\n", fmtUptime(m.Uptime))
	}
	return b.String()
}

func fmtBytes(n uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func fmtUptime(sec uint64) string {
	d := sec / 86400
	h := sec % 86400 / 3600
	m := sec % 3600 / 60
	if d > 0 {
		return fmt.Sprintf("%d天%d小时", d, h)
	}
	if h > 0 {
		return fmt.Sprintf("%d小时%d分", h, m)
	}
	return fmt.Sprintf("%d分", m)
}
