//go:build !embedui

package api

import "net/http"

// staticHandler without the embedui build tag serves a development notice.
// The Docker build compiles with -tags embedui and real assets.
func staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><title>ArrayDeck</title>
<body style="font-family:system-ui;padding:3rem;color:#182230;background:#f3f5f8">
<h1>ArrayDeck API</h1>
<p>The web UI is not embedded in this binary. Run the Vite dev server
(<code>cd web && npm run dev</code>) or build with <code>-tags embedui</code>.</p>
</body>`))
	})
}
