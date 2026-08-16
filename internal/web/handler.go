package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/*
var embedded embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested != "." && requested != "" {
			if info, err := fs.Stat(root, requested); err == nil && !info.IsDir() {
				if strings.Contains(path.Base(requested), ".") && requested != "index.html" {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		// BrowserRouter paths always receive index.html so a refresh restores the
		// exact menu/page instead of falling through to a server 404.
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}
