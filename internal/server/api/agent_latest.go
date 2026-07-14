package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// 最新 Release tag 的内存缓存，避免每次列表都打 GitHub API（未认证限流 60 次/小时）。
var (
	latestMu      sync.Mutex
	latestVersion string
	latestAt      time.Time
)

const latestTTL = 10 * time.Minute

var latestClient = &http.Client{Timeout: 5 * time.Second}

// latestAgentVersion 返回仓库最新 Release 的 tag（如 v1.0.7）。
// 未配置仓库或查询失败返回空串，调用方按“未知”处理。
func (a *API) latestAgentVersion() string {
	repo := a.deps.Store.GetSetting(settingRepo, "")
	if repo == "" {
		return ""
	}
	latestMu.Lock()
	defer latestMu.Unlock()
	if latestVersion != "" && time.Since(latestAt) < latestTTL {
		return latestVersion
	}
	resp, err := latestClient.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return latestVersion // 失败时沿用旧值（可能为空）
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return latestVersion
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.TagName == "" {
		return latestVersion
	}
	latestVersion = body.TagName
	latestAt = time.Now()
	return latestVersion
}

// handleAgentLatest 返回最新 Agent 版本号，供后台判断是否显示升级按钮。
func (a *API) handleAgentLatest(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": a.latestAgentVersion()})
}
