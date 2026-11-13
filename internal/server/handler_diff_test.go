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
	if scenario == "pending_none" {
		if _, err := profileapply.Apply(context.Background(), profileapply.Request{Scope: scope, None: true, NoSync: true}); err != nil {
			t.Fatal(err)
		}
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
	}
	if err := s.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	return s, scope, dest, source
}

func TestProfiles_APIDiffPreservationAgreement(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			scenarios := []string{"retained", "departing", "pending_none", "unmanaged", "no_profile", "old_name"}
			if mode == "copy" {
				scenarios = append(scenarios, "equal_mtime")
			}
			for _, scenario := range scenarios {
				t.Run(fmt.Sprintf("%v/%s/%s", project, mode, scenario), func(t *testing.T) {
					s, scope, dest, source := newProfileDiffFixture(t, project, mode, scenario)
					if scenario == "equal_mtime" {
						if _, err := sync.SyncTargetCopy("custom", config.TargetConfig{Path: dest}, source, false, true); err != nil {
							t.Fatal(err)
						}
						file := filepath.Join(source, "alpha", "SKILL.md")
						info, err := os.Stat(file)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file, []byte("replacement alpha"), 0600); err != nil {
							t.Fatal(err)
						}
						if err := os.Chtimes(file, info.ModTime(), info.ModTime()); err != nil {
							t.Fatal(err)
						}
						manifest, manifestErr := sync.ReadManifest(dest)
						stamp, stampErr := sync.DirMaxMtimeWithIgnore(filepath.Dir(file), sync.DefaultFileIgnorePatterns())
						if manifestErr != nil || stampErr != nil || stamp != manifest.Mtimes["alpha"] || !strings.HasPrefix(manifest.Outputs["alpha"], "sha256-v2:") {
							t.Fatalf("equal-mtime current-output precondition: %+v %d %v %v", manifest, stamp, manifestErr, stampErr)
						}
					}
					tree := func() string {
						files := map[string]string{}
						for _, root := range []string{source, dest, filepath.Dir(scope.Path()), os.Getenv("HOME")} {
							filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
								if err != nil {
									t.Fatal(err)
								}
								data, err := os.ReadFile(path)
								if err != nil && !info.IsDir() {
									t.Fatal(err)
								}
								files[path] = info.Mode().String() + string(data)
								return nil
							})
						}
						return fmt.Sprint(files)
					}
					treeBefore := tree()
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
									if scenario == "equal_mtime" {
										want = "update"
									}
									if scenario == "equal_mtime" && item.Reason != "content changed" {
										t.Errorf("%s did not predict changed selected bytes: %+v", endpoint, item)
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
					if tree() != treeBefore {
						t.Error("previews changed file paths, modes, source, target or state bytes")
					}
					rr := httptest.NewRecorder()
					s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader("{\"kind\":\"skill\"}")))
					if rr.Code != http.StatusOK {
						t.Fatal(rr.Code, rr.Body.String())
					}
					data, err := os.ReadFile(filepath.Join(dest, "alpha", "SKILL.md"))
					wantBytes := "authored alpha"
					if scenario == "equal_mtime" {
						wantBytes = "replacement alpha"
					}
					if err != nil || string(data) != wantBytes {
						t.Fatalf("actual sync lost previewed edit: %q %v", data, err)
					}
				})
			}
		}
	}
}
