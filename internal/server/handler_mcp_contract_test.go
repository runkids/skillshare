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

// The MCP half of the dashboard contract: each test replays what ui/src/api/mcp.ts and its
// callers send, with the revision taken from the preview the dialog used.

type mcpUIView struct {
	Source struct {
		Servers  map[string]json.RawMessage `json:"servers"`
		Projects map[string]struct {
			Servers map[string]json.RawMessage `json:"servers"`
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
		ID string `json:"id"`
	} `json:"backups"`
	Unmanaged []struct {
		Path  string   `json:"path"`
		Names []string `json:"names"`
	} `json:"unmanaged"`
}

func mcpPost(handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return w
}

func mcpUIList(t *testing.T, s *Server) mcpUIView {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleMCPList(w, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
	var view mcpUIView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &view) != nil {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	return view
}

func mcpUIPreviewRevision(t *testing.T, s *Server, mutation string) string {
	t.Helper()
	w := mcpPost(s.handleMCPPreview, "/api/mcp/preview", `{"mutation":`+mutation+`}`)
	var p struct {
		Revision string `json:"revision"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &p) != nil || p.Revision == "" {
		t.Fatalf("preview %s: %d %s", mutation, w.Code, w.Body)
	}
	return p.Revision
}

// mcpUIConfigure is mcpApi.configure.
func mcpUIConfigure(s *Server, mutation, revision string, sync bool) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"mutation": json.RawMessage(mutation), "revision": revision, "sync": sync})
	return mcpPost(s.handleMCPConfigure, "/api/mcp", string(body))
}

// mcpUISave is mcpApi.save: preview the mutation, save the source with that revision.
func mcpUISave(t *testing.T, s *Server, mutation string) {
	t.Helper()
	if w := mcpUIConfigure(s, mutation, mcpUIPreviewRevision(t, s, mutation), false); w.Code != http.StatusOK {
		t.Fatalf("save %s: %d %s", mutation, w.Code, w.Body)
	}
}

// mcpUISync is the Sync page and MCPSyncDialog (runSync): the global plan, or one project's.
func mcpUISync(t *testing.T, s *Server, root string) {
	t.Helper()
	revision := mcpUIPreviewRevision(t, s, `{}`)
	var w *httptest.ResponseRecorder
	if root == "" {
		w = mcpUIConfigure(s, `{}`, revision, true)
	} else {
		w = mcpPost(s.handleMCPConfigure, "/api/mcp", `{"mutation":{},"revision":"`+revision+`","sync":true,"root":"`+root+`"}`)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("sync %q: %d %s", root, w.Code, w.Body)
	}
}

const mcpUIServer = `{"url":"https://example.com/contract","targets":["claude"]}`

// mcpUIScope is one place the dashboard manages MCP from.
type mcpUIScope struct {
	s       *Server
	project string // mutation.project, empty outside an mcp.projects root
	root    string // the sync root the dashboard passes for this scope
	file    string // Claude's MCP file here
}

var mcpUIScopeNames = []string{"global", "mcp.projects root", "project mode"}

var mcpUIScopes = []func(t *testing.T) mcpUIScope{
	func(t *testing.T) mcpUIScope {
		s, _ := newTestServerWithExtras(t, nil, "")
		home := hooksTestHome(t)
		return mcpUIScope{s, "", "", filepath.Join(home, ".claude.json")}
	},
	func(t *testing.T) mcpUIScope {
		s, _ := newTestServerWithExtras(t, nil, "")
		hooksTestHome(t)
		root := t.TempDir()
		mcpUISave(t, s, `{"project":"`+root+`","settings":{"targets":["claude"]}}`) // AddProjectDialog
		return mcpUIScope{s, root, root, filepath.Join(root, ".mcp.json")}
	},
	func(t *testing.T) mcpUIScope {
		s, root := newTestProjectServerWithExtras(t, nil)
		hooksTestHome(t)
		return mcpUIScope{s, "", "", filepath.Join(root, ".mcp.json")}
	},
}

func (sc mcpUIScope) mutation(fields string) string {
	if sc.project == "" {
		return `{` + fields + `}`
	}
	return `{"project":"` + sc.project + `",` + fields + `}`
}

func (sc mcpUIScope) servers(t *testing.T) map[string]json.RawMessage {
	view := mcpUIList(t, sc.s)
	if sc.project != "" {
		return view.Source.Projects[sc.project].Servers
	}
	return view.Source.Servers
}

// addAndSync is MCPServerDialog's save followed by the scope's sync.
func (sc mcpUIScope) addAndSync(t *testing.T) {
	t.Helper()
	mcpUISave(t, sc.s, sc.mutation(`"name":"docs","server":`+mcpUIServer+`,"replace":false`))
	mcpUISync(t, sc.s, sc.root)
	if !strings.Contains(readOrEmpty(sc.file), "example.com/contract") {
		t.Fatalf("not written:\n%s", readOrEmpty(sc.file))
	}
}

func (sc mcpUIScope) pending(t *testing.T) []string {
	var out []string
	if p := mcpUIList(t, sc.s).Plan; p != nil {
		for _, c := range p.Changes {
			if c.Action != "unchanged" {
				out = append(out, c.Name+":"+c.Action)
			}
		}
	}
	return out
}

// MCPRemoveDialog: all three choices, in every scope.
func TestMCPUIContract_RemoveDialog(t *testing.T) {
	for i, newScope := range mcpUIScopes {
		for _, choice := range []string{"sync", "source only", "stop managing"} {
			t.Run(mcpUIScopeNames[i]+"/"+choice, func(t *testing.T) {
				sc := newScope(t)
				sc.addAndSync(t)
				removal := sc.mutation(`"name":"docs","remove":true`)
				planRevision := mcpUIPreviewRevision(t, sc.s, removal)
				var w *httptest.ResponseRecorder
				switch choice {
				case "sync":
					w = mcpUIConfigure(sc.s, removal, planRevision, true)
				case "source only":
					w = mcpUIConfigure(sc.s, removal, planRevision, false)
				case "stop managing":
					unmanage := sc.mutation(`"name":"docs","remove":true,"unmanage":true`)
					w = mcpUIConfigure(sc.s, unmanage, mcpUIPreviewRevision(t, sc.s, unmanage), false)
				}
				if w.Code != http.StatusOK {
					t.Fatalf("%d %s", w.Code, w.Body)
				}
				if _, ok := sc.servers(t)["docs"]; ok {
					t.Fatal("still in the source")
				}
				written := strings.Contains(readOrEmpty(sc.file), "example.com/contract")
				pending := sc.pending(t)
				switch choice {
				case "sync":
					if written || len(pending) != 0 {
						t.Fatalf("written=%v pending=%v", written, pending)
					}
				case "source only":
					if !written || len(pending) != 1 || pending[0] != "docs:remove" {
						t.Fatalf("written=%v pending=%v", written, pending)
					}
				case "stop managing":
					unmanaged := mcpUIList(t, sc.s).Unmanaged
					if !written || len(pending) != 0 || len(unmanaged) != 1 || unmanaged[0].Path != sc.file {
						t.Fatalf("written=%v pending=%v unmanaged=%+v", written, pending, unmanaged)
					}
				}
			})
		}
	}
}

// MCPPage's conflict "replace": the resolution alone, previewed and applied with sync.
func TestMCPUIContract_ReplaceOutsideEdit(t *testing.T) {
	for i, newScope := range mcpUIScopes {
		t.Run(mcpUIScopeNames[i], func(t *testing.T) {
			sc := newScope(t)
			sc.addAndSync(t)
			edited := strings.Replace(readOrEmpty(sc.file), "example.com/contract", "example.com/edited", 1)
			if err := os.WriteFile(sc.file, []byte(edited), 0644); err != nil {
				t.Fatal(err)
			}
			if p := mcpUIList(t, sc.s).Plan; p == nil || !p.Blocked {
				t.Fatalf("outside edit is not a conflict: %+v", p)
			}
			replace := `{"resolutions":[{"target":"claude","name":"docs","action":"replace"}]}`
			if w := mcpUIConfigure(sc.s, replace, mcpUIPreviewRevision(t, sc.s, replace), true); w.Code != http.StatusOK {
				t.Fatalf("replace: %d %s", w.Code, w.Body)
			}
			if got := readOrEmpty(sc.file); strings.Contains(got, "example.com/edited") || !strings.Contains(got, "example.com/contract") {
				t.Fatalf("replace did not restore the source server:\n%s", got)
			}
			if p := mcpUIList(t, sc.s).Plan; p == nil || p.Blocked {
				t.Fatalf("still blocked: %+v", p)
			}
		})
	}
}

// MCPImportDialog: import an Agent's own entry and save it with an adopt resolution; the
// next sync treats it as managed instead of a conflict.
func TestMCPUIContract_ImportAdopt(t *testing.T) {
	for i, newScope := range mcpUIScopes {
		t.Run(mcpUIScopeNames[i], func(t *testing.T) {
			sc := newScope(t)
			if err := os.WriteFile(sc.file, []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/native"}}}`), 0644); err != nil {
				t.Fatal(err)
			}
			request := `{"from":"claude"}`
			if sc.project != "" {
				request = `{"from":"claude","root":"` + sc.project + `"}`
			}
			w := mcpPost(sc.s.handleMCPImport, "/api/mcp/import", request)
			var imported struct {
				Candidates []struct {
					Name   string                     `json:"name"`
					Server map[string]json.RawMessage `json:"server"`
				} `json:"candidates"`
			}
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &imported) != nil || len(imported.Candidates) != 1 {
				t.Fatalf("import: %d %s", w.Code, w.Body)
			}
			server := imported.Candidates[0].Server
			server["targets"] = json.RawMessage(`["claude"]`)
			serverJSON, _ := json.Marshal(server)
			mcpUISave(t, sc.s, sc.mutation(`"name":"docs","server":`+string(serverJSON)+`,"replace":false,"resolutions":[{"target":"claude","name":"docs","action":"adopt"}]`))
			if p := mcpUIList(t, sc.s).Plan; p == nil || p.Blocked {
				t.Fatalf("adopted server is a conflict: %+v", p)
			}
			// The next sync may only re-lay out the adopted entry, never change or refuse it.
			mcpUISync(t, sc.s, sc.root)
			if got := readOrEmpty(sc.file); !strings.Contains(got, "example.com/native") {
				t.Fatalf("sync changed the adopted server:\n%s", got)
			}
		})
	}
}

