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
	"skillshare/internal/testutil"
)

// setServerGitRoot rewrites the test server's config file with a given git_root
// scope. The auto-reload middleware reloads config from disk on every /api/*
// request, so persisting to the file (not mutating s.cfg) is what the handlers
// actually see. src is the skills source preserved from newTestServer.
func setServerGitRoot(t *testing.T, scope, src string) {
	t.Helper()
	content := "git_root: " + scope + "\nsource: " + src + "\nmode: merge\ntargets: {}\n"
	if err := os.WriteFile(config.ConfigPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandleGitCommit_NoRemote_CreatesLocalCommit(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addSkill(t, src, "local-skill")

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"local checkpoint"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp pushResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response: %+v", resp)
	}

	message := testutil.RunGit(t, src, "log", "-1", "--pretty=%s")
	if message != "local checkpoint" {
		t.Fatalf("commit message = %q, want %q", message, "local checkpoint")
	}

	status := testutil.RunGit(t, src, "status", "--porcelain")
	if status != "" {
		t.Fatalf("expected clean working tree, got %q", status)
	}
}

func TestHandleGitCommit_DryRun_DoesNotCreateCommit(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addSkill(t, src, "dry-run-skill")

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"dry run checkpoint","dryRun":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	count := testutil.RunGit(t, src, "rev-list", "--count", "HEAD")
	if count != "1" {
		t.Fatalf("commit count = %q, want 1", count)
	}
}

func TestHandleGitCommit_NoChanges(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp pushResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "nothing to commit (working tree clean)" {
		t.Fatalf("message = %q, want nothing to commit", resp.Message)
	}
}

func initServerGitRepo(t *testing.T, dir string) {
	t.Helper()

	testutil.RunGit(t, dir, "init")
	testutil.ConfigureGitUser(t, dir)
	testutil.RunGit(t, dir, "commit", "--allow-empty", "-m", "initial")
}

func addFailingOriginRemote(t *testing.T, dir string) {
	t.Helper()
	missingRemote := "file://" + filepath.Join(t.TempDir(), "missing.git")
	testutil.RunGit(t, dir, "remote", "add", "origin", missingRemote)
}

func TestHandleGitBranches_FetchFailureReturnsError(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addFailingOriginRemote(t, src)

	req := httptest.NewRequest(http.MethodGet, "/api/git/branches?fetch=true", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "git fetch failed") {
		t.Fatalf("response %q does not mention fetch failure", rr.Body.String())
	}
}

func TestHandleGitCheckout_FetchFailureStopsCheckout(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	base := testutil.RunGit(t, src, "branch", "--show-current")
	testutil.RunGit(t, src, "checkout", "-b", "feature")
	testutil.RunGit(t, src, "checkout", base)
	addFailingOriginRemote(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/git/checkout", strings.NewReader(`{"branch":"feature"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "git fetch failed") {
		t.Fatalf("response %q does not mention fetch failure", rr.Body.String())
	}
	if current := testutil.RunGit(t, src, "branch", "--show-current"); current != base {
		t.Fatalf("current branch = %q, want %q", current, base)
	}
}

// POST /api/git/root with a non-default scope initializes a repo at that scope
// directory and persists git_root to config.
func TestHandleSetGitRoot_SwitchToAgents_InitsAndPersists(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":"agents"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["scope"] != "agents" {
		t.Fatalf("scope = %v, want agents", resp["scope"])
	}
	dir, _ := resp["gitRoot"].(string)
	if dir == "" {
		t.Fatalf("missing gitRoot in response: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git initialized at agents scope %s: %v", dir, err)
	}
	if s.cfg.GitRoot != "agents" {
		t.Errorf("cfg.GitRoot = %q, want agents (persisted)", s.cfg.GitRoot)
	}
}

// POST /api/git/root with a remoteURL initializes the scope repo and wires the
// "origin" remote on it in one request (UI parity with the CLI's
// `init --git-root <scope> --remote <url>`).
func TestHandleSetGitRoot_WithRemote_WiresOrigin(t *testing.T) {
	s, _ := newTestServer(t)

	const remote = "https://example.com/skills.git"
	body := `{"scope":"agents","remoteURL":"` + remote + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	dir, _ := resp["gitRoot"].(string)
	if dir == "" {
		t.Fatalf("missing gitRoot in response: %+v", resp)
	}

	got := testutil.RunGit(t, dir, "remote", "get-url", "origin")
	if got != remote {
		t.Errorf("origin = %q, want %q", got, remote)
	}
}

