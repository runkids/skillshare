package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"strings"
	"testing"
)

// These tests replay the requests the dashboard sends (ui/src/api/hooks.ts and its callers)
// against the real handlers: the same endpoints, the same mutation JSON, and the revision
// taken from the preview the dialog used. UI tests mock one fixed revision, so a mismatch
// between what the dashboard previews and what it saves only shows up here.

type hooksUIView struct {
	Source struct {
		Entries  map[string]json.RawMessage `json:"entries"`
		Projects map[string]struct {
			Entries map[string]json.RawMessage `json:"entries"`
		} `json:"projects"`
	} `json:"source"`
	Plan *struct {
		Blocked bool `json:"blocked"`
		Changes []struct {
			Name   string `json:"name"`
			Root   string `json:"root"`
			Action string `json:"action"`
		} `json:"changes"`
	} `json:"plan"`
	Backups []struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	} `json:"backups"`
	Unmanaged []struct {
		Path  string   `json:"path"`
		Names []string `json:"names"`
	} `json:"unmanaged"`
}

func hooksUIList(t *testing.T, s *Server) hooksUIView {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleHooksList(w, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	var view hooksUIView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &view) != nil {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	return view
}

// hooksUIConfigure is hooksApi.configure.
func hooksUIConfigure(s *Server, mutation, revision string, sync bool) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"mutation": json.RawMessage(mutation), "revision": revision, "sync": sync})
	return hooksPost(s, s.handleHooksConfigure, "/api/hooks", string(body))
}

// hooksUISync is HooksSyncDialog without a project, and the Sync page: preview nothing, apply that plan.
func hooksUISync(t *testing.T, s *Server) {
	t.Helper()
	if w := hooksUIConfigure(s, `{}`, hooksPreviewRevision(t, s, `{}`), true); w.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", w.Code, w.Body)
	}
}

func hooksEntryJSON(command string) string {
	return `{"bindings":{"claude":{"events":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + command + `"}]}]}}}}`
}

// hooksUIScope is one place the dashboard manages hooks from: the global page, a
// hooks.projects root's tab, or a server started in project mode.
type hooksUIScope struct {
	name     string
	s        *Server
	project  string // mutation.project, empty outside a hooks.projects root
	settings string
}

var hooksUIScopes = []func(t *testing.T) hooksUIScope{
	func(t *testing.T) hooksUIScope {
		s, _ := newTestServerWithExtras(t, nil, "")
		home := hooksTestHome(t)
		return hooksUIScope{"global", s, "", filepath.Join(home, ".claude", "settings.json")}
	},
	func(t *testing.T) hooksUIScope {
		s, _ := newTestServerWithExtras(t, nil, "")
		hooksTestHome(t)
		root := t.TempDir()
		return hooksUIScope{"hooks.projects root", s, root, filepath.Join(root, ".claude", "settings.json")}
	},
	func(t *testing.T) hooksUIScope {
		s, root := newTestProjectServerWithExtras(t, nil)
		hooksTestHome(t)
		return hooksUIScope{"project mode", s, "", filepath.Join(root, ".claude", "settings.json")}
	},
}

var hooksUIScopeNames = []string{"global", "hooks.projects root", "project mode"}

// mutation adds the scope's project field the way the dialogs spread `...(project && { project })`.
func (sc hooksUIScope) mutation(fields string) string {
	if sc.project == "" {
		return `{` + fields + `}`
	}
	return `{"project":"` + sc.project + `",` + fields + `}`
}

// addAndSync is HookDialog with "Save and sync": preview the mutation, configure with that revision.
func (sc hooksUIScope) addAndSync(t *testing.T, name, command string) {
	t.Helper()
	mutation := sc.mutation(`"name":"` + name + `","entry":` + hooksEntryJSON(command))
	if w := hooksUIConfigure(sc.s, mutation, hooksPreviewRevision(t, sc.s, mutation), true); w.Code != http.StatusOK {
		t.Fatalf("%s: add and sync: %d %s", sc.name, w.Code, w.Body)
	}
	if !strings.Contains(readOrEmpty(sc.settings), command) {
		t.Fatalf("%s: not written:\n%s", sc.name, readOrEmpty(sc.settings))
	}
}

