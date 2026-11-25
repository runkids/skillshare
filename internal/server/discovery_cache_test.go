package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/install"
)

// gitSkillRepo returns a file:// source holding one skill named demo.
func gitSkillRepo(t *testing.T) (repoDir, source string) {
	t.Helper()
	repoDir = t.TempDir()
	initGitRepo(t, repoDir)
	if err := os.MkdirAll(filepath.Join(repoDir, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "demo", "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "add demo"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}
	return repoDir, "file://" + repoDir
}

func postJSON(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rr
}

// The dashboard lists a repository's skills and then installs some of them.
// The install must copy from the listing's clone, not download the repository again.
func TestHandleInstallBatch_InstallsFromTheDiscoverClone(t *testing.T) {
	s, skillsDir := newTestServer(t)
	t.Cleanup(s.discoveries.close)
	repoDir, source := gitSkillRepo(t)

	if rr := postJSON(t, s, "/api/discover", `{"source":"`+source+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("discover: %d %s", rr.Code, rr.Body.String())
	}
	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatal(err)
	}
	rr := postJSON(t, s, "/api/install/batch", `{"source":"`+source+`","skills":[{"name":"demo","path":"demo"}]}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("install: %d %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "demo", "SKILL.md")); err != nil {
		t.Fatalf("demo was not installed from the kept clone (%s): %v", rr.Body.String(), err)
	}
}

func keptClone(t *testing.T, c *discoveryCache, key string) string {
	t.Helper()
	dir := t.TempDir()
	c.keep(key, &install.DiscoveryResult{RepoPath: dir, Source: &install.Source{Type: install.SourceTypeGitHTTPS}})
	return dir
}

func expireAll(c *discoveryCache) {
	c.mu.Lock()
	for key, k := range c.kept {
		k.expires = time.Now().Add(-time.Second)
		c.kept[key] = k
	}
	c.mu.Unlock()
}

func TestDiscoveryCache_RemovesAnExpiredClone(t *testing.T) {
	var c discoveryCache
	dir := keptClone(t, &c, "a")
	expireAll(&c)
	c.trim()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expired clone still on disk: %v", err)
	}
}

// An install owns the clone it took, so expiry waits until the install gives it back.
func TestDiscoveryCache_KeepsACloneInUseUntilItIsGivenBack(t *testing.T) {
	var c discoveryCache
	dir := keptClone(t, &c, "a")
	_, giveBack := c.take("a")
	if giveBack == nil {
		t.Fatal("kept clone was not found")
	}
	if again, _ := c.take("a"); again != nil {
		t.Fatal("a second install got a clone already in use")
	}
	expireAll(&c)
	c.trim()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("clone in use was removed: %v", err)
	}
	giveBack()
	expireAll(&c)
	c.trim()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("clone given back did not expire: %v", err)
	}
}

// Listings that are never installed, such as the hub's, must not pile up clones.
func TestDiscoveryCache_DropsTheOldestPastTheLimit(t *testing.T) {
	var c discoveryCache
	oldest := keptClone(t, &c, "first")
	for i := 1; i < discoveryKeepMax; i++ {
		keptClone(t, &c, string(rune('a'+i)))
	}
	keptClone(t, &c, "last")
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("oldest clone kept past the limit: %v", err)
	}
}

func TestDiscoveryCache_CloseRemovesKeptClones(t *testing.T) {
	var c discoveryCache
	dir := keptClone(t, &c, "a")
	c.close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("kept clone survived shutdown: %v", err)
	}
}

// A local source is the user's own folder, never a clone to remove.
func TestDiscoveryCache_LeavesALocalSourceAlone(t *testing.T) {
	var c discoveryCache
	dir := t.TempDir()
	c.keep("a", &install.DiscoveryResult{RepoPath: dir, Source: &install.Source{Type: install.SourceTypeLocalPath, Path: dir}})
	c.close()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("local source was removed: %v", err)
	}
}
