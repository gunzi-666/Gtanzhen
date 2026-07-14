// 面板 Server 入口。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"probe/internal/protocol"
	"probe/internal/server/alert"
	"probe/internal/server/api"
	"probe/internal/server/cron"
	"probe/internal/server/expiry"
	"probe/internal/server/hub"
	"probe/internal/server/monitor"
	"probe/internal/server/store"
	"probe/internal/server/task"
	"probe/web"
)

// version 由构建时通过 -ldflags "-X main.version=..." 注入。
var version = "dev"

func main() {
	addr := flag.String("addr", envOr("PROBE_ADDR", ":8008"), "监听地址")
	dbPath := flag.String("db", envOr("PROBE_DB", "probe.db"), "SQLite 数据库路径")
	adminUser := flag.String("user", envOr("PROBE_USER", "admin"), "管理员用户名")
	adminPass := flag.String("pass", envOr("PROBE_PASS", "admin"), "管理员密码")
	period := flag.Int("period", 2, "Agent 上报间隔（秒）")
	showVersion := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	agg := store.NewAggregator(st)
	st.RollupWorker(agg)

	h := hub.New(st.AuthBySecret, *period)
	engine := alert.NewEngine(st, h)
	h.SetMetricsHandler(func(serverID uint64, m protocol.Metrics) {
		agg.Add(serverID, m)
		engine.OnMetrics(serverID, m)
	})
	h.OfflineWatcher(func(serverID uint64) {
		engine.OnOffline(serverID)
	})

	// 任务管理器：下发探测/命令并回收结果。
	tm := task.NewManager(h, st.SecretOf)
	h.SetTaskResultHandler(tm.OnResult)

	// 服务监控调度与计划任务调度。
	monitor.New(st, tm).Run()
	cronRunner := cron.New(st, tm)
	cronRunner.Start()

	// 服务器到期每日 TG 提醒。
	expiry.Run(st)

	deps := api.Deps{Hub: h, Store: st, Dispatcher: tm, Cron: cronRunner}
	a := api.New(deps, *adminUser, *adminPass)

	handler := a.Routes(web.Dist())

	log.Printf("probe server %s listening on %s (admin user: %s)", version, *addr, *adminUser)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