func (sc hooksUIScope) entries(t *testing.T) map[string]json.RawMessage {
	view := hooksUIList(t, sc.s)
	if sc.project != "" {
		return view.Source.Projects[sc.project].Entries
	}
	return view.Source.Entries
}

// HooksRemoveDialog: all three choices, in every scope.
func TestHooksUIContract_RemoveDialog(t *testing.T) {
	for i, newScope := range hooksUIScopes {
		for _, choice := range []string{"sync", "source only", "stop managing"} {
			t.Run(hooksUIScopeNames[i]+"/"+choice, func(t *testing.T) {
				sc := newScope(t)
				sc.addAndSync(t, "guard", "echo contract-guard")
				removal := sc.mutation(`"name":"guard","remove":true`)
				planRevision := hooksPreviewRevision(t, sc.s, removal)
				var w *httptest.ResponseRecorder
				switch choice {
				case "sync":
					w = hooksUIConfigure(sc.s, removal, planRevision, true)
				case "source only":
					w = hooksUIConfigure(sc.s, removal, planRevision, false)
				case "stop managing":
					unmanage := sc.mutation(`"name":"guard","remove":true,"unmanage":true`)
					w = hooksUIConfigure(sc.s, unmanage, hooksPreviewRevision(t, sc.s, unmanage), false)
				}
				if w.Code != http.StatusOK {
					t.Fatalf("%d %s", w.Code, w.Body)
				}
				if _, ok := sc.entries(t)["guard"]; ok {
					t.Fatal("still in the source")
				}
				written := strings.Contains(readOrEmpty(sc.settings), "echo contract-guard")
				view := hooksUIList(t, sc.s)
				pending := view.Plan != nil && len(view.Plan.Changes) > 0
				switch choice {
				case "sync":
					if written || pending {
						t.Fatalf("written=%v pending=%v", written, pending)
					}
				case "source only":
					if !written || !pending || view.Plan.Changes[0].Action != "remove" {
						t.Fatalf("written=%v plan=%+v", written, view.Plan)
					}
				case "stop managing":
					if !written || pending || len(view.Unmanaged) != 1 || view.Unmanaged[0].Path != sc.settings {
						t.Fatalf("written=%v plan=%+v unmanaged=%+v", written, view.Plan, view.Unmanaged)
					}
				}
			})
		}
	}
}

// Stop managing plans no file change, so its revision differs from the removal preview's.
// A dashboard that reuses the removal preview's revision is refused as stale (fixed in the UI
// by previewing the unmanage mutation itself); this pins the server side of that contract.
func TestHooksUIContract_StopManagingRejectsTheRemovalPreviewRevision(t *testing.T) {
	sc := hooksUIScopes[0](t)
	sc.addAndSync(t, "guard", "echo contract-guard")
	removalRevision := hooksPreviewRevision(t, sc.s, `{"name":"guard","remove":true}`)
	w := hooksUIConfigure(sc.s, `{"name":"guard","remove":true,"unmanage":true}`, removalRevision, false)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "changed since preview") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if _, ok := sc.entries(t)["guard"]; !ok {
		t.Fatal("a refused save changed the source")
	}
}

// HookDialog edit: same name, new entry, no replace flag; then save-only edits and HooksScope's
// enable switch, which saves the entry with enabled:false.
func TestHooksUIContract_EditAndToggle(t *testing.T) {
	for i, newScope := range hooksUIScopes {
		t.Run(hooksUIScopeNames[i], func(t *testing.T) {
			sc := newScope(t)
			sc.addAndSync(t, "guard", "echo contract-v1")
			sc.addAndSync(t, "guard", "echo contract-v2")
			if got := readOrEmpty(sc.settings); strings.Contains(got, "contract-v1") {
				t.Fatalf("edit kept the old command:\n%s", got)
			}

			hooksSave(t, sc.s, sc.mutation(`"name":"guard","entry":`+strings.Replace(hooksEntryJSON("echo contract-v2"), `{"bindings"`, `{"enabled":false,"bindings"`, 1)))
			hooksUISync(t, sc.s)
			if strings.Contains(readOrEmpty(sc.settings), "contract-v2") {
				t.Fatal("a disabled hook stayed in the target file")
			}
			hooksSave(t, sc.s, sc.mutation(`"name":"guard","entry":`+hooksEntryJSON("echo contract-v2")))
			hooksUISync(t, sc.s)
			if !strings.Contains(readOrEmpty(sc.settings), "contract-v2") {
				t.Fatal("enabling did not write the hook back")
			}
		})
	}
}

