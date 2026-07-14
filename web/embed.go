// Package web 内嵌构建后的前端静态资源。
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// dist 目录由前端构建（vite build）产生。使用 all: 以包含带下划线开头的文件。
//
//go:embed all:dist
var distFS embed.FS

// Dist 返回内嵌前端的文件系统；若未构建则返回 nil，Server 仅提供 API。
func Dist() http.FileSystem {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	// 检测 index.html 是否存在，不存在说明尚未构建。
	if _, err := sub.Open("index.html"); err != nil {
		return nil
	}
	return http.FS(sub)
}
