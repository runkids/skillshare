package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHandleOpenInEditor_UsesEchoAsEditor(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// `true` is a universally-available no-op binary on POSIX.
	body := openInEditorRequest{Editor: "true"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp openInEditorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PID == 0 {
		t.Errorf("expected pid > 0, got %d", resp.PID)
	}
	if resp.Path != filepath.Join(src, "my-skill") {
		t.Errorf("unexpected path: %s", resp.Path)
	}
}

func TestHandleOpenInEditor_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	raw, _ := json.Marshal(openInEditorRequest{Editor: "true"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/no-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOpenInEditor_HeadlessRefuses(t *testing.T) {
	t.Setenv("SKILLSHARE_HEADLESS", "1")
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	raw, _ := json.Marshal(openInEditorRequest{Editor: "true"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestHandleOpenInEditor_UnknownBinary(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	raw, _ := json.Marshal(openInEditorRequest{Editor: "this-is-not-a-real-editor-xyz"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestPickEditor_AutoPrefersEditorEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	cand, name, err := pickEditor("auto")
	if err != nil {
		t.Fatalf("pickEditor: %v", err)
	}
	if cand.bin != "true" || name != "true" {
		t.Errorf("expected bin=true, got bin=%s name=%s", cand.bin, name)
	}
}

func TestSplitCommand(t *testing.T) {
	head, rest := splitCommand("code --wait --new-window")
	if head != "code" {
		t.Errorf("expected head=code, got %s", head)
	}
	if len(rest) != 2 || rest[0] != "--wait" || rest[1] != "--new-window" {
		t.Errorf("unexpected rest: %v", rest)
	}
}
