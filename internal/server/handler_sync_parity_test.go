package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
)

func TestHandleSync_KindParity(t *testing.T) {
	for _, kind := range []string{"skill", "agent"} {
		t.Run(kind, func(t *testing.T) {
			s, src, agents := newAgentSyncServer(t)
			addSkill(t, src, "alpha")
			skills := filepath.Join(t.TempDir(), "skills")
			agentTarget := filepath.Join(t.TempDir(), "agents")
			s.cfg.AgentsSource = agents
			s.cfg.Targets = map[string]config.TargetConfig{"custom": {Skills: &config.ResourceTargetConfig{Path: skills}, Agents: &config.ResourceTargetConfig{Path: agentTarget}}}
			if err := s.cfg.Save(); err != nil {
				t.Fatal(err)
			}
			rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"kind":"`+kind+`"}`)
			if rr.Code != http.StatusOK {
				t.Fatalf("sync: %d %s", rr.Code, rr.Body)
			}
			for resource, path := range map[string]string{"skill": filepath.Join(skills, "alpha"), "agent": filepath.Join(agentTarget, "tutor.md")} {
				_, err := os.Lstat(path)
				if resource == kind && err != nil {
					t.Fatalf("selected %s missing: %v", resource, err)
				}
				if resource != kind && !os.IsNotExist(err) {
					t.Fatalf("unselected %s changed: %v", resource, err)
				}
			}
		})
	}
}

func TestHandleSync_ProjectModeParity(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, ".skillshare", "skills")
	addSkill(t, src, "alpha")
	projectCfg := &config.ProjectConfig{Targets: []config.ProjectTargetEntry{{Name: "claude"}}}
	if err := projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	s := NewProject(&config.Config{Source: src, Mode: "merge"}, projectCfg, root, "127.0.0.1:0", "", "")
	rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rr.Code, rr.Body)
	}
	path := filepath.Join(root, ".claude", "skills", "alpha")
	link, err := os.Readlink(path)
	if err != nil || filepath.IsAbs(link) {
		t.Fatalf("project link = %q, %v; want relative link", link, err)
	}
}

func TestHandleSync_SelectedProjectKeepsDiagnosticsAndIgnoreEnvelope(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(fmt.Sprint(conflicting), func(t *testing.T) {
			shared := filepath.Join(t.TempDir(), "skills")
			s, src := newTestServerWithTargets(t, map[string]string{"codex": shared, "universal": shared})
			addSkill(t, src, "alpha")
			addSkill(t, src, "ignored")
			if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("ignored\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if conflicting {
				s.cfg.Targets["universal"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: shared, Exclude: []string{"missing*"}}}
				if err := s.cfg.Save(); err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Join(filepath.Dir(src), "app")
			addProject(t, s, root)
			rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"dryRun":true,"project":"`+root+`"}`)
			if rr.Code != http.StatusOK {
				t.Fatalf("sync: %d %s", rr.Code, rr.Body)
			}
			var got struct {
				Results         []syncTargetResult            `json:"results"`
				FolderConflicts []config.SkillsFolderConflict `json:"folder_conflicts"`
				PathOverlap     int                           `json:"path_overlap"`
				IgnoredCount    int                           `json:"ignored_count"`
				IgnoredSkills   []string                      `json:"ignored_skills"`
				IgnoreRoot      string                        `json:"ignore_root"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			wantConflicts, wantOverlap := 0, 2
			if conflicting {
				wantConflicts, wantOverlap = 1, 0
			}
			if len(got.Results) != 2 || !slices.ContainsFunc(got.Results, func(row syncTargetResult) bool { return row.Target == "app@claude" }) || !slices.ContainsFunc(got.Results, func(row syncTargetResult) bool { return row.Target == "app@codex" }) || len(got.FolderConflicts) != wantConflicts || got.PathOverlap != wantOverlap || got.IgnoredCount != 1 || !slices.Equal(got.IgnoredSkills, []string{"ignored"}) || got.IgnoreRoot != filepath.Join(src, ".skillignore") {
				t.Fatalf("selected sync lost whole-config diagnostics or ignore fields: %+v", got)
			}
		})
	}
}

func TestHandleSync_AgentIgnoreEnvelope(t *testing.T) {
	// Run as a transform extension after discovery, before the response is built.
	if ignored := os.Getenv("SKILLSHARE_TEST_REMOVE_IGNORED_AGENT"); ignored != "" {
		if err := os.Remove(ignored); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": filepath.Join(t.TempDir(), "skills")})
	agents := t.TempDir()
	s.cfg.AgentsSource = agents
	for _, name := range []string{"keep.md", "ignored.md"} {
		if err := os.WriteFile(filepath.Join(agents, name), []byte("# Agent\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ignore := filepath.Join(agents, ".agentignore")
	if err := os.WriteFile(ignore, []byte("ignored.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	extension := t.TempDir()
	manifest, err := json.Marshal(map[string]any{"run": []string{exe, "-test.run=^TestHandleSync_AgentIgnoreEnvelope$"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extension, "extension.yaml"), manifest, 0644); err != nil {
		t.Fatal(err)
	}
	target := s.cfg.Targets["claude"]
	target.Agents = &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "agents"), Mode: "copy", Extension: extension}
	s.cfg.Targets["claude"] = target
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	ignored := filepath.Join(agents, "ignored.md")
	t.Setenv("SKILLSHARE_TEST_REMOVE_IGNORED_AGENT", ignored)
	rr := projectRequest(t, s, http.MethodPost, "/api/sync", `{"kind":"agent"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rr.Code, rr.Body)
	}
	if _, err := os.Stat(ignored); !os.IsNotExist(err) {
		t.Fatalf("extension did not remove ignored agent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target.Agents.Path, "keep.md")); err != nil {
		t.Fatalf("agent sync did not finish: %v", err)
	}
	var got struct {
		Root    string   `json:"agent_ignore_root"`
		Count   int      `json:"agent_ignored_count"`
		Ignored []string `json:"agent_ignored_skills"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || !slices.Equal(got.Ignored, []string{"ignored.md"}) {
		t.Fatalf("agent ignore envelope = %+v", got)
	}
	// The root is the resolved path, which can spell a temp dir differently.
	want, err := os.Stat(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if root, err := os.Stat(got.Root); err != nil || !os.SameFile(want, root) {
		t.Fatalf("agent_ignore_root = %q: %v", got.Root, err)
	}
}
