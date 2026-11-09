package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectCrossSiteWrites(t *testing.T) {
	tests := []struct {
		name, method, origin, fetchSite string
		want                            int
	}{
		{"dashboard fetch", http.MethodPost, "http://127.0.0.1:19420", "same-origin", http.StatusOK},
		{"client without browser headers", http.MethodPost, "", "", http.StatusOK},
		{"same-origin metadata without Origin", http.MethodDelete, "", "same-origin", http.StatusOK},
		{"read from another site", http.MethodGet, "https://evil.example", "cross-site", http.StatusOK},
		{"form or no-cors fetch from another site", http.MethodPost, "https://evil.example", "cross-site", http.StatusForbidden},
		{"another site without Fetch Metadata", http.MethodPost, "https://evil.example", "", http.StatusForbidden},
		{"another port on the same host", http.MethodPut, "http://127.0.0.1:3000", "same-site", http.StatusForbidden},
		{"another port without Fetch Metadata", http.MethodPost, "http://127.0.0.1:3000", "", http.StatusForbidden},
		{"sandboxed or opaque origin", http.MethodPost, "null", "", http.StatusForbidden},
		{"cross-site metadata with a matching Origin", http.MethodPost, "http://127.0.0.1:19420", "cross-site", http.StatusForbidden},
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/targets", nil)
			req.Host = "127.0.0.1:19420"
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			rr := httptest.NewRecorder()
			rejectCrossSiteWrites(ok).ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

// A page on another site can send a text/plain POST without a CORS preflight; the dashboard
// must refuse it before any handler runs.
func TestCrossSitePostDoesNotAddATarget(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(`{"name":"evil","path":"/tmp/evil-target"}`))
	req.Host = "127.0.0.1:19420"
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if _, added := s.cfg.Targets["evil"]; rr.Code != http.StatusForbidden || added {
		t.Fatalf("status = %d, target added = %v: %s", rr.Code, added, rr.Body.String())
	}
}

// The update stream runs updates over GET, because EventSource can only GET, so another
// site must not start it either.
func TestCrossSiteUpdateStreamIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/update/stream?force=true&skipAudit=true", nil)
	req.Host = "127.0.0.1:19420"
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
}