// POST /api/git/root rejects unknown or empty scopes with 400.
func TestHandleSetGitRoot_InvalidScope_Rejected(t *testing.T) {
	s, _ := newTestServer(t)

	for _, scope := range []string{`"bogus"`, `""`} {
		req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":`+scope+`}`))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("scope %s: expected 400, got %d: %s", scope, rr.Code, rr.Body.String())
		}
	}
}

// git_root is global-only; project mode must reject the request.
func TestHandleSetGitRoot_ProjectMode_Rejected(t *testing.T) {
	s, _ := newProjectTargetServer(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":"root"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 in project mode, got %d: %s", rr.Code, rr.Body.String())
	}
}

// GET /api/git/status reports a scope mismatch when the configured root has no
// repo but a sibling scope directory does.
func TestHandleGitStatus_ScopeMismatch(t *testing.T) {
	s, _ := newTestServer(t)

	// git_root defaults to skills (the source). Plant a stray repo at the agents
	// scope dir so the configured root lacks .git but agents has one.
	agentsDir := s.cfg.EffectiveAgentsSource()
	if err := os.MkdirAll(filepath.Join(agentsDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/git/status", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp gitStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.ScopeMismatch {
		t.Fatalf("expected scopeMismatch=true, got: %+v", resp)
	}
	if resp.MismatchScope != "agents" {
		t.Errorf("mismatchScope = %q, want agents", resp.MismatchScope)
	}
	if resp.MismatchDir != agentsDir {
		t.Errorf("mismatchDir = %q, want %q", resp.MismatchDir, agentsDir)
	}
}

// An invalid (hand-edited) git_root must not silently fall back to the skills
// scope — git operations reject it with a 400, mirroring the CLI's resolveGitRoot.
func TestHandleGitCommit_InvalidGitRoot_Rejected(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	setServerGitRoot(t, "agnets", src) // typo for "agents" — not a valid scope

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"x"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid git_root, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid git_root") {
		t.Errorf("expected 'invalid git_root' in error, got: %s", rr.Body.String())
	}
}

// A nested git repo at the root scope blocks commit with a 400 — committing it
// would record an empty submodule and silently drop its files.
func TestHandleGitCommit_NestedRepo_Blocked(t *testing.T) {
	s, src := newTestServer(t)
	setServerGitRoot(t, "root", src)
	base := config.BaseDir() // root scope operates on BaseDir
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	initServerGitRepo(t, base)
	if err := os.MkdirAll(filepath.Join(base, "vendored", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"x"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 nested-repo block, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "nested git repositories") {
		t.Errorf("expected nested-repo error, got: %s", rr.Body.String())
	}
}

// A root-scope dry-run must be strictly read-only — it must not untrack
// config.yaml (no `git rm --cached`). Regression for the guard mutating the
// repo before the dry-run branch.
func TestHandleGitCommit_DryRun_DoesNotUntrackConfig(t *testing.T) {
	s, src := newTestServer(t)
	setServerGitRoot(t, "root", src)
	base := config.BaseDir() // root scope operates on BaseDir
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	// A repo that (wrongly) tracks config.yaml: the real run would untrack it,
	// but a dry-run must leave it alone.
	if err := os.WriteFile(filepath.Join(base, "config.yaml"), []byte("x: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, base, "add", "config.yaml")
	testutil.RunGit(t, base, "commit", "-m", "initial with config")
	// A stageable change so the request reaches the dry-run branch.
	if err := os.WriteFile(filepath.Join(base, "note.md"), []byte("# n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"x","dryRun":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	tracked := testutil.RunGit(t, base, "ls-files", "--", "config.yaml")
	if !strings.Contains(tracked, "config.yaml") {
		t.Error("dry-run must not untrack config.yaml, but it is no longer tracked")
	}
}