// useMCPToggle and MCPProjectView's switches save the whole server with replace:true; a
// project turning a global server off saves a disabled entry, and on again removes it.
func TestMCPUIContract_Toggles(t *testing.T) {
	sc := mcpUIScopes[0](t)
	sc.addAndSync(t)
	mcpUISave(t, sc.s, `{"name":"docs","server":{"url":"https://example.com/contract","targets":[]},"replace":true}`)
	mcpUISync(t, sc.s, "")
	if strings.Contains(readOrEmpty(sc.file), "example.com/contract") {
		t.Fatal("unticked Agent kept the server")
	}
	mcpUISave(t, sc.s, `{"name":"docs","server":`+mcpUIServer+`,"replace":true}`)
	mcpUISync(t, sc.s, "")

	root := t.TempDir()
	mcpUISave(t, sc.s, `{"project":"`+root+`","settings":{"targets":["claude"]}}`)
	mcpUISave(t, sc.s, `{"project":"`+root+`","name":"docs","replace":true,"server":{"disabled":true}}`)
	mcpUISync(t, sc.s, root)
	if pending := sc.pending(t); len(pending) != 0 {
		t.Fatalf("turning off in a project left changes: %v", pending)
	}
	mcpUISave(t, sc.s, `{"project":"`+root+`","name":"docs","remove":true}`)
	mcpUISync(t, sc.s, root)
	if pending := sc.pending(t); len(pending) != 0 {
		t.Fatalf("turning back on left changes: %v", pending)
	}

	mcpUISave(t, sc.s, `{"project":"`+root+`","remove":true}`) // MCPProjectView and ProjectDetailPage dropping the project
	if _, ok := mcpUIList(t, sc.s).Source.Projects[root]; ok {
		t.Fatal("project still configured")
	}
}

