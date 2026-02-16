package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// uiPlaceholderHandler returns a handler for when no UI dist is available.
// Used in dev mode (Vite on :5173) or API-only scenarios.
func uiPlaceholderHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Skillshare UI (dev mode)</title>
<style>body{font-family:system-ui,sans-serif;max-width:600px;margin:60px auto;padding:0 20px;color:#333}
code{background:#f0f0f0;padding:2px 6px;border-radius:3px;font-size:0.9em}
pre{background:#f0f0f0;padding:12px 16px;border-radius:6px;overflow-x:auto}</style>
</head>
<body>
<h1>Skillshare UI — Dev Mode</h1>
<p>This is the API server. The frontend needs to be started separately:</p>
<pre>cd ui && pnpm run dev</pre>
<p>Then open <a href="http://localhost:5173">http://localhost:5173</a> (Vite proxies <code>/api</code> to this server).</p>
<p>Or use <code>make ui-dev</code> to start both together.</p>
<hr>
<p style="color:#888;font-size:0.85em">In production builds, <code>skillshare ui</code> downloads and serves the frontend automatically.</p>
</body>
</html>`))
	})
}

// spaHandlerFromDisk serves a SPA from a directory on disk.
// Unknown paths fall back to index.html for client-side routing.
func spaHandlerFromDisk(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		fullPath := filepath.Join(dir, path)
		if _, err := os.Stat(fullPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html
		indexPath := filepath.Join(dir, "index.html")
		index, err := os.ReadFile(indexPath)
		if err != nil {
			http.Error(w, "UI assets not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}
