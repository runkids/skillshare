//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/profile"
)

func TestProfileAssign(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			f.run("profile", "use", "a").AssertSuccess(t)
			statePath, _, err := config.ProfilePaths(f.config)
			if err != nil {
				t.Fatal(err)
			}
			stateBefore := f.sb.ReadFile(statePath)
			out := f.run("profile", "add", "a", "beta", "--json")
			out.AssertSuccess(t)
			var row struct {
				Profile string               `json:"profile"`
				Changed bool                 `json:"changed"`
				Skills  []profile.SkillResult `json:"skills"`
			}
			if err := json.Unmarshal([]byte(out.Stdout), &row); err != nil || row.Profile != "a" || !row.Changed || len(row.Skills) != 1 || row.Skills[0] != (profile.SkillResult{Name: "beta", Result: "added"}) {
				t.Fatalf("membership JSON: %s (%v)", out.Stdout, err)
			}
			if _, err := os.Lstat(filepath.Join(f.target, "beta")); !os.IsNotExist(err) {
				t.Fatal("add auto-applied targets", err)
			}
			removed := f.run("profile", "remove", "a", "alpha")
			removed.AssertSuccess(t)
			removed.AssertOutputContains(t, "removed")
			removed.AssertOutputContains(t, "skillshare sync")
			if _, err := os.Stat(filepath.Join(f.target, "alpha", "SKILL.md")); err != nil || f.sb.ReadFile(statePath) != stateBefore {
				t.Fatal("membership edit changed targets or local state", err)
			}
			f.sb.WriteFile(filepath.Join(f.source, "team", "review", "SKILL.md"), "# Review")
			f.sb.WriteFile(filepath.Join(f.source, "other", "review", "SKILL.md"), "# Other review")
			f.run("profile", "add", "b", "team/review").AssertSuccess(t)
			f.run("profile", "remove", "b", "team__review").AssertSuccess(t)
			f.run("profile", "edit", "b", "--add-exclude", "a*").AssertSuccess(t)
			before := f.sb.ReadFile(f.config)
			for _, args := range [][]string{{"beta", "alpha"}, {"review"}, {"beta", "missing"}} {
				result := f.run(append([]string{"profile", "add", "b"}, args...)...)
				result.AssertFailure(t)
				if f.sb.ReadFile(f.config) != before || f.sb.ReadFile(statePath) != stateBefore {
					t.Fatal("refused batch changed config/state")
				}
				if args[len(args)-1] == "alpha" {
					result.AssertOutputContains(t, "a*")
				}
			}
			logPath := filepath.Join(oplog.LogDir(f.config), oplog.OpsFile)
			if !strings.Contains(f.sb.ReadFile(logPath), "profile definitions") {
				t.Fatal("membership edit missing operation log")
			}
		})
	}
}

func TestProfileAssignSelectionFixtures(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, tc := range []struct {
			name, definition, op, skill, result string
			blocked                            bool
		}{
			{"pass add", "{}", "add", "alpha", "already-in", false},
			{"pass remove", "{}", "remove", "alpha", "removed", false},
			{"last include", "{skills: {include: [alpha], exclude: [beta, 'z-*']}}", "remove", "alpha", "removed", false},
			{"empty blocked", "{skills: {include: [], exclude: ['a*']}}", "add", "alpha", "a*", true},
			{"empty mapping", "{skills: {}}", "add", "alpha", "added", false},
			{"null skills", "{skills: null}", "add", "alpha", "added", false},
			{"blank include", "{skills: {include: ['', ' ']}}", "add", "alpha", "added", false},
			{"exact exclude", "{skills: {include: ['*'], exclude: [alpha]}}", "add", "alpha", "added", false},
		} {
			t.Run(fmt.Sprintf("%v/%s", project, tc.name), func(t *testing.T) {
				f := newProfileFixture(t, project, "copy")
				f.sb.WriteFile(f.config, f.sb.ReadFile(f.config)+"  fixture: "+tc.definition+"\n")
				before := f.sb.ReadFile(f.config)
				out := f.run("profile", tc.op, "fixture", tc.skill, "--json")
				if tc.blocked {
					out.AssertFailure(t)
					out.AssertOutputContains(t, tc.result)
					if f.sb.ReadFile(f.config) != before {
						t.Fatal("blocked add changed definition")
					}
					return
				}
				out.AssertSuccess(t)
				out.AssertOutputContains(t, tc.result)
				show := f.run("profile", "show", "fixture", "--json")
				show.AssertSuccess(t)
				var view struct {
					Profiles []struct{ Selected []string }
				}
				if err := json.Unmarshal([]byte(show.Stdout), &view); err != nil || len(view.Profiles) != 1 {
					t.Fatal(show.Stdout, err)
				}
				selected := false
				for _, name := range view.Profiles[0].Selected {
					selected = selected || name == tc.skill
				}
				if selected != (tc.op == "add") {
					t.Fatal("membership disagrees with show", show.Stdout)
				}
				if tc.result == "already-in" && f.sb.ReadFile(f.config) != before {
					t.Fatal("no-op rewrote config")
				}
			})
		}
	}
}
