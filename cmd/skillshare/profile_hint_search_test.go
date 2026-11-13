package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/profile"
	"skillshare/internal/search"
)

func TestSearchInstallProfileHints(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, scenario := range []string{"success", "skip", "failure", "none", "unavailable", "empty-profile"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, scenario), func(t *testing.T) {
				active, include := "work", "[keep]"
				if scenario == "none" {
					active = ""
				} else if scenario == "unavailable" {
					active = "missing"
				} else if scenario == "empty-profile" {
					include = "[]"
				}
				cfg, _, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: "+include+"\n", profile.State{Version: 1, Active: active})
				if project {
					t.Chdir(root)
					global := filepath.Join(root, "global.yaml")
					if err := os.WriteFile(global, []byte("source: "+t.TempDir()+"\ntargets: {}\nprofiles:\n  work: {}\n"), 0600); err != nil {
						t.Fatal(err)
					}
					t.Setenv("SKILLSHARE_CONFIG", global)
				}
				origin := writeHintSkill(t, t.TempDir(), "origin")
				writeHintSkill(t, origin, "nested/review")
				result := search.SearchResult{Name: "published", Source: origin}
				if scenario == "skip" {
					writeHintSkill(t, cfg.Source, "published")
				} else if scenario == "failure" {
					result.Source = filepath.Join(root, "absent")
				}
				var commandErr error
				output := captureStdout(t, func() {
					if project {
						commandErr = installFromSearchResultProject(result, root)
					} else {
						commandErr = installFromSearchResult(result, cfg)
					}
				})
				output = stripANSIWarnings(output)
				if (commandErr != nil) != (scenario == "failure") {
					t.Fatalf("unexpected install result: %v: %s", commandErr, output)
				}
				wantHint := scenario == "success" || scenario == "empty-profile"
				if count := strings.Count(output, `Not in active profile "work"`); (wantHint && count != 1) || (!wantHint && count != 0) {
					t.Fatalf("hint count = %d: %s", count, output)
				}
				if wantHint && !strings.Contains(output, "skillshare profile add work published published__nested__review") {
					t.Fatal("hint must use final recursive names", output)
				}
				mode := "-g"
				if project {
					mode = "-p"
				}
				if wantHint && !strings.Contains(output, "published__nested__review "+mode+"\n") {
					t.Fatal("single search hint lost originating scope", output)
				}
				_, err := os.Stat(filepath.Join(cfg.Source, "published", "SKILL.md"))
				if (err == nil) != (scenario != "failure") {
					t.Fatal("publication outcome changed", err)
				}
			})
		}
	}
}

func TestSearchBatchProfileHints(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, scenario := range []string{"success", "mixed", "skip", "failure", "none", "empty", "duplicate-grouped", "duplicate-local"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, scenario), func(t *testing.T) {
				duplicate := strings.HasPrefix(scenario, "duplicate-")
				active := "work"
				if scenario == "none" {
					active = ""
				}
				cfg, _, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: active})
				mode := modeGlobal
				if project {
					t.Chdir(root)
					mode = modeProject
					global := filepath.Join(root, "global.yaml")
					if err := os.WriteFile(global, []byte("source: "+t.TempDir()+"\ntargets: {}\nprofiles:\n  work: {}\n"), 0600); err != nil {
						t.Fatal(err)
					}
					t.Setenv("SKILLSHARE_CONFIG", global)
				}
				origin := writeHintSkill(t, t.TempDir(), "origin")
				writeHintSkill(t, origin, "nested/review")
				var selected []search.SearchResult
				published := scenario == "success" || scenario == "mixed" || scenario == "none" || duplicate
				if published {
					repo := t.TempDir()
					writeHintSkill(t, repo, "skills/a")
					writeHintSkill(t, repo, "skills/b")
					if scenario == "duplicate-grouped" {
						writeHintSkill(t, repo, "extras/a")
						writeHintSkill(t, repo, "extras/a/skipped-only")
					}
					initGitRepoForTest(t, repo)
					for _, name := range []string{"a", "b"} {
						selected = append(selected, search.SearchResult{Name: name, Source: "file://" + filepath.ToSlash(repo) + "//skills/" + name})
					}
					if scenario == "duplicate-grouped" {
						selected = append(selected, search.SearchResult{Name: "a", Source: "file://" + filepath.ToSlash(repo) + "//extras/a"})
					}
					selected = append(selected, search.SearchResult{Name: "published", Source: origin})
				}
				if scenario == "duplicate-local" {
					skipped := writeHintSkill(t, t.TempDir(), "origin")
					writeHintSkill(t, skipped, "skipped-only")
					selected = append(selected, search.SearchResult{Name: "published", Source: skipped})
				}
				if scenario == "mixed" || scenario == "skip" || duplicate {
					selected = append(selected, search.SearchResult{Name: "edit", Source: origin})
				}
				if scenario == "mixed" || scenario == "failure" || duplicate {
					selected = append(selected, search.SearchResult{Name: "broken", Source: filepath.Join(root, "absent")})
				}
				var commandErr error
				output := captureStdout(t, func() { commandErr = batchInstallFromSearchWithProgress(selected, mode, root) })
				output = stripANSIWarnings(output)
				if (commandErr != nil) != (scenario == "mixed" || scenario == "failure" || duplicate) {
					t.Fatalf("unexpected batch result: %v: %s", commandErr, output)
				}
				for _, name := range []string{"a", "b", "published", "published/nested/review"} {
					_, err := os.Stat(filepath.Join(cfg.Source, filepath.FromSlash(name), "SKILL.md"))
					if (err == nil) != published {
						t.Fatalf("publication %s changed: %v", name, err)
					}
				}
				if _, err := os.Stat(filepath.Join(cfg.Source, "broken")); !os.IsNotExist(err) {
					t.Fatal("failed result published a root", err)
				}
				for _, name := range []string{"a/skipped-only", "published/skipped-only"} {
					if _, err := os.Stat(filepath.Join(cfg.Source, filepath.FromSlash(name))); !os.IsNotExist(err) {
						t.Fatal("skipped content was published", name, err)
					}
				}
				wantHint := scenario == "success" || scenario == "mixed" || duplicate
				if count := strings.Count(output, `Not in active profile "work"`); (wantHint && count != 1) || (!wantHint && count != 0) {
					t.Fatalf("batch hint count = %d: %s", count, output)
				}
				scope := "-g"
				if project {
					scope = "-p"
				}
				if wantHint && !strings.Contains(output, "published__nested__review "+scope+"\n") {
					t.Fatal("batch search hint lost originating scope", output)
				}
				if wantHint && !strings.Contains(output, "skillshare profile add work a b published published__nested__review\n") {
					t.Fatal("batch hint must name only successful final skills once", output)
				}
			})
		}
	}
}
