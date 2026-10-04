package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidSourceName(t *testing.T) {
	for name, want := range map[string]bool{
		"my-skill":    true,
		"group/skill": true,
		"_repo/skill": true,
		"a/../b":      true,
		"":            false,
		"  ":          false,
		".":           false,
		"..":          false,
		"../outside":  false,
		"../../etc":   false,
		"group/../..": false,
		"/abs/path":   false,
		`..\outside`:  false,
		`C:\outside`:  false,
		"C:relative":  false,
	} {
		if got := validSourceName(name); got != want {
			t.Errorf("validSourceName(%q) = %v, want %v", name, got, want)
		}
	}
}

// addOutsideSkill creates a skill next to the source, outside it, and returns
// the name that would reach it through the source by escaping.
func addOutsideSkill(t *testing.T, sourceDir string) string {
	t.Helper()
	outside := filepath.Join(filepath.Dir(sourceDir), "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("---\nname: outside\n---\n# outside"), 0644); err != nil {
		t.Fatal(err)
	}
	return "../outside"
}

func TestHandleAuditSkill_RejectsNameOutsideSource(t *testing.T) {
	s, src := newTestServer(t)
	name := addOutsideSkill(t, src)

	req := httptest.NewRequest(http.MethodGet, "/api/audit/"+strings.ReplaceAll(name, "/", "%2F"), nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid skill name") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestHandleUpdate_RejectsNameOutsideSource(t *testing.T) {
	s, src := newTestServer(t)
	name := addOutsideSkill(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(`{"name":"`+name+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateStream_ReportsNameOutsideSourceAsError(t *testing.T) {
	s, src := newTestServer(t)
	name := addOutsideSkill(t, src)

	req := httptest.NewRequest(http.MethodGet, "/api/update/stream?names="+strings.ReplaceAll(name, "/", "%2F"), nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `"action":"error"`) || !strings.Contains(body, "invalid skill name") {
		t.Fatalf("expected an error result for %q, got: %s", name, body)
	}
}

func TestHandleBatchUninstall_RejectsNameOutsideSource(t *testing.T) {
	s, src := newTestServer(t)
	outside := filepath.Join(filepath.Dir(src), "_outside")
	if err := os.MkdirAll(filepath.Join(outside, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uninstall/batch", strings.NewReader(`{"names":["../_outside"]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with a failed item, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results []batchUninstallItemResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Success || !strings.Contains(resp.Results[0].Error, "invalid skill name") {
		t.Fatalf("unexpected results: %s", rr.Body.String())
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("directory outside the source was touched: %v", err)
	}
}
