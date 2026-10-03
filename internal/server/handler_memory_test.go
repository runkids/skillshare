package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/memory"
)

func TestMemoryAPI_ScopesAndConflict(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			var s *Server
			var root string
			if project {
				s, root = newTestProjectServerWithExtras(t, nil)
				root = filepath.Join(root, ".skillshare/extras/memory")
			} else {
				s, _ = newTestServerWithExtras(t, nil, "")
				root, _ = memory.GlobalRoot(s.cfg)
			}
			request := func(method, path, body string) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				s.mux.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
				return rr
			}
			rr := request("POST", "/api/extras/memory/init", "")
			if rr.Code != http.StatusOK {
				t.Fatalf("init: %d %s", rr.Code, rr.Body)
			}
			for _, path := range []string{"INDEX.md", "LEARNED.md"} {
				if _, err := os.Stat(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			}
			body := `{"path":"build.md","content":"# Build\nUse the devcontainer.","version":""}`
			rr = request("PUT", "/api/extras/memory/notes/content", body)
			if rr.Code != http.StatusOK {
				t.Fatalf("write: %d %s", rr.Code, rr.Body)
			}
			var note memory.Note
			json.Unmarshal(rr.Body.Bytes(), &note)
			os.WriteFile(filepath.Join(root, "build.md"), []byte("# External"), 0644)
			encoded, _ := json.Marshal(map[string]string{"path": "build.md", "content": "# Lost", "version": note.Version})
			rr = request("PUT", "/api/extras/memory/notes/content", string(encoded))
			if rr.Code != http.StatusConflict {
				t.Fatalf("stale edit: %d %s", rr.Code, rr.Body)
			}
			rr = request("GET", "/api/extras/memory/notes?search=External", "")
			if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "build.md") {
				t.Fatalf("search: %d %s", rr.Code, rr.Body)
			}
			rr = request("GET", "/api/extras/memory/notes/content?path=../outside.md", "")
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("traversal: %d %s", rr.Code, rr.Body)
			}
			encoded, _ = json.Marshal(map[string]string{"path": "build.md", "version": note.Version})
			rr = request("DELETE", "/api/extras/memory/notes/content", string(encoded))
			if rr.Code != http.StatusConflict {
				t.Fatalf("stale delete: %d %s", rr.Code, rr.Body)
			}
			current, err := memory.Read(root, "build.md")
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ = json.Marshal(map[string]string{"path": "build.md", "version": current.Version})
			rr = request("DELETE", "/api/extras/memory/notes/content", string(encoded))
			if rr.Code != http.StatusOK {
				t.Fatalf("delete: %d %s", rr.Code, rr.Body)
			}
			rr = request("GET", "/api/extras/memory/notes/content?path=build.md", "")
			if rr.Code != http.StatusNotFound {
				t.Fatalf("deleted note: %d %s", rr.Code, rr.Body)
			}
		})
	}
}

func TestMemoryAPI_UsesExtraSourceAndPreservesIndex(t *testing.T) {
	root := t.TempDir()
	s, _ := newTestServerWithExtras(t, []config.ExtraConfig{{Name: "memory", Source: root}}, "")
	os.WriteFile(filepath.Join(root, "INDEX.md"), []byte("# Personal index"), 0644)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest("POST", "/api/extras/memory/init", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("init: %d %s", rr.Code, rr.Body)
	}
	data, _ := os.ReadFile(filepath.Join(root, "INDEX.md"))
	if string(data) != "# Personal index" {
		t.Fatal("index overwritten")
	}
}
