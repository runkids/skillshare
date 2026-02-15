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
<head><title>Skillshare UI</title></head>
<body>
<h1>Skillshare UI</h1>
<p>The frontend is served by Vite at <a href="http://localhost:5173">localhost:5173</a> in dev mode.</p>
<p>In production, run <code>skillshare ui</code> to download and launch the web dashboard.</p>
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
