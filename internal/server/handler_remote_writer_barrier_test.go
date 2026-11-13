package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/memory"
)

func TestMemoryWrite_WaitsForRemotePublication(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			extra := config.ExtraConfig{Name: "memory", Remote: "example-org/ai-hub/memory", Branch: "before", Targets: []config.ExtraTargetConfig{}}
			var s *Server
			if project {
				s, _ = newTestProjectServerWithExtras(t, []config.ExtraConfig{extra})
			} else {
				s, _ = newTestServerWithExtras(t, []config.ExtraConfig{extra}, "")
			}
			root, err := s.memoryRoot()
			if err != nil {
				t.Fatal(err)
			}
			if err := memory.Init(root); err != nil {
				t.Fatal(err)
			}
			unlock, err := install.LockExtrasRoot(filepath.Dir(root))
			if err != nil {
				t.Fatal(err)
			}
			defer unlock()
			entered := make(chan struct{}, 1)
			old := lockRemoteExtraRoot
			lockRemoteExtraRoot = func(path string) (func(), error) {
				entered <- struct{}{}
				return install.LockExtrasRoot(path)
			}
			defer func() { lockRemoteExtraRoot = old }()
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				rr := httptest.NewRecorder()
				s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/extras/memory/notes/content", strings.NewReader(`{"path":"new.md","content":"# Saved after publication\n"}`)))
				done <- rr
			}()
			select {
			case rr := <-done:
				t.Fatalf("writer bypassed held root lock: %d %s", rr.Code, rr.Body)
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("writer did not reach lock boundary")
			}
			// The writer is now inside the real flock acquisition while another
			// participant owns it. Publish config before letting it proceed.
			if _, err := os.Stat(filepath.Join(root, "new.md")); !os.IsNotExist(err) {
				t.Fatalf("write happened under held lock: %v", err)
			}
			if project {
				fresh, err := config.LoadProjectReadOnly(s.projectRoot)
				if err != nil {
					t.Fatal(err)
				}
				fresh.Extras[0].Branch = "published"
				if err := fresh.Save(s.projectRoot); err != nil {
					t.Fatal(err)
				}
			} else {
				fresh, err := config.LoadReadOnly()
				if err != nil {
					t.Fatal(err)
				}
				fresh.Extras[0].Branch = "published"
				if err := fresh.Save(); err != nil {
					t.Fatal(err)
				}
			}
			unlock()
			select {
			case rr := <-done:
				if rr.Code != http.StatusOK {
					t.Fatalf("%d %s", rr.Code, rr.Body)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("writer did not finish after unlock")
			}
			if s.extrasConfig()[0].Branch != "published" {
				t.Error("writer used pre-publication config")
			}
			if data, err := os.ReadFile(filepath.Join(root, "new.md")); err != nil || string(data) != "# Saved after publication\n" {
				t.Errorf("actual write missing: %q %v", data, err)
			}
		})
	}
}
