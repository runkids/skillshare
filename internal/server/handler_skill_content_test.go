package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlePutSkillContent_WritesFile(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := skillContentRequest{Content: "---\nname: my-skill\ndescription: edited\n---\n# Updated"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp skillContentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response decode: %v", err)
	}
	if resp.BytesWritten != len(body.Content) {
		t.Errorf("expected %d bytes written, got %d", len(body.Content), resp.BytesWritten)
	}

	on := filepath.Join(src, "my-skill", "SKILL.md")
	got, err := os.ReadFile(on)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(got) != body.Content {
		t.Errorf("file content mismatch:\nwant: %q\ngot:  %q", body.Content, got)
	}
}

func TestHandlePutSkillContent_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	body := skillContentRequest{Content: "foo"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/nope/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutSkillContent_InvalidJSON(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutSkillContent_TooLarge(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// Assemble a 3 MiB body — exceeds the 2 MiB ceiling.
	huge := make([]byte, 3*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	body := skillContentRequest{Content: string(huge)}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
}

func TestHandlePutSkillContent_AtomicTempCleanup(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := skillContentRequest{Content: "new content"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected %d", rr.Code)
	}

	entries, err := os.ReadDir(filepath.Join(src, "my-skill"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".skillshare-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestHandlePutSkillContent_RejectsTraversal(t *testing.T) {
	// The route only exposes {name}; Go's mux rejects a literal "/" inside
	// {name}, but we still want to confirm `..` as a name doesn't escape.
	s, src := newTestServer(t)
	addSkill(t, src, "victim")

	body := skillContentRequest{Content: "boom"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/..%2Fescape/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	// Any of: 400 / 404 is acceptable — what matters is that the victim's file
	// is untouched.
	got, err := os.ReadFile(filepath.Join(src, "victim", "SKILL.md"))
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if !strings.Contains(string(got), "# victim") {
		t.Errorf("victim content was modified: %q", got)
	}
}
