package server

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func postHubRefs(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest("POST", "/api/hub/refs", bytes.NewBufferString(body)))
	return rr
}

func TestHubRefs_RejectsInvalidSource(t *testing.T) {
	s, _ := newTestServer(t)
	for _, body := range []string{`{}`, `{"source":""}`, `not json`} {
		if rr := postHubRefs(t, s, body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rr.Code)
		}
	}
}

func TestHubRefs_NotPinnable(t *testing.T) {
	s, _ := newTestServer(t)
	rr := postHubRefs(t, s, `{"source":"https://git.example.com/o/r/skills/foo","ref":"v1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	want := `{"pinnable":false,"current":"","defaultBranch":"","branches":[],"tags":[]}`
	if got := bytes.TrimSpace(rr.Body.Bytes()); string(got) != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// A skill installed from an SSH or other-host remote can still switch branch
// from its detail page (issue #787).
func TestHubRefs_AnyGitListsUnpinnableRefs(t *testing.T) {
	s, _ := newTestServer(t)
	remote := t.TempDir()
	initGitRepo(t, remote)
	runGit(t, remote, "branch", "-M", "main")
	runGit(t, remote, "branch", "dev")

	rr := postHubRefs(t, s, `{"source":"file://`+remote+`//skills/foo","anyGit":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	want := `{"pinnable":false,"current":"","defaultBranch":"main","branches":["dev","main"],"tags":[]}`
	if got := bytes.TrimSpace(rr.Body.Bytes()); string(got) != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// Draft refs are read from the URLs alone, without asking the remote.
func TestHubDraftRefs(t *testing.T) {
	s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	body := `{"schemaVersion":1,"skills":[` +
		`{"name":"a","source":"github.com/acme/r/tree/v1.0/skills/a"},` +
		`{"name":"b","source":"acme/r/skills/b"}]}`
	s.handler.ServeHTTP(rr, httptest.NewRequest("POST", "/api/hub/drafts/import", bytes.NewBufferString(body)))
	var resp struct {
		Draft struct {
			Entries []struct {
				ID string `json:"id"`
			} `json:"entries"`
		} `json:"draft"`
		Refs map[string]string `json:"refs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || len(resp.Draft.Entries) != 2 {
		t.Fatalf("import: %d %s", rr.Code, rr.Body.String())
	}
	entries := resp.Draft.Entries
	want := map[string]string{entries[0].ID: "v1.0"}
	if !maps.Equal(resp.Refs, want) {
		t.Errorf("refs = %v, want %v", resp.Refs, want)
	}
}

func TestHandleSearch_BuiltinResultsCarryRef(t *testing.T) {
	s, src := newTestServer(t)
	os.MkdirAll(filepath.Join(src, "foo"), 0755)
	os.WriteFile(filepath.Join(src, "foo", "SKILL.md"), []byte("---\nname: foo\n---\nFoo."), 0644)
	os.WriteFile(filepath.Join(src, ".metadata.json"), []byte(`{"version":1,"entries":{"foo":{"source":"github.com/acme/r/tree/v2.0/skills/foo"}}}`), 0644)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/search?q=foo&hub=@builtin", nil))
	var resp struct {
		Results []struct {
			Ref string `json:"ref"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || len(resp.Results) != 1 {
		t.Fatalf("search: %d %s", rr.Code, rr.Body.String())
	}
	if resp.Results[0].Ref != "v2.0" {
		t.Errorf("ref = %q, want v2.0", resp.Results[0].Ref)
	}
}
