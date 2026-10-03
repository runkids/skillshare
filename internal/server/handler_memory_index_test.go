package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillshare/internal/memory"
)

func TestMemoryIndexAPI_Scopes(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			var s *Server
			if project {
				s, _ = newTestProjectServerWithExtras(t, nil)
			} else {
				s, _ = newTestServerWithExtras(t, nil, "")
			}
			root, err := s.memoryRoot()
			if err != nil {
				t.Fatal(err)
			}
			if err := memory.Init(root); err != nil {
				t.Fatal(err)
			}
			if _, err := memory.Write(root, "wiki/build.md", "# Build\n", ""); err != nil {
				t.Fatal(err)
			}
			index, _ := memory.Read(root, "INDEX.md")
			for version, status := range map[string]int{"stale": http.StatusConflict, index.Version: http.StatusOK} {
				body, _ := json.Marshal(map[string]string{"path": "wiki/build.md", "version": version})
				rr := httptest.NewRecorder()
				s.mux.ServeHTTP(rr, httptest.NewRequest("PUT", "/api/extras/memory/index", strings.NewReader(string(body))))
				if rr.Code != status {
					t.Fatalf("index update: %d %s", rr.Code, rr.Body)
				}
			}
			updated, _ := memory.Read(root, "INDEX.md")
			if !strings.Contains(updated.Content, "wiki/build.md") {
				t.Fatal("index link missing")
			}
		})
	}
}
