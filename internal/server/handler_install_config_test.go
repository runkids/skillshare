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
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

func TestHandleInstallFromConfig_FollowsAdvancedProjectLock(t *testing.T) {
	s, project := newTestProjectServerWithExtras(t, nil)
	base := t.TempDir()
	remote := testutil.SetupBareRemoteRepo(t, base)
	testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"demo/SKILL.md": "# Original\n"})
	source := "file://" + remote + "//demo"
	s.projectCfg.Skills = []config.SkillEntry{{Name: "demo", Source: source}}
	if err := s.projectCfg.Save(project); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handleInstallFromConfig(rr, httptest.NewRequest(http.MethodPost, "/api/install/from-config", nil))
	installed := filepath.Join(s.skillsSource(), "demo", "SKILL.md")
	if body, err := os.ReadFile(installed); err != nil || string(body) != "# Original\n" {
		t.Fatalf("initial install: %q, %v; response: %s", body, err, rr.Body.String())
	}

	seed := filepath.Join(base, "seed-main")
	if err := os.WriteFile(filepath.Join(seed, "demo", "SKILL.md"), []byte("# New pin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, seed, "commit", "-am", "advance skill")
	testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
	lockDir := filepath.Join(project, ".skillshare")
	lock, err := install.LoadLock(lockDir)
	if err != nil {
		t.Fatal(err)
	}
	pin := lock.Skills["demo"]
	pin.Commit = strings.TrimSpace(testutil.RunGit(t, seed, "rev-parse", "HEAD"))
	lock.Skills["demo"] = pin
	if err := lock.Save(lockDir); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	s.handleInstallFromConfig(rr, httptest.NewRequest(http.MethodPost, "/api/install/from-config", nil))
	if body, err := os.ReadFile(installed); err != nil || string(body) != "# New pin\n" {
		t.Fatalf("dashboard did not follow advanced lock: %q, %v; response: %s", body, err, rr.Body.String())
	}
}

func TestHandleMissingConfigEntries_ListsAbsentEntries(t *testing.T) {
	s, src := newTestServer(t)
	store := install.LoadMetadataOrNew(src)
	store.Set("_team-skills", &install.MetadataEntry{Source: "https://github.com/team/skills.git", Tracked: true, Branch: "main"})
	if err := store.Save(src); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	rr := httptest.NewRecorder()
	s.handleMissingConfigEntries(rr, httptest.NewRequest(http.MethodGet, "/api/install/missing", nil))
	var resp struct {
		Entries []configEntryInfo `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	want := configEntryInfo{Name: "_team-skills", Source: "https://github.com/team/skills.git", Tracked: true, Branch: "main"}
	if len(resp.Entries) != 1 || resp.Entries[0] != want {
		t.Fatalf("entries = %+v, want [%+v]", resp.Entries, want)
	}
}

func TestHandleInstallFromConfig_NothingMissing(t *testing.T) {
	s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	s.handleInstallFromConfig(rr, httptest.NewRequest(http.MethodPost, "/api/install/from-config", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleInstallFromConfig_AllFailedLogsError(t *testing.T) {
	s, src := newTestServer(t)
	store := install.LoadMetadataOrNew(src)
	store.Set("broken", &install.MetadataEntry{Source: ""})
	if err := store.Save(src); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	rr := httptest.NewRecorder()
	s.handleInstallFromConfig(rr, httptest.NewRequest(http.MethodPost, "/api/install/from-config", nil))
	entries, _ := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if len(entries) != 1 || entries[0].Status != "error" || !strings.Contains(entries[0].Message, "broken: ") {
		t.Errorf("oplog = %+v, want one error entry naming the skill", entries)
	}
}

func TestHandleMissingConfigEntries_ProjectModeChecksProjectSource(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.Skills = []config.SkillEntry{{Name: "present", Source: "github.com/team/skills/present"}, {Name: "absent", Source: "github.com/team/skills/absent"}}
	// Only the project source has "present"; the global source has neither.
	if err := os.MkdirAll(filepath.Join(s.projectCfg.EffectiveSkillsSource(root), "present"), 0o755); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.handleMissingConfigEntries(rr, httptest.NewRequest(http.MethodGet, "/api/install/missing", nil))
	var resp struct {
		Entries []configEntryInfo `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].Name != "absent" {
		t.Fatalf("entries = %+v, want only absent", resp.Entries)
	}
}
