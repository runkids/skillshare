package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	ssync "skillshare/internal/sync"
)

func TestHandleSync_MergeMode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	body := `{"dryRun":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 sync result, got %d", len(resp.Results))
	}
	if resp.Results[0]["target"] != "claude" {
		t.Errorf("expected target 'claude', got %v", resp.Results[0]["target"])
	}
}

func TestHandleSync_BacksUpTargetsFirst(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")
	addSkill(t, tgtPath, "local-only")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if backups, err := backup.List(); err != nil || len(backups) != 1 {
		t.Fatalf("expected one pre-sync backup, got %d (err: %v)", len(backups), err)
	}
}

func TestHandleSync_IgnoredSkillNotPrunedFromRegistry(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Create a skill with install metadata (so it appears in registry)
	addSkill(t, src, "kept-skill")
	addSkillMeta(t, src, "kept-skill", "github.com/user/kept")

	// Create another skill that will be ignored
	addSkill(t, src, "ignored-skill")
	addSkillMeta(t, src, "ignored-skill", "github.com/user/ignored")

	// Add .skillignore to exclude the second skill
	os.WriteFile(filepath.Join(src, ".skillignore"), []byte("ignored-skill\n"), 0644)

	// Pre-populate store with both entries and persist to disk
	// (server auto-reloads metadata from disk on each request)
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("kept-skill", &install.MetadataEntry{Source: "github.com/user/kept"})
	s.skillsStore.Set("ignored-skill", &install.MetadataEntry{Source: "github.com/user/ignored"})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Run sync (non-dry-run)
	body := `{"dryRun":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both entries should survive — ignored skill still exists on disk
	names := s.skillsStore.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 metadata entries after sync, got %d: %v", len(names), names)
	}
}

func TestHandleSync_NoTargets(t *testing.T) {
	s, _ := newTestServer(t) // no targets configured

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results for no targets, got %d", len(resp.Results))
	}
}

