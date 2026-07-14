// Package alert 实现告警规则引擎与通知渠道。
package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"probe/internal/server/store"
)

// Sender 抽象一个可发送文本消息的通知渠道。
type Sender interface {
	Send(title, body string) error
}

// BuildSender 根据通知渠道配置构造 Sender。
func BuildSender(n *store.Notification) (Sender, error) {
	switch n.Type {
	case "telegram":
		var c telegramConfig
		if err := json.Unmarshal([]byte(n.Config), &c); err != nil {
			return nil, err
		}
		return &c, nil
	case "email":
		var c emailConfig
		if err := json.Unmarshal([]byte(n.Config), &c); err != nil {
			return nil, err
		}
		return &c, nil
	case "webhook":
		var c webhookConfig
		if err := json.Unmarshal([]byte(n.Config), &c); err != nil {
			return nil, err
		}
		return &c, nil
	default:
		return nil, fmt.Errorf("unknown notification type: %s", n.Type)
	}
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// SendTelegram 直接用指定的 Bot 向指定会话发送一条消息（供绑定/验证码等场景复用）。
func SendTelegram(botToken, chatID, title, body string) error {
	c := telegramConfig{BotToken: botToken, ChatID: chatID}
	return c.Send(title, body)
}

// telegramConfig Telegram Bot 配置。
type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func (c *telegramConfig) Send(title, body string) error {
	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.BotToken)
	form := url.Values{}
	form.Set("chat_id", c.ChatID)
	form.Set("text", title+"\n\n"+body)
	resp, err := httpClient.PostForm(api, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram status %d", resp.StatusCode)
	}
	return nil
}

// emailConfig SMTP 邮件配置。
type emailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"` // 逗号分隔
}

func (c *emailConfig) Send(title, body string) error {
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	auth := smtp.PlainAuth("", c.Username, c.Password, c.Host)
	to := splitComma(c.To)
	from := c.From
	if from == "" {
		from = c.Username
	}
	msg := bytes.Buffer{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + strings.Join(to, ",") + "\r\n")
	msg.WriteString("Subject: " + title + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)
	return smtp.SendMail(addr, auth, from, to, msg.Bytes())
}

// webhookConfig 通用 Webhook 配置。
// Body 支持占位符 {{title}} 和 {{body}}；为空时发送标准 JSON。
type webhookConfig struct {
	URL         string `json:"url"`
	Method      string `json:"method"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
}

func (c *webhookConfig) Send(title, body string) error {
	method := c.Method
	if method == "" {
		method = http.MethodPost
	}
	var payload string
	if c.Body != "" {
		payload = strings.ReplaceAll(c.Body, "{{title}}", jsonEscape(title))
		payload = strings.ReplaceAll(payload, "{{body}}", jsonEscape(body))
	} else {
		b, _ := json.Marshal(map[string]string{"title": title, "body": body})
		payload = string(b)
	}
	ct := c.ContentType
	if ct == "" {
		ct = "application/json"
	}
	req, err := http.NewRequest(method, c.URL, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ct)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// jsonEscape 转义字符串以安全嵌入 JSON 模板（去掉首尾引号）。
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
