package server

import (
	"encoding/json"
	"fmt"
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