func TestHandleSync_AgentPrunesOrphanWhenSourceEmpty(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "claude-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	if err := os.MkdirAll(agentTarget, 0o755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	orphanPath := filepath.Join(agentTarget, "tutor.md")
	if err := os.Symlink(filepath.Join(agentSource, "tutor.md"), orphanPath); err != nil {
		t.Fatalf("seed orphan agent symlink: %v", err)
	}

	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: agentTarget},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Lstat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("expected orphan agent symlink to be pruned, got err=%v", err)
	}

	var resp struct {
		Results []struct {
			Target string   `json:"target"`
			Pruned []string `json:"pruned"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sync response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 sync result, got %d", len(resp.Results))
	}
	if resp.Results[0].Target != "claude" {
		t.Fatalf("expected claude target, got %q", resp.Results[0].Target)
	}
	if len(resp.Results[0].Pruned) != 1 || resp.Results[0].Pruned[0] != "tutor.md" {
		t.Fatalf("expected pruned tutor.md, got %+v", resp.Results[0].Pruned)
	}
}

func TestHandleSync_AgentsRespectTargetExcludeFilter(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "claude-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	if err := os.MkdirAll(agentTarget, 0o755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentSource, "tutor.md"), []byte("# Tutor"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{
			Path:    agentTarget,
			Exclude: []string{"*"},
		},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Lstat(filepath.Join(agentTarget, "tutor.md")); !os.IsNotExist(err) {
		t.Fatalf("excluded agent should not be synced, got err=%v", err)
	}
}

func TestHandleSync_AgentsPruneExcludedTargetAgent(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "claude-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	if err := os.MkdirAll(agentTarget, 0o755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	sourceAgent := filepath.Join(agentSource, "tutor.md")
	if err := os.WriteFile(sourceAgent, []byte("# Tutor"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	targetAgent := filepath.Join(agentTarget, "tutor.md")
	if err := os.Symlink(sourceAgent, targetAgent); err != nil {
		t.Fatalf("seed synced agent: %v", err)
	}

	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{
			Path:    agentTarget,
			Exclude: []string{"*"},
		},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Lstat(targetAgent); !os.IsNotExist(err) {
		t.Fatalf("excluded synced agent should be pruned, got err=%v", err)
	}
}

// newAgentSyncServer returns a server whose agents source holds tutor.md.
func newAgentSyncServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, src := newTestServer(t)
	agentSource := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentSource, "tutor.md"), []byte("# Tutor"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.cfg.AgentsSource = agentSource
	return s, src, agentSource
}

func TestHandleSync_AgentTargetFailureCountsInLog(t *testing.T) {
	s, _, _ := newAgentSyncServer(t)
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-agents"), Include: []string{"["}},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool {
		return strings.HasPrefix(w, "agent sync failed for claude: invalid agent filter: ")
	}) {
		t.Fatalf("expected invalid filter warning, got %v", resp.Warnings)
	}
	entries, err := oplog.Read(config.ConfigPath(), oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read ops log: %v (%d entries)", err, len(entries))
	}
	if e := entries[0]; e.Status != "error" || e.Args["targets_failed"] != float64(1) {
		t.Fatalf("expected error sync with targets_failed 1, got status %q args %v", e.Status, e.Args)
	}
}

func TestHandleSync_LogCountsEachFailedTargetOnce(t *testing.T) {
	s, src, _ := newAgentSyncServer(t)
	addSkill(t, src, "alpha")
	s.cfg.Targets["both"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "both-skills"), Include: []string{"["}},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "both-agents"), Include: []string{"["}},
	}
	s.cfg.Targets["agent-only"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "agent-only-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "agent-only-agents"), Include: []string{"["}},
	}
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-agents")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	entries, err := oplog.Read(config.ConfigPath(), oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read ops log: %v (%d entries)", err, len(entries))
	}
	e := entries[0]
	got, _ := json.Marshal(e.Args["failed_targets"])
	if e.Status != "partial" || e.Args["targets_failed"] != float64(2) || e.Args["targets_total"] != float64(3) || string(got) != `["agent-only","both"]` {
		t.Fatalf("expected partial sync with 2 of 3 targets failed (agent-only, both), got status %q args %v", e.Status, e.Args)
	}
}

func TestHandleSync_AgentsSkipTargetWithoutAgentsPathSilently(t *testing.T) {
	s, _, _ := newAgentSyncServer(t)
	s.cfg.Targets["custom-tool"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "custom-skills")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results  []syncTargetResult `json:"results"`
		Warnings []string           `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 || len(resp.Warnings) != 0 {
		t.Fatalf("expected silent skip, got results %+v warnings %v", resp.Results, resp.Warnings)
	}
}

func TestHandleSync_MergesAgentsIntoSkillsRow(t *testing.T) {
	s, src, _ := newAgentSyncServer(t)
	addSkill(t, src, "alpha")
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-agents")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results []syncTargetResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || !slices.Equal(resp.Results[0].Linked, []string{"alpha", "tutor.md"}) {
		t.Fatalf("expected one claude row linking alpha and tutor.md, got %+v", resp.Results)
	}
}

func TestHandleSync_ProjectSyncsOnlyThatProjectsSkills(t *testing.T) {
	global := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": global})
	addSkill(t, src, "team-a")
	root := filepath.Join(filepath.Dir(src), "app")
	addProject(t, s, root)

	rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"project":"`+root+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "team-a")); err != nil {
		t.Fatalf("project target not synced: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(global, "team-a")); !os.IsNotExist(err) {
		t.Fatalf("global target was synced, err=%v", err)
	}
}

func TestHandleSync_ProjectSyncsOnlyThatProjectsAgents(t *testing.T) {
	s, src := newTestServer(t)
	agentSource := filepath.Join(t.TempDir(), "agents")
	globalAgents := filepath.Join(t.TempDir(), "claude-agents")
	for _, dir := range []string{agentSource, globalAgents} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(agentSource, "tutor.md"), []byte("# Tutor"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: globalAgents},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(filepath.Dir(src), "app")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"create":true,"root":"` + root + `","targets":["claude"],"agents":{"mode":"merge","include":[],"exclude":[]}}`
	if rr := projectRequest(t, s, http.MethodPut, "/api/projects", body); rr.Code != http.StatusOK {
		t.Fatalf("add project: %d %s", rr.Code, rr.Body)
	}

	rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"kind":"agent","project":"`+root+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "agents", "tutor.md")); err != nil {
		t.Fatalf("project agent not synced: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(globalAgents, "tutor.md")); !os.IsNotExist(err) {
		t.Fatalf("global agent was synced, err=%v", err)
	}
}

