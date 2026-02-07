//go:build !dev

package server

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// spaHandler returns a handler that serves the embedded SPA.
// Unknown paths fall back to index.html for client-side routing.
func spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("embed: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for unknown paths
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "UI not built. Run: make ui-build", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}
