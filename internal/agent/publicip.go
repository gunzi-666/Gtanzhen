package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 公网 IP 回显服务，依次尝试，任一成功即返回。
// 前三个双栈（靠强制拨号协议族区分 v4/v6），最后一个 AWS 官方仅 IPv4（纯兜底）。
var ipEchoServices = []string{
	"https://icanhazip.com",         // Cloudflare 运营
	"https://api64.ipify.org",       // ipify
	"https://ident.me",              // 备用
	"https://checkip.amazonaws.com", // AWS 官方，仅 IPv4
}

// ipCache 缓存探测结果，避免 Agent 断线重连风暴时频繁请求外部服务。
type ipCache struct {
	mu   sync.Mutex
	v4   string
	v6   string
	at   time.Time
}

var pubIPCache ipCache

// PublicIPs 返回本机公网 IPv4 与 IPv6（探测失败的一侧为空字符串）。
// 结果缓存 10 分钟。
func PublicIPs() (ipv4, ipv6 string) {
	pubIPCache.mu.Lock()
	defer pubIPCache.mu.Unlock()
	if time.Since(pubIPCache.at) < 10*time.Minute {
		return pubIPCache.v4, pubIPCache.v6
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ipv4 = fetchPublicIP(ctx, "tcp4") }()
	go func() { defer wg.Done(); ipv6 = fetchPublicIP(ctx, "tcp6") }()
	wg.Wait()

	pubIPCache.v4, pubIPCache.v6, pubIPCache.at = ipv4, ipv6, time.Now()
	return ipv4, ipv6
}

// fetchPublicIP 强制以指定协议族（tcp4/tcp6）访问回显服务获取公网 IP。
func fetchPublicIP(ctx context.Context, network string) string {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
	defer client.CloseIdleConnections()

	for _, url := range ipEchoServices {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(string(body)))
		if ip == nil {
			continue
		}
		// 确认返回的地址协议族与请求一致，防回显服务异常。
		if (network == "tcp4") == (ip.To4() != nil) {
			return ip.String()
		}
	}
	return ""
}