func TestHandleSync_UnknownProjectIsRejected(t *testing.T) {
	s, src := newTestServer(t)
	rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"project":"`+filepath.Join(filepath.Dir(src), "nope")+`"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSync_ReportsPathOverlapCount(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "agents-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"universal": shared, "codex": shared})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		PathOverlap int `json:"path_overlap"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.PathOverlap != 2 {
		t.Errorf("path_overlap = %d, want 2", resp.PathOverlap)
	}
}

func TestHandleSync_ReportsFolderConflicts(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "agents-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"universal": shared, "codex": shared})
	addSkill(t, src, "alpha")
	s.cfg.Targets["universal"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: shared, Exclude: []string{"feature-radar*"}}}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	for _, tc := range []struct{ method, url, body string }{
		{http.MethodPost, "/api/sync", `{"dryRun":true}`},
		{http.MethodGet, "/api/diff", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", tc.url, rr.Code, rr.Body.String())
		}
		var resp struct {
			PathOverlap     int                           `json:"path_overlap"`
			FolderConflicts []config.SkillsFolderConflict `json:"folder_conflicts"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if len(resp.FolderConflicts) != 1 {
			t.Fatalf("%s: expected 1 folder conflict, got %+v", tc.url, resp.FolderConflicts)
		}
		c := resp.FolderConflicts[0]
		if c.Keep != "universal" || len(c.Stop) != 1 || c.Stop[0] != "codex" {
			t.Errorf("%s: conflict = %+v, want keep universal, stop codex", tc.url, c)
		}
		if resp.PathOverlap != 0 {
			t.Errorf("%s: overlap fully explained by the conflict, got path_overlap %d", tc.url, resp.PathOverlap)
		}
	}
}

func TestHandleSync_SkillTargetFailureKeepsSyncingOthers(t *testing.T) {
	s, src, _ := newAgentSyncServer(t)
	addSkill(t, src, "alpha")
	s.cfg.Targets["broken"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "broken-skills"), Include: []string{"["}},
	}
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-agents")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results  []syncTargetResult `json:"results"`
		Warnings []string           `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Target != "claude" || !slices.Equal(resp.Results[0].Linked, []string{"alpha", "tutor.md"}) {
		t.Fatalf("expected only the claude row with alpha and tutor.md, got %+v", resp.Results)
	}
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool {
		return strings.HasPrefix(w, "broken: sync failed: ")
	}) {
		t.Fatalf("expected sync failed warning for broken, got %v", resp.Warnings)
	}
	entries, err := oplog.Read(config.ConfigPath(), oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read ops log: %v (%d entries)", err, len(entries))
	}
	if e := entries[0]; e.Status != "partial" || e.Args["targets_failed"] != float64(1) {
		t.Fatalf("expected partial sync with targets_failed 1, got status %q args %v", e.Status, e.Args)
	}
}

func TestHandleSync_EverySkillTargetFailingIsReported(t *testing.T) {
	s, _ := newTestServer(t)
	file := filepath.Join(t.TempDir(), "skills-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.cfg.Targets["broken"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "broken-skills"), Include: []string{"["}},
	}
	s.cfg.Targets["invalid"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: file}}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Failed []syncFailure `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range resp.Failed {
		parts[f.Target] = f.Part
	}
	if len(resp.Failed) != 2 || parts["broken"] != "skill" || parts["invalid"] != "config" {
		t.Fatalf("expected broken (skill) and invalid (config) failed, got %+v", resp.Failed)
	}
	entries, err := oplog.Read(config.ConfigPath(), oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read ops log: %v (%d entries)", err, len(entries))
	}
	if e := entries[0]; e.Status != "error" || e.Args["targets_failed"] != float64(2) {
		t.Fatalf("expected error sync with targets_failed 2, got status %q args %v", e.Status, e.Args)
	}
}

func TestHandleSync_SkillPruneFailureIsWarning(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	tgtPath := filepath.Join(t.TempDir(), "copy-skills")
	// The manifest lives inside the orphan it lists, so pruning the orphan
	// leaves the manifest write nowhere to go.
	if err := os.MkdirAll(filepath.Join(tgtPath, "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tgtPath, "orphan", "manifest.json"), []byte(`{"managed":{"orphan":"x"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("orphan", "manifest.json"), filepath.Join(tgtPath, ssync.ManifestFile)); err != nil {
		t.Fatal(err)
	}
	s.cfg.Targets["copier"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: tgtPath, Mode: "copy"},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool {
		return strings.HasPrefix(w, "copier: prune failed: failed to write manifest: ")
	}) {
		t.Fatalf("expected prune failed warning, got %v", resp.Warnings)
	}
}

