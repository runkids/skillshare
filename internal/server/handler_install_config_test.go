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
)

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
