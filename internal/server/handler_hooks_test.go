package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/oplog"
)

const hooksTestMutation = `{"name":"bash-log","entry":{"bindings":{"claude":{"events":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo skillshare-hooks-test"}]}]}}}}}`

func hooksTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home
}

func hooksPost(s *Server, handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return w
}

// hooksPreviewRevision previews the mutation and returns its revision.
func hooksPreviewRevision(t *testing.T, s *Server, mutation string) string {
	t.Helper()
	w := hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":`+mutation+`}`)
	var p struct {
		Revision string `json:"revision"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &p) != nil || p.Revision == "" {
		t.Fatalf("preview: %d %s", w.Code, w.Body)
	}
	return p.Revision
}

func TestHooksAPIPreviewThenApply(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	settings := filepath.Join(home, ".claude", "settings.json")

	revision := hooksPreviewRevision(t, s, hooksTestMutation)
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatal("preview wrote native configuration")
	}
	w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":`+hooksTestMutation+`,"revision":"`+revision+`","sync":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	data, err := os.ReadFile(settings)
	if err != nil || !strings.Contains(string(data), "skillshare-hooks-test") {
		t.Fatalf("apply did not write Claude settings: %v %s", err, data)
	}

	list := httptest.NewRecorder()
	s.handleHooksList(list, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	var view struct {
		Source struct {
			Entries map[string]json.RawMessage `json:"entries"`
		} `json:"source"`
	}
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &view) != nil || view.Source.Entries["bash-log"] == nil {
		t.Fatalf("list: %d %s", list.Code, list.Body)
	}
}

func TestHooksAPIProjectModeWritesProjectFiles(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	home := hooksTestHome(t)

	revision := hooksPreviewRevision(t, s, hooksTestMutation)
	w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":`+hooksTestMutation+`,"revision":"`+revision+`","sync":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", w.Code, w.Body)
	}
	if data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json")); err != nil || !strings.Contains(string(data), "skillshare-hooks-test") {
		t.Fatalf("project apply did not write project settings: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("project apply wrote global settings")
	}
}

func TestHooksAPIRejectsUnpreviewedOrStaleSaves(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)

	missing := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":`+hooksTestMutation+`,"sync":true}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("save without revision: %d %s", missing.Code, missing.Body)
	}
	stale := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":`+hooksTestMutation+`,"revision":"stale","sync":true}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revision: %d %s", stale.Code, stale.Body)
	}
}

func TestHooksAPIRejectsMalformedRequests(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"unknown field", s.handleHooksPreview, `{"mutation":{},"extra":true}`},
		{"two documents", s.handleHooksPreview, `{"mutation":{}}{"mutation":{}}`},
		{"import unknown field", s.handleHooksImport, `{"from":"claude","piExtension":"builtin"}`},
		{"restore without backup", s.handleHooksRestore, `{"preview":true}`},
		{"restore without preview", s.handleHooksRestore, `{"backupId":"x"}`},
	} {
		if w := hooksPost(s, tc.handler, "/", tc.body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", tc.name, w.Code, w.Body)
		}
	}
	large := hooksPost(s, s.handleHooksPreview, "/", `{"mutation":{"name":"`+strings.Repeat("x", int(defaultJSONBodyLimit))+`"}}`)
	if large.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: %d", large.Code)
	}
}

func TestHooksAPIImportListsCandidatesWithoutExecuting(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	marker := filepath.Join(t.TempDir(), "executed")
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	native := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"touch ` + marker + `"}]}]}}`
	if err := os.WriteFile(settings, []byte(native), 0644); err != nil {
		t.Fatal(err)
	}
	w := hooksPost(s, s.handleHooksImport, "/api/hooks/import", `{"from":"claude"}`)
	var result struct {
		Candidates []struct {
			Name string `json:"name"`
		} `json:"candidates"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Candidates) == 0 {
		t.Fatalf("import: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("import executed a hook command")
	}
}

func TestHooksRoutesRejectRebindingHost(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	s.addr = "127.0.0.1:19420"
	for _, route := range []string{"GET /api/hooks", "POST /api/hooks", "POST /api/hooks/preview", "POST /api/hooks/render", "POST /api/hooks/import", "POST /api/hooks/restore"} {
		method, path, _ := strings.Cut(route, " ")
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.RemoteAddr = "127.0.0.1:5555"
		req.Host = "attacker.example:19420"
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want 403", route, w.Code)
		}
	}
}

func TestHooksAPIRenderShowsOneHookWithoutWriting(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	config := readOrEmpty(s.configPath())
	root := t.TempDir()
	for _, tc := range []struct{ mutation, path string }{
		{hooksTestMutation, filepath.Join(home, ".claude", "settings.json")},
		{hooksProjectEntry(root, "guard", "echo skillshare-hooks-test"), filepath.Join(root, ".claude", "settings.json")},
	} {
		w := hooksPost(s, s.handleHooksRender, "/api/hooks/render", `{"mutation":`+tc.mutation+`}`)
		var body struct {
			Rendered []struct {
				Target, Path, Content, Error string
			} `json:"rendered"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Rendered) != 1 {
			t.Fatalf("render: %d %s", w.Code, w.Body)
		}
		f := body.Rendered[0]
		if f.Target != "claude" || f.Path != tc.path || f.Error != "" || !strings.Contains(f.Content, `"hooks"`) || !strings.Contains(f.Content, "skillshare-hooks-test") {
			t.Fatalf("rendered: %+v", f)
		}
		if _, err := os.Stat(tc.path); !os.IsNotExist(err) {
			t.Fatal("render wrote native configuration")
		}
	}
	if readOrEmpty(s.configPath()) != config {
		t.Fatal("render must not save the source")
	}
	if entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10); len(entries) != 0 {
		t.Fatalf("render must not write the operations log: %+v", entries)
	}
}

