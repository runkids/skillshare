//go:build dev

package server

import (
	"net/http"
)

// spaHandler returns a placeholder handler when building with -tags dev.
// In dev mode, use the Vite dev server on :5173 instead.
func spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Skillshare UI (dev)</title></head>
<body>
<h1>Skillshare UI — Dev Mode</h1>
<p>The frontend is served by Vite at <a href="http://localhost:5173">localhost:5173</a>.</p>
<p>Or run <code>make ui-build</code> to embed the built frontend.</p>
</body>
</html>`))
	})
}
