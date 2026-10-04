package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

// Matches the five-entry CLI layout in TestSkillfollowBehaviorMatrix.
func skillfollowServerFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, source := newTestServer(t)
	external := t.TempDir()
	for _, rel := range []string{"repo/a", "group/c", "hidden/secret"} {
		addSkill(t, external, rel)
	}
	testutil.RunGit(t, external, "init", "repo")
	for name, target := range map[string]string{"_repo": "repo", "group": "group", "undeclared": "hidden"} {
		if err := os.Symlink(filepath.Join(external, target), filepath.Join(source, name)); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"rejected": "not a directory", ".skillfollow": "_repo\ngroup\nmissing\nrejected\n"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return s, source, external
}

func snapshotFollowedTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		files[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestServerSkillfollowBehaviorMatrix(t *testing.T) {
	s, source, external := skillfollowServerFixture(t)
	before := snapshotFollowedTree(t, external)
	matrix := []struct {
		name     string
		method   string
		path     string
		body     string
		status   int
		visible  []string
		absent   []string
		reported []string
	}{
		{"skills", "GET", "/api/resources", "", 200, []string{"_repo/a", "group/c"}, []string{"secret"}, []string{"\"isInRepo\":true"}},
		{"overview", "GET", "/api/overview", "", 200, []string{"_repo"}, []string{"secret"}, []string{"\"skillCount\":2", "\"topLevelCount\":2"}},
		// pending 3c: update-all currently excludes both followed trees.
		{"update", "POST", "/api/update", `{"all":true}`, 200, nil, []string{"\"_repo\"", "group/c", "secret"}, []string{"\"results\":null"}},
		// pending 3c: followed repo check and local group discovery are not connected yet.
		{"check", "GET", "/api/check", "", 200, nil, []string{"\"_repo\"", "group/c", "secret"}, []string{"\"tracked_repos\":[]"}},
		{"hub", "GET", "/api/hub/index", "", 200, []string{"_repo/a", "group/c"}, []string{"secret"}, nil},
		{"hub-candidates", "GET", "/api/hub/drafts/candidates", "", 200, []string{"_repo/a", "group/c"}, []string{"secret"}, nil},
		{"skill-content", "GET", "/api/resources/group__c", "", 200, []string{"group/c"}, []string{"secret"}, nil},
		{"uninstall-repo", "DELETE", "/api/repos/_repo", "", 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"uninstall", "DELETE", "/api/resources/group__c", "", 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"toggle", "POST", "/api/resources/group__c/disable", "", 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"toggle-repo", "POST", "/api/resources/_repo__a/disable", "", 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"toggle-batch", "POST", "/api/resources/batch/toggle", `{"names":["group__c","_repo__a"],"enable":false}`, 200, nil, nil, []string{"is a link; edit its target directly", "\"failed\":2"}},
		{"targets", "PATCH", "/api/resources/group__c/targets", `{"target":"claude"}`, 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"targets-repo", "PATCH", "/api/resources/_repo__a/targets", `{"target":"claude"}`, 409, nil, nil, []string{"is a link; edit its target directly"}},
		{"targets-batch", "POST", "/api/resources/batch/targets", `{"folder":"*","target":"claude"}`, 200, nil, nil, []string{"is a link; edit its target directly", "\"updated\":0"}},
		{"uninstall-batch", "POST", "/api/uninstall/batch", `{"names":["group__c","_repo","_repo__a"],"force":true}`, 200, nil, nil, []string{"is a link; edit its target directly", "\"failed\":3"}},
		{"content-write", "PUT", "/api/resources/group__c/content", `{"content":"changed"}`, 409, nil, nil, []string{"is a link; edit its target directly"}},
	}
	for _, row := range matrix {
		t.Run(row.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, httptest.NewRequest(row.method, row.path, strings.NewReader(row.body)))
			body := rr.Body.String()
			if rr.Code != row.status {
				t.Fatalf("status %d, want %d: %s", rr.Code, row.status, body)
			}
			if !json.Valid(rr.Body.Bytes()) {
				t.Fatalf("invalid JSON: %s", body)
			}
			for _, value := range append(row.visible, row.reported...) {
				if !strings.Contains(body, value) {
					t.Errorf("missing %q: %s", value, body)
				}
			}
			for _, value := range row.absent {
				if strings.Contains(body, value) {
					t.Errorf("unexpected %q: %s", value, body)
				}
			}
			if !reflect.DeepEqual(before, snapshotFollowedTree(t, external)) {
				t.Fatal("request changed followed content")
			}
			for _, name := range []string{"_repo", "group", "undeclared"} {
				if _, err := os.Readlink(filepath.Join(source, name)); err != nil {
					t.Fatalf("link changed: %s: %v", name, err)
				}
			}
			if _, err := os.Stat(filepath.Join(source, ".skillignore")); !os.IsNotExist(err) {
				t.Fatal("refused toggle changed .skillignore")
			}
		})
	}
}

func TestServerSkillfollowActiveTargets(t *testing.T) {
	s, _, external := skillfollowServerFixture(t)
	disabled := false
	s.cfg.Targets = map[string]config.TargetConfig{
		"active": {Path: filepath.Join(external, "repo")},
		"off":    {Skills: &config.ResourceTargetConfig{Path: filepath.Join(external, "group"), Enabled: &disabled}},
	}
	follow := s.skillFollowSet()
	states := map[string]string{}
	for _, entry := range follow.Entries() {
		states[entry.Name] = string(entry.State)
	}
	if states["_repo"] != "target-overlap" || states["group"] != "followed" {
		t.Fatal(states)
	}
}

func TestServerSkillfollowProjectPolicy(t *testing.T) {
	s, source, _ := skillfollowServerFixture(t)
	root := t.TempDir()
	pcfg := &config.ProjectConfig{Sources: config.ProjectSources{Skills: source}}
	if err := pcfg.Save(root); err != nil {
		t.Fatal(err)
	}
	s = NewProject(s.cfg, pcfg, root, "127.0.0.1:0", "", "")
	for _, path := range []string{"/api/resources", "/api/overview"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != 200 || !strings.Contains(rr.Body.String(), "_repo") {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
	// Project root is the staging boundary, even when the source is external.
	if err := os.Symlink(root, filepath.Join(source, "staged")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow.local"), []byte("staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	follow := s.skillFollowSet()
	found := false
	for _, entry := range follow.Entries() {
		if entry.Name == "staged" {
			found = true
			if entry.State != "inside-git-root" {
				t.Fatal(entry)
			}
		}
	}
	if !found {
		t.Fatal("missing staged classification")
	}
}
