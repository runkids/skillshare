package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestGitStatus_ExcludesOnlyKnownEmptyExtraRootLock(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	s.cfg.ExtrasSource = filepath.Join(src, "custom-extras")
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	root := s.cfg.EffectiveExtrasSource()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, ".skillshare-extras.lock")
	if err := os.WriteFile(lock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	status := decodeBody[gitStatusResponse](t, serveJSON(t, s, http.MethodGet, "/api/git/status", ""))
	if status.IsDirty || len(status.Files) != 0 {
		t.Errorf("known empty admin lock leaked into status: %+v", status)
	}
	other := filepath.Join(src, "upstream", ".skillshare-extras.lock")
	if err := os.MkdirAll(filepath.Dir(other), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, nil, 0600); err != nil {
		t.Fatal(err)
	}
	status = decodeBody[gitStatusResponse](t, serveJSON(t, s, http.MethodGet, "/api/git/status", ""))
	if !status.IsDirty || len(status.Files) == 0 {
		t.Error("same-name upstream content hidden from dirty status")
	}
	rr := serveJSON(t, s, http.MethodPost, "/api/git/commit", `{"message":"preserve upstream content"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", rr.Code, rr.Body)
	}
	if got := testutil.RunGit(t, src, "ls-tree", "-r", "--name-only", "HEAD"); got != "upstream/.skillshare-extras.lock" {
		t.Errorf("wrong files staged: %q", got)
	}
}
