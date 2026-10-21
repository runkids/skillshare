package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
)

func TestProjectRoutes_ReadOnlyLegacyLoad(t *testing.T) {
	s, source := newTestServer(t)
	path := config.ConfigPath()
	raw := []byte("source: " + filepath.ToSlash(source) + "\nmode: merge\ntargets:\n  claude:\n    path: " + filepath.ToSlash(filepath.Join(filepath.Dir(source), "target")) + "\n")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	s.cfg = cfg
	rr := projectRequest(t, s, http.MethodGet, "/api/projects", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
		t.Fatal("project list migrated user configuration")
	}
	if _, err := os.Stat(filepath.Join(config.StateDir(), "projects")); !os.IsNotExist(err) {
		t.Fatalf("read created mutation lock: %v", err)
	}
}

func TestProjectMutation_ReloadsUnderSharedLock(t *testing.T) {
	s, source := newTestServer(t)
	path := config.ConfigPath()
	release, err := projects.LockConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	root := filepath.Join(filepath.Dir(source), "new-project")
	other := filepath.Join(filepath.Dir(source), "other-project")
	for _, dir := range []string{root, other} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/projects", strings.NewReader(`{"create":true,"root":"`+filepath.ToSlash(root)+`","targets":["claude"],"skills":{}}`)))
		done <- rr
	}()
	select {
	case rr := <-done:
		t.Fatalf("project mutation ignored shared lock: %d %s", rr.Code, rr.Body)
	case <-time.After(100 * time.Millisecond):
	}
	raw := "source: " + filepath.ToSlash(source) + "\ntargets: {}\nprojects:\n  " + filepath.ToSlash(other) + ":\n    targets: [codex]\n    skills: {}\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	release()
	select {
	case rr := <-done:
		if rr.Code != http.StatusOK {
			t.Fatalf("%d %s", rr.Code, rr.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("project mutation did not release")
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Projects[other]; !ok {
		t.Fatal("reload before lock lost concurrent registration")
	}
	if _, ok := cfg.Projects[root]; !ok {
		t.Fatal("new registration missing")
	}
}

func TestProjectPartialFailure_MessageRetainsCompletedWork(t *testing.T) {
	s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	s.writeProjectFailure(rr, http.StatusInternalServerError, projects.Result{Registration: "kept", Detach: &ssync.DetachReport{Removed: []string{"/project/skills/alpha"}}}, fmt.Errorf("save denied"), time.Now(), "update", "/project")
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	message := payload["error"].(string)
	if !strings.Contains(message, "registration kept") || !strings.Contains(message, "/project/skills/alpha") {
		t.Fatal("dashboard error hides completed work:", message)
	}
}

// Saving only validates and writes, as on main; unticking a target leaves its
// folder for the user or a later remove.
func TestSaveProject_DoesNotDetach(t *testing.T) {
	s, source := newTestServer(t)
	root := filepath.Join(filepath.Dir(source), "app")
	addProject(t, s, root)
	original := detachSkillsTarget
	t.Cleanup(func() { detachSkillsTarget = original })
	detachSkillsTarget = func(path string, opts ssync.DetachOptions) (ssync.DetachReport, error) {
		t.Fatalf("save detached %s", path)
		return ssync.DetachReport{}, nil
	}
	body, err := json.Marshal(map[string]any{"root": root, "targets": []string{"claude"}, "skills": nil})
	if err != nil {
		t.Fatal(err)
	}
	rr := projectRequest(t, s, http.MethodPut, "/api/projects", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if s.cfg.Projects[root].Skills != nil || len(s.cfg.Projects[root].Targets) != 1 {
		t.Fatalf("save not applied: %+v", s.cfg.Projects[root])
	}
}