// HooksSyncDialog with a takeover (and HookDialog with its takeover box): the source entry
// with replace:true, previewed and applied with sync, overwrites an outside edit.
func TestHooksUIContract_TakeOverOutsideEdit(t *testing.T) {
	for i, newScope := range hooksUIScopes {
		t.Run(hooksUIScopeNames[i], func(t *testing.T) {
			sc := newScope(t)
			sc.addAndSync(t, "guard", "echo contract-guard")
			edited := strings.Replace(readOrEmpty(sc.settings), "echo contract-guard", "echo edited-outside", 1)
			if err := os.WriteFile(sc.settings, []byte(edited), 0644); err != nil {
				t.Fatal(err)
			}
			if view := hooksUIList(t, sc.s); view.Plan == nil || !view.Plan.Blocked {
				t.Fatalf("outside edit is not a conflict: %+v", view.Plan)
			}
			takeover := sc.mutation(`"name":"guard","entry":` + string(sc.entries(t)["guard"]) + `,"replace":true`)
			if w := hooksUIConfigure(sc.s, takeover, hooksPreviewRevision(t, sc.s, takeover), true); w.Code != http.StatusOK {
				t.Fatalf("takeover: %d %s", w.Code, w.Body)
			}
			if got := readOrEmpty(sc.settings); strings.Contains(got, "edited-outside") || !strings.Contains(got, "contract-guard") {
				t.Fatalf("takeover did not restore the source hook:\n%s", got)
			}
			if view := hooksUIList(t, sc.s); view.Plan == nil || view.Plan.Blocked {
				t.Fatalf("still blocked after takeover: %+v", view.Plan)
			}
		})
	}
}

