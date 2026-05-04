package ui

import "net/http"

// FrontendHandler returns the embedded React bundle. For Day 6 (backend
// only) this is a static placeholder so curl tests against /api/* work
// while the frontend is being scaffolded. Day 10 swaps this for a real
// //go:embed all:frontend/dist + http.FileServer.
func FrontendHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(placeholderHTML))
	})
}

const placeholderHTML = `<!doctype html>
<html><head><title>mcpscope</title>
<style>body{font:14px/1.5 -apple-system,system-ui,sans-serif;max-width:640px;margin:4rem auto;padding:0 1rem;color:#333}
code{background:#f4f4f4;padding:2px 6px;border-radius:3px;font:13px ui-monospace,monospace}</style>
</head><body>
<h1>mcpscope</h1>
<p>Backend is up. The web UI is being scaffolded — for now use the JSON API:</p>
<ul>
<li><code>GET /api/sessions?limit=50</code></li>
<li><code>GET /api/sessions/{id}/messages?from=0</code></li>
<li><code>GET /api/sessions/{id}/live</code> (SSE)</li>
</ul>
<p>Or fall back to the CLI: <code>mcpscope sessions</code>, <code>mcpscope show &lt;id&gt;</code>.</p>
</body></html>
`
