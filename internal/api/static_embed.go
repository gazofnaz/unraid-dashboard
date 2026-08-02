//go:build embedui

package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// webdist is populated by the build pipeline (web/dist is copied here before
// compiling with -tags embedui).
//
//go:embed all:webdist
var webdist embed.FS

// staticHandler serves the compiled SPA with an index.html fallback for
// client-side routes.
func staticHandler() http.Handler {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("embedded UI is missing index.html: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" && path != "index.html" {
			if f, err := sub.Open(path); err == nil {
				f.Close()
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: serve index.html for any unknown route.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