// HooksImportDialog in a hooks.projects root: import with root, save with adopt, then the
// project's sync adopts the registration in place.
func TestHooksUIContract_ProjectImportAdopt(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	hooksTestHome(t)
	root := t.TempDir()
	hooksSave(t, s, `{"project":"`+root+`"}`) // AddProjectDialog
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	native := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo project-native"}]}]}}`
	if err := os.WriteFile(settings, []byte(native), 0644); err != nil {
		t.Fatal(err)
	}
	w := hooksPost(s, s.handleHooksImport, "/api/hooks/import", `{"from":"claude","root":"`+root+`"}`)
	var imported struct {
		Candidates []struct {
			Name  string          `json:"name"`
			Entry json.RawMessage `json:"entry"`
		} `json:"candidates"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &imported) != nil || len(imported.Candidates) != 1 {
		t.Fatalf("import: %d %s", w.Code, w.Body)
	}
	hooksSave(t, s, `{"project":"`+root+`","name":"stop","entry":`+string(imported.Candidates[0].Entry)+`,"adopt":true}`)
	if w := hooksPost(s, s.handleHooksConfigure, "/api/hooks", `{"mutation":{},"revision":"`+hooksPreviewRevision(t, s, `{}`)+`","sync":true,"root":"`+root+`"}`); w.Code != http.StatusOK {
		t.Fatalf("project sync: %d %s", w.Code, w.Body)
	}
	view := hooksUIList(t, s)
	if len(view.Unmanaged) != 0 || view.Plan == nil || view.Plan.Blocked || view.Plan.Changes[0].Action != "unchanged" {
		t.Fatalf("adopted hook is not managed: unmanaged=%+v plan=%+v", view.Unmanaged, view.Plan)
	}
	if readOrEmpty(settings) != native {
		t.Fatalf("adopting rewrote the file:\n%s", readOrEmpty(settings))
	}

	hooksSave(t, s, `{"project":"`+root+`","remove":true}`) // ProjectDetailPage removing the project
	if _, ok := hooksUIList(t, s).Source.Projects[root]; ok {
		t.Fatal("project still configured")
	}
}

// The global sync writes the whole plan, hooks.projects roots included.
func TestHooksUIContract_GlobalSyncWritesEveryScope(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := hooksTestHome(t)
	root := t.TempDir()
	hooksSave(t, s, hooksTestMutation)
	hooksSave(t, s, hooksProjectEntry(root, "a", "echo hook-a"))
	hooksUISync(t, s)
	if !strings.Contains(readOrEmpty(filepath.Join(home, ".claude", "settings.json")), "skillshare-hooks-test") || !strings.Contains(readOrEmpty(filepath.Join(root, ".claude", "settings.json")), "echo hook-a") {
		t.Fatal("global sync skipped a scope")
	}
}

// HooksRestoreDialog: preview the backup, restore with that revision. A shared settings file
// gets back only what that write changed; the rest of the file stays.
func TestHooksUIContract_RestoreBackup(t *testing.T) {
	sc := hooksUIScopes[0](t)
	if err := os.MkdirAll(filepath.Dir(sc.settings), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sc.settings, []byte(`{"model":"opus"}`), 0644); err != nil {
		t.Fatal(err)
	}
	sc.addAndSync(t, "guard", "echo contract-guard")
	backups := hooksUIList(t, sc.s).Backups
	if len(backups) != 1 {
		t.Fatalf("backups: %+v", backups)
	}
	preview := hooksPost(sc.s, sc.s.handleHooksRestore, "/api/hooks/restore", `{"backupId":"`+backups[0].ID+`","preview":true}`)
	var p struct {
		Revision string `json:"revision"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &p) != nil || p.Revision == "" {
		t.Fatalf("restore preview: %d %s", preview.Code, preview.Body)
	}
	if w := hooksPost(sc.s, sc.s.handleHooksRestore, "/api/hooks/restore", `{"backupId":"`+backups[0].ID+`","revision":"`+p.Revision+`"}`); w.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", w.Code, w.Body)
	}
	if got := readOrEmpty(sc.settings); strings.Contains(got, "contract-guard") || !strings.Contains(got, `"model":"opus"`) {
		t.Fatalf("restored file:\n%s", got)
	}
}

// HookDialog with its takeover box ticked but saved without sync: replace is not kept in the
// source, so the conflict stays for the next takeover preview to resolve.
func TestHooksUIContract_TakeOverSaveOnlyKeepsTheConflict(t *testing.T) {
	sc := hooksUIScopes[0](t)
	sc.addAndSync(t, "guard", "echo contract-guard")
	edited := strings.Replace(readOrEmpty(sc.settings), "echo contract-guard", "echo edited-outside", 1)
	if err := os.WriteFile(sc.settings, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	hooksSave(t, sc.s, `{"name":"guard","entry":`+hooksEntryJSON("echo contract-v2")+`,"replace":true}`)
	if view := hooksUIList(t, sc.s); view.Plan == nil || !view.Plan.Blocked {
		t.Fatalf("save-only takeover resolved the conflict: %+v", view.Plan)
	}
	if readOrEmpty(sc.settings) != edited {
		t.Fatal("save-only takeover wrote the target file")
	}
}

func TestHooksUIContract_AccountTargets(t *testing.T) {
	s, _ := newTestServer(t)
	home := hooksTestHome(t)
	dir := filepath.Join(home, ".codex-2")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	s.cfg.Targets["codex-2"] = config.TargetConfig{Agent: "codex", ConfigDir: dir}
	w := httptest.NewRecorder()
	s.handleHooksList(w, httptest.NewRequest(http.MethodGet, "/api/hooks", nil))
	var inv hooks.Inventory
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &inv) != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	found := false
	for _, target := range inv.Targets {
		if target.Name == "codex-2" && target.Agent == "codex" && target.Kind == "command" {
			found = true
		}
	}
	if !found || inv.Paths["codex-2"] != filepath.Join(dir, "hooks.json") {
		t.Fatalf("%+v", inv)
	}
	mutation := `{"name":"k","entry":{"bindings":{"codex-2":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}}}}`
	w = hooksPost(s, s.handleHooksPreview, "/api/hooks/preview", `{"mutation":`+mutation+`}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"target":"codex-2"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
