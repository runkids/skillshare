package skillserve

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPHandler serves srv over Streamable HTTP, and the Executor skill index
// under IndexPath. A non-empty token must be sent as a bearer token on every
// request. Cross-origin browser requests to MCP are refused. Without a token,
// a loopback server also refuses other host names, so a DNS rebinding page cannot
// use it; with one, the token proves that, and a TLS proxy may keep its public Host.
func HTTPHandler(srv *Server, token string) http.Handler {
	h := httpHandler(srv, token != "")
	if token == "" {
		return h
	}
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// httpHandler shares the SDK origin/Host policy after the owning router has
// authenticated the request. It never exposes an unauthenticated public bypass.
func httpHandler(srv *Server, authenticated bool) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv.Server }, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		CrossOriginProtection:      http.NewCrossOriginProtection(),
		DisableLocalhostProtection: authenticated,
	})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, IndexPath) {
			mcpHandler.ServeHTTP(w, r)
			return
		}
		// Reading needs no CSRF check: without CORS headers a page cannot read the answer.
		if !authenticated && rebound(r) {
			http.Error(w, fmt.Sprintf("Forbidden: invalid Host header %q", r.Host), http.StatusForbidden)
			return
		}
		srv.svc.serveIndex(w, r)
	})

	if !authenticated {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-cache")
		h.ServeHTTP(&privateResponseWriter{ResponseWriter: w}, r)
	})
}

// The SDK sets its streaming directives late. Enforce privacy when headers are
// committed, retaining no-transform and forwarding streaming flushes/controllers.
type privateResponseWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *privateResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *privateResponseWriter) WriteHeader(status int) {
	if !w.committed && status >= 200 {
		if cache := strings.Join(w.Header().Values("Cache-Control"), ", "); !strings.Contains(cache, "private") {
			w.Header().Set("Cache-Control", "private, "+cache)
		}
		w.committed = true
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *privateResponseWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *privateResponseWriter) FlushError() error {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *privateResponseWriter) Flush() { _ = w.FlushError() }
