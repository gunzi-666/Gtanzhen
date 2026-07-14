package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(v)
}

// withCORS 允许同源前端与开发期跨域调试。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler 提供内嵌前端，找不到的路径回退到 index.html 以支持前端路由。
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := path.Clean(r.URL.Path)
		if f, err := fsys.Open(upath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(upath, "/api/") {
			http.NotFound(w, r)
			return
		}
		// 回退到 index.html。
		index, err := fsys.Open("/index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusNotFound)
			return
		}
		defer index.Close()
		stat, _ := index.Stat()
		if stat != nil {
			http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
			return
		}
		http.NotFound(w, r)
	})
}
