package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"skillshare/internal/config"
)

func TestHandleAdoptPreviewRejectsOverlappingRootsAsBadRequest(t *testing.T) {
	s, sourcePath := newTestServer(t)
	s.cfg.Targets["universal"] = config.TargetConfig{Path: sourcePath, Mode: "merge"}
	req := httptest.NewRequest(http.MethodGet, "/api/adopt/preview", nil)
	rr := httptest.NewRecorder()

	s.handleAdoptPreview(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "overlaps skills source") {
		t.Fatalf("unexpected response: %s", rr.Body.String())
	}
}

func TestHandleAdoptApplyRejectsNonLocalOrCrossSiteRequests(t *testing.T) {
	for _, tc := range []struct {
		name       string
		remoteAddr string
		origin     string
	}{
		{name: "remote client", remoteAddr: "203.0.113.10:4321", origin: "http://skillshare.example"},
		{name: "cross-site origin", remoteAddr: "127.0.0.1:4321", origin: "https://evil.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestServer(t)
			req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", strings.NewReader(`{"names":["example"]}`))
			req.Host = "skillshare.example"
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.handleAdoptApply(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusForbidden, rr.Body.String())
			}
		})
	}
}

func TestHandleAdoptApplyRequiresJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", strings.NewReader(`{"names":["example"]}`))
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	s.handleAdoptApply(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusUnsupportedMediaType, rr.Body.String())
	}
}

func TestHandleAdoptApplyRequiresExplicitSelection(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", strings.NewReader(`{"names":[]}`))
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleAdoptApply(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "at least one skill name") {
		t.Fatalf("unexpected response: %s", rr.Body.String())
	}
}

func TestHandleAdoptApplyLimitsRequestBody(t *testing.T) {
	s, _ := newTestServer(t)
	payload := `{"names":["` + strings.Repeat("x", 70<<10) + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleAdoptApply(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
}

func TestHandleAdoptApplyRejectsNamesOutsidePreview(t *testing.T) {
	agentsPath := t.TempDir()
	s, _ := newTestServerWithTargets(t, map[string]string{"universal": agentsPath})
	req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", strings.NewReader(`{"names":["not-detected"]}`))
	req.RemoteAddr = "127.0.0.1:4321"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleAdoptApply(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "not adoptable") {
		t.Fatalf("unexpected response: %s", rr.Body.String())
	}
}

type gatedJSONReader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	sent    bool
}

type observedJSONReader struct {
	reader *strings.Reader
	done   chan struct{}
	once   sync.Once
}

func (r *observedJSONReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err == io.EOF {
		r.once.Do(func() { close(r.done) })
	}
	return n, err
}

func (r *gatedJSONReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	if r.sent {
		return 0, io.EOF
	}
	r.sent = true
	return copy(p, `{"names":["missing"]}`), nil
}

func TestHandleAdoptApply_DoesNotBlockReadsWhileReceivingBody(t *testing.T) {
	s, _ := newTestServer(t)
	body := &gatedJSONReader{started: make(chan struct{}), release: make(chan struct{})}
	applyDone := make(chan struct{})
	go func() {
		defer close(applyDone)
		req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", body)
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("Content-Type", "application/json")
		s.handleAdoptApply(httptest.NewRecorder(), req)
	}()

	<-body.started
	previewDone := make(chan struct{})
	go func() {
		defer close(previewDone)
		req := httptest.NewRequest(http.MethodGet, "/api/adopt/preview", nil)
		s.handleAdoptPreview(httptest.NewRecorder(), req)
	}()

	blocked := false
	select {
	case <-previewDone:
	case <-time.After(2 * time.Second):
		blocked = true
	}
	close(body.release)
	<-applyDone
	if blocked {
		t.Fatal("adopt apply held the config lock while waiting for its request body")
	}
}

func TestHandleAdoptApply_WaitsForConfigMutationBeforeFilesystemOperation(t *testing.T) {
	agentsPath := filepath.Join(t.TempDir(), "agents")
	s, sourcePath := newTestServerWithTargets(t, map[string]string{"universal": agentsPath})
	addSkill(t, agentsPath, "example")
	body := &observedJSONReader{
		reader: strings.NewReader(`{"names":["example"]}`),
		done:   make(chan struct{}),
	}

	s.mu.Lock()
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", body)
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("Content-Type", "application/json")
		s.handleAdoptApply(httptest.NewRecorder(), req)
	}()

	select {
	case <-body.done:
	case <-time.After(2 * time.Second):
		s.mu.Unlock()
		t.Fatal("adopt apply did not read its body before waiting for config mutation")
	}
	select {
	case <-handlerDone:
		s.mu.Unlock()
		t.Fatal("adopt apply raced an in-progress config mutation")
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "example")); !os.IsNotExist(err) {
		s.mu.Unlock()
		t.Fatalf("adopt mutated source before config mutation completed: %v", err)
	}
	s.mu.Unlock()

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("adopt apply did not resume after config mutation completed")
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "example", "SKILL.md")); err != nil {
		t.Fatalf("adopt did not migrate skill after config mutation completed: %v", err)
	}
}

func TestHandleAdoptApplySerializesFilesystemOperationAfterReadingBody(t *testing.T) {
	s, _ := newTestServer(t)
	body := &observedJSONReader{
		reader: strings.NewReader(`{"names":["missing"]}`),
		done:   make(chan struct{}),
	}
	s.adoptMu.Lock()
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		req := httptest.NewRequest(http.MethodPost, "/api/adopt/apply", body)
		req.RemoteAddr = "127.0.0.1:4321"
		req.Header.Set("Content-Type", "application/json")
		s.handleAdoptApply(httptest.NewRecorder(), req)
	}()

	select {
	case <-body.done:
	case <-time.After(2 * time.Second):
		s.adoptMu.Unlock()
		t.Fatal("adopt apply did not finish reading its request body")
	}
	select {
	case <-handlerDone:
		s.adoptMu.Unlock()
		t.Fatal("adopt apply bypassed the filesystem-operation lock")
	case <-time.After(100 * time.Millisecond):
	}
	s.adoptMu.Unlock()

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("adopt apply did not resume after the filesystem-operation lock was released")
	}
}
