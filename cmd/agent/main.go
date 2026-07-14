// Agent 探针端入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"probe/internal/agent"
)

// version 由构建时通过 -ldflags "-X main.version=..." 注入。
var version = "dev"

func main() {
	var cfg agent.Config
	flag.StringVar(&cfg.Server, "server", envOr("PROBE_SERVER", "ws://127.0.0.1:8008/api/agent"), "面板 WebSocket 地址")
	flag.StringVar(&cfg.Secret, "secret", os.Getenv("PROBE_SECRET"), "本机 secret")
	flag.IntVar(&cfg.ReportPeriod, "period", 2, "指标上报间隔（秒）")
	flag.BoolVar(&cfg.DisableCommand, "disable-command", false, "禁用远程执行命令")
	showVersion := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if cfg.Secret == "" {
		log.Fatal("secret 不能为空，请用 -secret 或环境变量 PROBE_SECRET 指定")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("probe agent %s starting, server=%s", version, cfg.Server)
	agent.NewClient(cfg).Run(ctx)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
