// Package expiry 实现服务器到期的每日 Telegram 提醒。
// 到期前 3 天内（含已到期），每天在设定时间通过已绑定的 TG Bot 发送一条汇总消息。
package expiry

import (
	"fmt"
	"log"
	"strings"
	"time"

	"probe/internal/server/alert"
	"probe/internal/server/store"
)

// 与 api 包共用同一批 settings 键名（值以数据库为准）。
const (
	keyEnabled = "expire_notify_enabled"
	keyTime    = "expire_notify_time"
	keyLast    = "expire_notify_last" // 上次发送日期 YYYY-MM-DD，防止重复发送
	keyTGToken = "tg_bot_token"
	keyTGChat  = "tg_chat_id"
)

// Run 启动后台循环，每 30 秒检查一次是否到达发送时间。
func Run(st *store.Store) {
	go func() {
		t := time.NewTicker(30 * time.Second)
		for range t.C {
			check(st)
		}
	}()
}

func check(st *store.Store) {
	if st.GetSetting(keyEnabled, "0") != "1" {
		return
	}
	token := st.GetSetting(keyTGToken, "")
	chat := st.GetSetting(keyTGChat, "")
	if token == "" || chat == "" {
		return
	}
	now := time.Now()
	if now.Format("15:04") != st.GetSetting(keyTime, "09:00") {
		return
	}
	today := now.Format("2006-01-02")
	if st.GetSetting(keyLast, "") == today {
		return // 今天已发过
	}

	msg := buildMessage(st, now)
	if msg == "" {
		// 没有临期服务器也记录日期，避免整分钟内反复查询。
		_ = st.SetSetting(keyLast, today)
		return
	}
	if err := alert.SendTelegram(token, chat, "服务器到期提醒", msg); err != nil {
		log.Printf("expiry notify: send telegram failed: %v", err)
		return // 发送失败不记日期，30 秒后在同一分钟内还有机会重试
	}
	_ = st.SetSetting(keyLast, today)
}

// buildMessage 汇总 3 天内到期（含已到期）的服务器，无临期项时返回空串。
func buildMessage(st *store.Store, now time.Time) string {
	servers, err := st.ListServers()
	if err != nil {
		log.Printf("expiry notify: list servers: %v", err)
		return ""
	}
	var lines []string
	for _, s := range servers {
		if s.ExpiresAt == 0 {
			continue
		}
		left := time.Unix(s.ExpiresAt, 0).Sub(now)
		days := int(left.Hours() / 24)
		switch {
		case left <= 0:
			lines = append(lines, fmt.Sprintf("· %s 已到期（%s）", s.Name, time.Unix(s.ExpiresAt, 0).Format("2006-01-02")))
		case days < 3 || (days == 3 && left.Hours() <= 72):
			lines = append(lines, fmt.Sprintf("· %s 将于 %s 到期（剩 %d 天）", s.Name, time.Unix(s.ExpiresAt, 0).Format("2006-01-02"), days+1))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n\n请及时续费。"
}
