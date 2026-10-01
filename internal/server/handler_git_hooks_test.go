package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/hooks"
)

func TestGitHooksAPIContract(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "explicit.gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "system.gitconfig"))
	if err := os.WriteFile(os.Getenv("GIT_CONFIG_GLOBAL"), []byte("[credential]\n helper = SECRET\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mutation := `{"name":"guard","entry":{"bindings":{"git":{"commands":{"check":{"events":["pre-commit"],"command":"echo check"}}}}}}`
	w := hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":`+mutation+`}`)
	var preview struct {
		Revision string
		Files    []hooks.FileDiff
		Changes  []hooks.Change
		Blocked  bool
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.Revision == "" || preview.Blocked {
		t.Fatalf("Git preview: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "SECRET") {
		t.Fatal("private config in API preview")
	}
	w = hooksUIConfigure(s, mutation, preview.Revision, true)
	if w.Code != http.StatusOK {
		t.Fatalf("save and sync: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	s.handleHooksList(w, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	var inv hooks.Inventory
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &inv) != nil || inv.Git == nil || inv.Git.Include.Target != os.Getenv("GIT_CONFIG_GLOBAL") || !inv.Git.Include.Present || !inv.Git.Include.Owned {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	w = hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":{"name":"guard","remove":true,"unmanage":true}}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "cannot keep files") {
		t.Fatalf("keep files: %d %s", w.Code, w.Body)
	}
	// A symlink requires a manual include and produces inactive without blocking.
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "linked.gitconfig"))
	if err := os.Symlink(filepath.Join(home, "explicit.gitconfig"), os.Getenv("GIT_CONFIG_GLOBAL")); err != nil {
		t.Fatal(err)
	}
	// Existing manual include is present through the link, so it remains active.
	w = hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":{}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("symlink manual include: %d %s", w.Code, w.Body)
	}
	if err := os.WriteFile(filepath.Join(home, "empty.gitconfig"), []byte("[credential]\n helper = SECRET\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "inactive.gitconfig"))
	if err := os.Symlink(filepath.Join(home, "empty.gitconfig"), os.Getenv("GIT_CONFIG_GLOBAL")); err != nil {
		t.Fatal(err)
	}
	w = hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":{}}`)
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.Blocked {
		t.Fatalf("inactive contract: %d %s", w.Code, w.Body)
	}
	inactive := false
	for _, c := range preview.Changes {
		inactive = inactive || c.Action == "inactive"
	}
	if !inactive || strings.Contains(w.Body.String(), "SECRET") {
		t.Fatalf("inactive/private contract: %s", w.Body)
	}
}