func TestHandleSync_SkillPruneWarningsAreReported(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(tgtPath, "outside")); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool {
		return strings.HasPrefix(w, "outside: symlink to external location")
	}) {
		t.Fatalf("expected external symlink prune warning, got %v", resp.Warnings)
	}
}

func TestHandleSync_UnmatchedIncludeIsReported(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	s.cfg.Targets["claude"] = config.TargetConfig{
		// The blank include is ignored, so "missing" is the only pattern and selects nothing.
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills"), Include: []string{"missing", ""}},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings  []string           `json:"warnings"`
		Unmatched []unmatchedInclude `json:"unmatched"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Unmatched) != 1 || resp.Unmatched[0].Target != "claude" || !slices.Equal(resp.Unmatched[0].Patterns, []string{"missing"}) || !resp.Unmatched[0].All {
		t.Fatalf("unmatched = %+v, want claude with missing and all", resp.Unmatched)
	}
	// The dashboard explains it from unmatched, so it is not repeated as a raw warning.
	if slices.ContainsFunc(resp.Warnings, func(w string) bool { return strings.Contains(w, `"missing"`) }) {
		t.Fatalf("warnings = %q, want the unmatched include left out", resp.Warnings)
	}
}

// newSymlinkConflictServer sets up a symlink-mode target whose path links to
// another folder, next to a merge target that syncs fine.
func newSymlinkConflictServer(t *testing.T) (s *Server, src, linkPath, elsewhere string) {
	t.Helper()
	s, src = newTestServer(t)
	addSkill(t, src, "alpha")
	elsewhere = t.TempDir()
	linkPath = filepath.Join(t.TempDir(), "linked-skills")
	if err := os.Symlink(elsewhere, linkPath); err != nil {
		t.Fatal(err)
	}
	s.cfg.Targets["linked"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: linkPath, Mode: "symlink"},
	}
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	return s, src, linkPath, elsewhere
}

func TestHandleSync_SymlinkConflictFailsTargetWithoutForce(t *testing.T) {
	s, _, linkPath, elsewhere := newSymlinkConflictServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results  []syncTargetResult `json:"results"`
		Warnings []string           `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	want := "linked: sync failed: conflict - symlink points to " + elsewhere + " (use --force to override)"
	if !slices.Contains(resp.Warnings, want) {
		t.Fatalf("expected %q, got %v", want, resp.Warnings)
	}
	if len(resp.Results) != 1 || resp.Results[0].Target != "claude" {
		t.Fatalf("expected only the claude row, got %+v", resp.Results)
	}
	if got, _ := os.Readlink(linkPath); got != elsewhere {
		t.Fatalf("expected the conflicting symlink kept, points to %q", got)
	}
}

func TestHandleSync_ReportsFailedTargetsApartFromWarnings(t *testing.T) {
	s, _, _, elsewhere := newSymlinkConflictServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	var resp struct {
		Failed []syncFailure `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	errText := "conflict - symlink points to " + elsewhere + " (use --force to override)"
	want := []syncFailure{{Target: "linked", Part: "skill", Error: errText, Message: "linked: sync failed: " + errText, Conflict: true}}
	if !slices.Equal(resp.Failed, want) {
		t.Fatalf("expected %+v, got %+v", want, resp.Failed)
	}
}

func TestHandleSync_SymlinkConflictReplacedWithForce(t *testing.T) {
	s, src, linkPath, _ := newSymlinkConflictServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"force":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results []syncTargetResult `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected rows for both targets, got %+v", resp.Results)
	}
	if ssync.CheckStatus(linkPath, src) != ssync.StatusLinked {
		t.Fatalf("expected %s linked to the source after force", linkPath)
	}
}

func TestHandleSync_InvalidTargetConfigFailsOnlyThatTarget(t *testing.T) {
	s, src, _ := newAgentSyncServer(t)
	addSkill(t, src, "alpha")
	file := filepath.Join(t.TempDir(), "skills-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	brokenAgents := filepath.Join(t.TempDir(), "broken-agents")
	s.cfg.Targets["broken"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: file},
		Agents: &config.ResourceTargetConfig{Path: brokenAgents},
	}
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-agents")},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results  []syncTargetResult `json:"results"`
		Warnings []string           `json:"warnings"`
		Failed   []syncFailure      `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Target != "claude" || !slices.Equal(resp.Results[0].Linked, []string{"alpha", "tutor.md"}) {
		t.Fatalf("expected only the claude row with alpha and tutor.md, got %+v", resp.Results)
	}
	if len(resp.Failed) != 1 || resp.Failed[0].Target != "broken" || resp.Failed[0].Part != "config" ||
		!strings.Contains(resp.Failed[0].Error, "path is not a directory") || !slices.Contains(resp.Warnings, resp.Failed[0].Message) {
		t.Fatalf("expected one config failure for broken also in warnings, got failed %+v warnings %v", resp.Failed, resp.Warnings)
	}
	if _, err := os.Stat(brokenAgents); !os.IsNotExist(err) {
		t.Fatalf("expected no agent sync into the invalid target, stat err = %v", err)
	}
	entries, err := oplog.Read(config.ConfigPath(), oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read ops log: %v (%d entries)", err, len(entries))
	}
	got, _ := json.Marshal(entries[0].Args["failed_targets"])
	if e := entries[0]; e.Status != "partial" || e.Args["targets_failed"] != float64(1) || string(got) != `["broken"]` {
		t.Fatalf("expected partial sync with broken failed, got status %q args %v", e.Status, e.Args)
	}
}

func TestHandleSync_ProjectTargetWithoutPathFailsAlone(t *testing.T) {
	projectRoot := t.TempDir()
	src := filepath.Join(projectRoot, ".skillshare", "skills")
	addSkill(t, src, "alpha")
	projectCfg := &config.ProjectConfig{Targets: []config.ProjectTargetEntry{{Name: "claude"}, {Name: "custom"}}}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatal(err)
	}
	s := NewProject(&config.Config{Source: src, Mode: "merge"}, projectCfg, projectRoot, "127.0.0.1:0", "", "")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Results []syncTargetResult `json:"results"`
		Failed  []syncFailure      `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Target != "claude" {
		t.Fatalf("expected claude to sync, got %+v", resp.Results)
	}
	if len(resp.Failed) != 1 || resp.Failed[0].Target != "custom" || resp.Failed[0].Part != "config" || !strings.Contains(resp.Failed[0].Error, "missing path") {
		t.Fatalf("expected one config failure for custom, got %+v", resp.Failed)
	}
}

// TestHandleSync_MalformedMetadataIsWarnedNotOverwritten verifies that sync
// refuses to adopt moved records when .metadata.json does not load, so the
// empty stand-in store is never saved over it.
func TestHandleSync_MalformedMetadataIsWarnedNotOverwritten(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")
	metaPath := filepath.Join(src, install.MetadataFileName)
	if err := os.WriteFile(metaPath, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings []string `json:"warnings"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !slices.ContainsFunc(resp.Warnings, func(w string) bool { return strings.HasPrefix(w, "install metadata not updated") }) {
		t.Errorf("warnings = %v, want the metadata load failure", resp.Warnings)
	}
	if data, _ := os.ReadFile(metaPath); string(data) != "{not json" {
		t.Errorf("metadata file = %q, want it left untouched", data)
	}
}
