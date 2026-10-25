package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"skillshare/internal/check/checktest"
	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestURLBranchKey_RoundTrip(t *testing.T) {
	tests := []struct {
		url    string
		branch string
	}{
		{"https://github.com/owner/repo", "develop"},
		{"https://github.com/owner/repo", ""},
		{"git@github.com:org/repo.git", ""},
		{"git@github.com:org/repo.git", "feature"},
		{"https://gitlab.com/group/subgroup/project.git", "main"},
		{"file:///tmp/repo.git", "dev"},
	}

	for _, tt := range tests {
		key := urlBranchKey(tt.url, tt.branch)
		gotURL, gotBranch := splitURLBranch(key)
		if gotURL != tt.url {
			t.Errorf("urlBranchKey(%q, %q) → splitURLBranch: URL = %q, want %q", tt.url, tt.branch, gotURL, tt.url)
		}
		if gotBranch != tt.branch {
			t.Errorf("urlBranchKey(%q, %q) → splitURLBranch: Branch = %q, want %q", tt.url, tt.branch, gotBranch, tt.branch)
		}
	}
}

func TestSplitURLBranch_SSHURLWithoutBranch(t *testing.T) {
	// Regression: "@" in SSH URLs must not be confused with the separator.
	url, branch := splitURLBranch("git@github.com:org/repo.git")
	if url != "git@github.com:org/repo.git" {
		t.Errorf("URL = %q, want full SSH URL", url)
	}
	if branch != "" {
		t.Errorf("Branch = %q, want empty", branch)
	}
}

func TestCheckFollowedCheckoutResolution(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			root := t.TempDir()
			testutil.SetIsolatedXDG(t, root)
			t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
			source := filepath.Join(root, "skills")
			cfg := &config.Config{Source: source, FollowSourceLinks: true}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if project {
				projectCfg := &config.ProjectConfig{Sources: config.ProjectSources{Skills: source}, FollowSourceLinks: true}
				if err := projectCfg.Save(root); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(source, 0755); err != nil {
				t.Fatal(err)
			}
			remote := testutil.SetupBareRemoteRepo(t, root)
			testutil.SeedRemoteBranch(t, root, remote, "main", map[string]string{"SKILL.md": "# Safe skill\n"})
			checkout := filepath.Join(root, "checkout")
			testutil.RunGit(t, "", "clone", remote, checkout)
			if err := os.Symlink(checkout, filepath.Join(source, "_dev-skills")); err != nil {
				t.Fatal(err)
			}
			for _, names := range [][]string{nil, {"_dev-skills"}} {
				output := captureStdoutStderr(t, func() {
					var err error
					if project {
						err = cmdCheckProject(root, &checkOptions{names: names, json: true})
					} else {
						err = cmdCheck(append([]string{"-g", "--json"}, names...))
					}
					if err != nil {
						t.Errorf("check: %v", err)
					}
				})
				var result checkOutput
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatalf("invalid check output: %v: %s", err, output)
				}
				if len(result.TrackedRepos) != 1 || result.TrackedRepos[0].Name != "_dev-skills" || result.TrackedRepos[0].Status != "up_to_date" {
					t.Fatalf("check %v did not resolve followed checkout: %s", names, output)
				}
			}
		})
	}
}

// wantCLISkills is what the CLI reports for checktest.Seed.
func wantCLISkills(versions map[string]string, names ...string) []checkSkillResult {
	statuses := map[string]string{
		"broken":  "error",
		"changed": "update_available",
		"current": "up_to_date",
		"doomed":  "stale",
		"notree":  "update_available",
		"plain":   "local",
		"same":    "up_to_date",
		"slash":   "up_to_date",
	}
	if len(names) == 0 {
		for name := range statuses {
			names = append(names, name)
		}
	}
	var want []checkSkillResult
	for _, name := range names {
		r := checkSkillResult{Name: name, Source: "src/" + name, Version: versions[name], Status: statuses[name]}
		if name == "current" || name == "plain" {
			r.InstalledAt = "2024-05-06"
		}
		want = append(want, r)
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
	return want
}

func TestCheck_ResolvesEveryStatus(t *testing.T) {
	root := t.TempDir()
	testutil.SetIsolatedXDG(t, root)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	source := filepath.Join(root, "skills")
	if err := (&config.Config{Source: source}).Save(); err != nil {
		t.Fatal(err)
	}
	versions := checktest.Seed(t, root, source)

	runJSON := func(t *testing.T, args ...string) checkOutput {
		t.Helper()
		output := captureStdoutStderr(t, func() {
			err := cmdCheck(append([]string{"-g", "--json"}, args...))
			wantError := len(args) == 0 || (len(args) == 1 && args[0] == "broken")
			if (err != nil) != wantError {
				t.Errorf("check error = %v, want error: %t", err, wantError)
			}
		})
		var result checkOutput
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("invalid check output: %v: %s", err, output)
		}
		return result
	}

	t.Run("all", func(t *testing.T) {
		result := runJSON(t)
		if result.Skills[0].Name != "plain" {
			t.Errorf("skills without a remote must come first, got %+v", result.Skills[0])
		}
		sort.Slice(result.Skills, func(i, j int) bool { return result.Skills[i].Name < result.Skills[j].Name })
		if want := wantCLISkills(versions); !reflect.DeepEqual(result.Skills, want) {
			t.Errorf("skills:\n got %+v\nwant %+v", result.Skills, want)
		}
	})

	t.Run("filtered", func(t *testing.T) {
		result := runJSON(t, "doomed", "plain", "same", "current")
		if result.Skills[0].Name != "plain" {
			t.Errorf("skills without a remote must come first, got %+v", result.Skills[0])
		}
		sort.Slice(result.Skills, func(i, j int) bool { return result.Skills[i].Name < result.Skills[j].Name })
		if want := wantCLISkills(versions, "doomed", "plain", "same", "current"); !reflect.DeepEqual(result.Skills, want) {
			t.Errorf("skills:\n got %+v\nwant %+v", result.Skills, want)
		}
	})

	t.Run("failed filtered", func(t *testing.T) {
		result := runJSON(t, "broken")
		if len(result.Skills) != 1 || result.Skills[0].Status != "error" {
			t.Fatalf("expected failed skill, got %+v", result.Skills)
		}
	})

	t.Run("text", func(t *testing.T) {
		output := captureStdoutStderr(t, func() {
			if err := cmdCheck([]string{"-g"}); err == nil {
				t.Error("check must fail when a remote cannot be checked")
			}
		})
		const summary = "Updates available for 2 skills, 3 up to date, 1 local skipped, 1 stale, 1 failed to check"
		if !strings.Contains(output, summary) {
			t.Errorf("summary %q missing from:\n%s", summary, output)
		}
	})
}
