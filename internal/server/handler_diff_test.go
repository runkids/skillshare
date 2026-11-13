package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

func TestHandleDiff_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Diffs []any `json:"diffs"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(resp.Diffs))
	}
}

func TestHandleDiff_WithTarget(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Diffs []map[string]any `json:"diffs"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Diffs) != 1 {
		t.Fatalf("expected 1 diff target, got %d", len(resp.Diffs))
	}
}

func TestHandleDiff_AgentPruneWhenSourceEmpty(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "claude-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	if err := os.MkdirAll(agentTarget, 0o755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}
	if err := os.Symlink(filepath.Join(agentSource, "tutor.md"), filepath.Join(agentTarget, "tutor.md")); err != nil {
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

	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Diffs []struct {
			Target string `json:"target"`
			Items  []struct {
				Skill  string `json:"skill"`
				Action string `json:"action"`
				Kind   string `json:"kind"`
			} `json:"items"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal diff response: %v", err)
	}

	if len(resp.Diffs) != 1 {
		t.Fatalf("expected 1 diff target, got %d", len(resp.Diffs))
	}
	if resp.Diffs[0].Target != "claude" {
		t.Fatalf("expected claude target, got %q", resp.Diffs[0].Target)
	}
	if len(resp.Diffs[0].Items) != 1 {
		t.Fatalf("expected 1 diff item, got %d", len(resp.Diffs[0].Items))
	}
	item := resp.Diffs[0].Items[0]
	if item.Skill != "tutor.md" || item.Action != "prune" || item.Kind != "agent" {
		t.Fatalf("unexpected diff item: %+v", item)
	}
}

// Refs #391: the Sync tab listed every converted extension agent as pending.
func TestHandleDiff_AgentExtensionOutputsAreNotPending(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "codex-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	srcFile := filepath.Join(agentSource, "reviewer.md")
	if err := os.WriteFile(srcFile, []byte("# Reviewer"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	agents := []resource.DiscoveredResource{{FlatName: "reviewer.md", AbsPath: srcFile, RelPath: "reviewer.md"}}
	spec := &sync.ExtensionSpec{Run: []string{"cat"}, Dir: agentSource, Name: "id", OutputExt: "toml"}
	if _, err := sync.SyncAgentsTransform(agents, agentSource, agentTarget, "copy", spec, false, false); err != nil {
		t.Fatalf("seed converted output: %v", err)
	}

	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["codex"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "codex-skills")},
		Agents: &config.ResourceTargetConfig{Path: agentTarget, Extension: "id"},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "reviewer") {
		t.Fatalf("converted agent should not be pending: %s", rr.Body.String())
	}
}

// Shared by diff transport and sync tests; every expected edit is authored here.
func newProfileDiffFixture(t *testing.T, project bool, mode, scenario string) (*Server, profileapply.Scope, string, string) {
	t.Helper()
	home := t.TempDir()
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_STATE_HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME"} {
		t.Setenv(key, home)
	}
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	dest := t.TempDir()
	var s *Server
	var source, path, text string
	scope := profileapply.Scope{}
	if project {
		var root string
		s, root = newProjectTargetServer(t, []config.ProjectTargetEntry{{Name: "custom", Skills: &config.ResourceTargetConfig{Path: dest, Mode: "copy"}}})
		source, path = filepath.Join(root, ".skillshare", "skills"), config.ProjectConfigPath(root)
		scope.ProjectRoot = root
		text = "targets:\n  - name: custom\n    skills:\n      path: " + dest + "\n      mode: copy\n"
	} else {
		s, source = newTestServerWithTargets(t, map[string]string{"custom": dest})
		path = config.ConfigPath()
		text = "source: " + source + "\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: " + dest + "\n"
	}
	scope.ConfigPath = path
	if scenario == "old_name" {
		addTrackedRepo(t, source, "_emil-design")
		addSkillNested(t, source, "_emil-design/skills/alpha")
		text = strings.Replace(text, "path: "+dest, "path: "+dest+"\n      target_naming: standard", 1)
	} else {
		addSkill(t, source, "alpha")
	}
	addSkill(t, source, "beta")
	text += "profiles:\n  all: {}\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if scenario == "no_profile" {
		if _, err := sync.SyncTargetCopy("custom", config.TargetConfig{Path: dest}, source, false, false); err != nil {
			t.Fatal(err)
		}
	} else if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, Name: "all", NoSync: scenario == "unmanaged"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "alpha"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "alpha", "SKILL.md"), []byte("authored alpha"), 0600); err != nil {
		t.Fatal(err)
	}
	text = strings.ReplaceAll(text, "mode: copy", "mode: "+mode)
	if scenario == "old_name" {
		text = strings.Replace(text, "target_naming: standard", "target_naming: prefixed", 1)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if scenario == "departing" {
		if _, err := profileapply.Mutate(context.Background(), scope, profile.Mutation{Op: "edit", Name: "all", AddInclude: []string{"beta"}}); err != nil {
			t.Fatal(err)
		}
	} else if scenario == "pending_none" {
		if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, None: true, NoSync: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	return s, scope, dest, source
}

func TestProfiles_APIDiffPreservationAgreement(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			for _, scenario := range []string{"retained", "departing", "pending_none", "unmanaged", "no_profile", "old_name"} {
				t.Run(fmt.Sprintf("%v/%s/%s", project, mode, scenario), func(t *testing.T) {
					s, scope, dest, _ := newProfileDiffFixture(t, project, mode, scenario)
					before, err := os.ReadFile(scope.Path())
					if err != nil {
						t.Fatal(err)
					}
					manifestPath := filepath.Join(dest, sync.ManifestFile)
					manifestBefore, _ := os.ReadFile(manifestPath)
					for _, endpoint := range []string{"/api/diff", "/api/diff/stream"} {
						rr := httptest.NewRecorder()
						s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, endpoint, nil))
						if rr.Code != http.StatusOK {
							t.Fatalf("preview %s: %d %s", endpoint, rr.Code, rr.Body.String())
						}
						var payload struct {
							Diffs []diffTarget
						}
						body := rr.Body.String()
						if endpoint == "/api/diff/stream" {
							for _, event := range strings.Split(body, "\n\n") {
								if strings.Contains(event, "event: done\n") {
									body = strings.SplitN(event, "data: ", 2)[1]
								}
							}
						}
						if err := json.Unmarshal([]byte(body), &payload); err != nil {
							t.Fatal(err, body)
						}
						found := false
						for _, target := range payload.Diffs {
							for _, item := range target.Items {
								if item.Skill != "alpha" && !(scenario == "old_name" && mode == "copy" && item.Skill == "emil-design-alpha") {
									continue
								}
								found = true
								if scenario != "no_profile" {
									want := "skip"
									if scenario == "departing" || scenario == "old_name" && mode == "merge" {
										want = "kept"
									}
									if item.Action != want || item.Reason == "" {
										t.Errorf("%s predicts write/hides kept alpha: %+v", endpoint, item)
									}
									if scenario == "unmanaged" && strings.Contains(item.Reason, "discard") {
										t.Error("unmanaged recovery suggests discard")
									}
								} else if item.Action == "kept" {
									t.Error("profile preservation changed non-profile preview")
								}
							}
						}
						if scenario != "no_profile" && !found {
							t.Errorf("%s omitted preserved alpha", endpoint)
						}
					}
					after, err := os.ReadFile(scope.Path())
					if err != nil || string(after) != string(before) {
						t.Fatal("preview changed config", err)
					}
					manifestAfter, _ := os.ReadFile(manifestPath)
					if string(manifestAfter) != string(manifestBefore) {
						t.Fatal("preview changed ownership records")
					}
					rr := httptest.NewRecorder()
					s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader("{\"kind\":\"skill\"}")))
					if rr.Code != http.StatusOK {
						t.Fatal(rr.Code, rr.Body.String())
					}
					data, err := os.ReadFile(filepath.Join(dest, "alpha", "SKILL.md"))
					if err != nil || string(data) != "authored alpha" {
						t.Fatalf("actual sync lost previewed edit: %q %v", data, err)
					}
				})
			}
		}
	}
}
