package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNumericConnectionCredentialRejectedBeforeProbeAndFingerprint(t *testing.T) {
	const secret = "9007199254740993"
	var requests atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer endpoint.Close()
	for _, option := range []any{int64(9007199254740993), map[string]any{"nested": []any{int64(9007199254740993)}}} {
		_, _, server, now := baselineFixture(t)
		server.URL = endpoint.URL
		server.Options = map[string]map[string]any{"claude": {"timeout": option}}
		if err := server.Validate("echo"); err != nil {
			t.Fatalf("unsupported regression fixture: %v", err)
		}
		now.Secrets = []string{secret}
		if !serverHasKnownSecret(server, "", "echo", now.Secrets) {
			t.Fatal("numeric credential passed connection preflight")
		}
		p := newLiveProbe(server, "", DefaultLiveTimeout, CheckOptions{knownSecrets: now.Secrets})
		if live, err := p.probe(); err == nil || live != nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "connection identity") {
			t.Fatal("numeric credential reached live transport")
		}
		if identity, err := toolConnectionIdentity(server, now); err == nil || identity != "" || strings.Contains(err.Error(), secret) {
			t.Fatal("numeric credential produced a connection fingerprint")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("numeric connection credential reached the endpoint")
	}
}
