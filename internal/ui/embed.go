package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

// frontendFS holds the compiled Vite bundle. The dist/ directory is created
// by `npm run build` (see Makefile); `go build` will fail with a clear
// error if dist/ is missing, prompting the developer to run `make build`
// first.
//
//go:embed all:frontend/dist
var frontendFS embed.FS

// FrontendHandler serves the embedded React bundle at /. Anything not
// found under dist/ falls through to a 404 — hash routing means the
// browser only ever requests "/" + "/assets/*", so no SPA fallback to
// index.html is needed.
func FrontendHandler() http.Handler {
	sub, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		// embed.FS errors at compile time, not runtime; this is unreachable.
		panic("ui: bad embed: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