func TestHooksAPIRenderRejectsNonRenderRequests(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)
	for _, body := range []string{
		`{"mutation":` + hooksTestMutation + `,"revision":"x"}`,
		`{"mutation":{"name":"bash-log","remove":true}}`,
		`{"mutation":{"name":"bash-log"}}`,
		`{"mutation":{"entry":{"bindings":{}}}}`,
		`{"mutation":` + strings.Replace(hooksTestMutation, `"name"`, `"replace":true,"name"`, 1) + `}`,
		`{"mutation":` + hooksTestMutation + `}{}`,
	} {
		if w := hooksPost(s, s.handleHooksRender, "/api/hooks/render", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d %s", body, w.Code, w.Body)
		}
	}
}

// hooksSave previews and saves a mutation to the source without syncing.
func hooksSave(t *testing.T, s *Server, mutation string) {
	t.Helper()
	revision := hooksPreviewRevision(t, s, mutation)
	if w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":`+mutation+`,"revision":"`+revision+`"}`); w.Code != http.StatusOK {
		t.Fatalf("save %s: %d %s", mutation, w.Code, w.Body)
	}
}

func hooksProjectEntry(root, name, command string) string {
	return `{"project":"` + root + `","name":"` + name + `","entry":{"bindings":{"claude":{"events":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + command + `"}]}]}}}}}`
}

func readOrEmpty(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func TestHooksAPIProjectSyncAppliesOnlyThatRoot(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	rootA, rootB := t.TempDir(), t.TempDir()
	settingsA := filepath.Join(rootA, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsA), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsA, []byte(`{"theme":"dark"}`), 0644); err != nil {
		t.Fatal(err)
	}
	hooksSave(t, s, hooksProjectEntry(rootA, "a", "echo hook-a"))
	hooksSave(t, s, hooksProjectEntry(rootB, "b", "echo hook-b"))
	hooksSave(t, s, hooksTestMutation)

	revision := hooksPreviewRevision(t, s, `{}`)
	w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":{},"revision":"`+revision+`","sync":true,"root":"`+rootA+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("project sync: %d %s", w.Code, w.Body)
	}
	if got := readOrEmpty(settingsA); !strings.Contains(got, "echo hook-a") || !strings.Contains(got, `"theme"`) {
		t.Fatalf("root A not synced with its settings kept:\n%s", got)
	}
	if got := readOrEmpty(filepath.Join(rootB, ".claude", "settings.json")); got != "" {
		t.Fatalf("syncing root A wrote root B:\n%s", got)
	}
	if got := readOrEmpty(filepath.Join(home, ".claude", "settings.json")); got != "" {
		t.Fatalf("syncing root A wrote the global settings:\n%s", got)
	}

	stale := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":{},"revision":"`+revision+`","sync":true,"root":"`+rootB+`"}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("reused revision: %d %s", stale.Code, stale.Body)
	}

	hooksSave(t, s, `{"project":"`+rootA+`","name":"a","remove":true}`)
	revision = hooksPreviewRevision(t, s, `{}`)
	if w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":{},"revision":"`+revision+`","sync":true,"root":"`+rootA+`"}`); w.Code != http.StatusOK {
		t.Fatalf("prune sync: %d %s", w.Code, w.Body)
	}
	if got := readOrEmpty(settingsA); strings.Contains(got, "echo hook-a") || !strings.Contains(got, `"theme"`) {
		t.Fatalf("prune did not remove only the owned hook:\n%s", got)
	}
	if got := readOrEmpty(filepath.Join(rootB, ".claude", "settings.json")); got != "" {
		t.Fatalf("pruning root A wrote root B:\n%s", got)
	}
}

func TestHooksAPIProjectSyncRejectsInvalidRoot(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)
	root := t.TempDir()
	hooksSave(t, s, hooksProjectEntry(root, "a", "echo hook-a"))
	revision := hooksPreviewRevision(t, s, `{}`)
	for name, body := range map[string]string{
		"with changes": `{"mutation":{"name":"a","remove":true},"revision":"` + revision + `","sync":true,"root":"` + root + `"}`,
		"without sync": `{"mutation":{},"revision":"` + revision + `","root":"` + root + `"}`,
		"unknown root": `{"mutation":{},"revision":"` + revision + `","sync":true,"root":"` + t.TempDir() + `"}`,
	} {
		if w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}

	project, _ := newTestProjectServerWithExtras(t, nil)
	revision = hooksPreviewRevision(t, project, `{}`)
	w := hooksPost(project, project.handleHooksConfigure, "/api/hooks", `{"mutation":{},"revision":"`+revision+`","sync":true,"root":"`+root+`"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "global mode") {
		t.Fatalf("project mode root: %d %s", w.Code, w.Body)
	}
}

func TestHooksAPIImportReadsProjectRoot(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)
	root := t.TempDir()
	hooksSave(t, s, `{"project":"`+root+`"}`)
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo project-native"}]}]}}`), 0644); err != nil {
		t.Fatal(err)
	}
	w := hooksPost(s, s.handleHooksImport, "/api/hooks/import", `{"from":"claude","root":"`+root+`"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "project-native") {
		t.Fatalf("project import: %d %s", w.Code, w.Body)
	}
	global := hooksPost(s, s.handleHooksImport, "/api/hooks/import", `{"from":"claude"}`)
	if global.Code != http.StatusOK || strings.Contains(global.Body.String(), "project-native") {
		t.Fatalf("global import read the project file: %d %s", global.Code, global.Body)
	}
	if w := hooksPost(s, s.handleHooksImport, "/api/hooks/import", `{"from":"claude","root":"`+t.TempDir()+`"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown import root: %d %s", w.Code, w.Body)
	}
}
