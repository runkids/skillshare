package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPluginAPIRequiresPreviewAndLocalOrigin(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	for _, body := range []string{`{"request":{"action":"sync"}}`, `{"request":{"action":"sync"},"unknown":true}`} {
		w := httptest.NewRecorder()
		s.handlePluginApply(w, httptest.NewRequest(http.MethodPost, "/api/plugins/apply", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodPost, "http://attacker.example/api/plugins/apply", strings.NewReader(`{}`))
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	s.requireLocalPlugin(s.handlePluginApply)(w, r)
	if w.Code != 403 {
		t.Fatalf("rebind accepted: %d", w.Code)
	}
}
