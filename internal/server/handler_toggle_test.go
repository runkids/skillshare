package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillshare/internal/oplog"
)

func TestHandleDisableSkill_OverriddenByLocalNegation_Conflict(t *testing.T) {
	s, src := newTestServer(t)
	seedLocalNegation(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/resources/feature-radar__feature-radar/disable", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), ".skillignore.local") {
		t.Fatalf("expected 409 naming .skillignore.local, got %d: %s", rr.Code, rr.Body.String())
	}
}

// The conflict still leaves the pattern in .skillignore, so the change must be logged.
func TestHandleDisableSkill_OverriddenByLocalNegation_LogsError(t *testing.T) {
	s, src := newTestServer(t)
	seedLocalNegation(t, src)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/resources/feature-radar__feature-radar/disable", nil))

	entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if len(entries) != 1 || entries[0].Command != "disable" || entries[0].Status != "error" {
		t.Fatalf("expected one disable error entry, got %+v", entries)
	}
}