// MCPPage's default targets: a server without its own targets follows them.
func TestMCPUIContract_DefaultTargets(t *testing.T) {
	sc := mcpUIScopes[0](t)
	mcpUISave(t, sc.s, `{"settings":{"targets":["claude"]},"replace":true}`)
	mcpUISave(t, sc.s, `{"name":"docs","server":{"url":"https://example.com/contract"},"replace":false}`)
	mcpUISync(t, sc.s, "")
	if !strings.Contains(readOrEmpty(sc.file), "example.com/contract") {
		t.Fatal("default target was not written")
	}
}

// MCPRestoreDialog: preview the backup, restore with that revision.
func TestMCPUIContract_RestoreBackup(t *testing.T) {
	sc := mcpUIScopes[0](t)
	if err := os.WriteFile(sc.file, []byte(`{"theme":"dark"}`), 0644); err != nil {
		t.Fatal(err)
	}
	sc.addAndSync(t)
	backups := mcpUIList(t, sc.s).Backups
	if len(backups) != 1 {
		t.Fatalf("backups: %+v", backups)
	}
	preview := mcpPost(sc.s.handleMCPRestore, "/api/mcp/restore", `{"backupId":"`+backups[0].ID+`","preview":true}`)
	var p struct {
		Revision string `json:"revision"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &p) != nil || p.Revision == "" {
		t.Fatalf("restore preview: %d %s", preview.Code, preview.Body)
	}
	if w := mcpPost(sc.s.handleMCPRestore, "/api/mcp/restore", `{"backupId":"`+backups[0].ID+`","revision":"`+p.Revision+`"}`); w.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", w.Code, w.Body)
	}
	if got := readOrEmpty(sc.file); strings.Contains(got, "example.com/contract") || !strings.Contains(got, `"theme"`) {
		t.Fatalf("restored file:\n%s", got)
	}
}
