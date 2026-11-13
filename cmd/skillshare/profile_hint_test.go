package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/profile"
	"skillshare/internal/sourcewalk"
)

func writeHintSkill(t *testing.T, root, rel string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestProfileHintFinalNames(t *testing.T) {
	cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
	root := cfg.EffectiveSkillsSource()
	repo := writeHintSkill(t, root, "team/_repo")
	writeHintSkill(t, root, "team/_repo/skills/review")
	writeHintSkill(t, root, "team/_repo-other/review")
	writeHintSkill(t, root, "elsewhere/review")
	want := `Not in active profile "work": skillshare profile add work team___repo team___repo__skills__review`
	if got := installedProfileHint(cfg, root, cfg.SkillsWalk(), []string{repo, repo}); got != want {
		t.Fatalf("hint = %q, want %q", got, want)
	}
	for _, names := range [][]string{nil, {"keep"}} {
		if got := profileHint(cfg, names); got != "" {
			t.Fatal("selected/empty input must not hint", got)
		}
	}
	for _, active := range []string{"", "missing"} {
		other := cfg.WithActiveProfile(active)
		if got := profileHint(other, []string{"edit"}); got != "" {
			t.Fatal("none/unavailable must not hint", got)
		}
	}
	empty := filepath.Join(root, "_empty")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	if got := installedProfileHint(cfg, root, cfg.SkillsWalk(), []string{empty}); got != "" {
		t.Fatal("zero-skill publication must not hint", got)
	}
}

func TestProfileHintFollowedLogicalGroup(t *testing.T) {
	cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
	external := t.TempDir()
	writeHintSkill(t, external, "review")
	logical := filepath.Join(cfg.Source, "team")
	if err := os.Symlink(external, logical); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	walk := config.SkillsWalk(true, cfg.Source, cfg.Targets)
	if got := installedProfileHint(cfg, cfg.Source, walk, []string{logical}); !strings.HasSuffix(got, "work team__review") {
		t.Fatal("hint must retain logical group", got)
	}
	// An incomplete traversal must not turn the visible subset into a hint.
	if err := os.Symlink(filepath.Join(external, "absent"), filepath.Join(cfg.Source, "broken")); err != nil {
		t.Fatal(err)
	}
	if got := installedProfileHint(cfg, cfg.Source, config.SkillsWalk(true, cfg.Source, cfg.Targets), []string{logical}); got != "" {
		t.Fatal("incomplete discovery must omit hint", got)
	}
}

func TestInstallAndNewProfileHints(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, operation := range []string{"install", "dry", "failure", "agent", "new", "new-dry", "new-failure", "json"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, operation), func(t *testing.T) {
				cfg, projectCfg, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
				mode := "-g"
				if project {
					t.Chdir(root)
					mode = "-p"
					// Opposite global selector catches use of the synthetic/global config.
					global := filepath.Join(root, "global.yaml")
					if err := os.WriteFile(global, []byte("source: "+t.TempDir()+"\ntargets: {}\nprofiles:\n  work: {}\n"), 0600); err != nil {
						t.Fatal(err)
					}
					t.Setenv("SKILLSHARE_CONFIG", global)
				}
				if !project {
					// A global command must retain its scope when copied inside a project.
					path := config.ProjectConfigPath(root)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("sources:\n  skills: "+t.TempDir()+"\ntargets: []\nprofiles:\n  work: {skills: {include: []}}\n"), 0600); err != nil {
						t.Fatal(err)
					}
					t.Chdir(root)
				}
				origin := writeHintSkill(t, t.TempDir(), "review")
				writeHintSkill(t, origin, "nested/review")
				args := []string{origin, mode, "--into", "team", "--skip-audit"}
				wantHint := operation == "install"
				switch operation {
				case "dry":
					args = append(args, "--dry-run")
				case "failure":
					args[0] = filepath.Join(root, "absent")
				case "agent":
					repo := t.TempDir()
					if err := os.WriteFile(filepath.Join(repo, "reviewer.md"), []byte("---\nname: reviewer\ndescription: fixture\n---\n# Review\n"), 0600); err != nil {
						t.Fatal(err)
					}
					initGitRepoForTest(t, repo)
					args = []string{"file://" + filepath.ToSlash(repo), mode, "--track", "--name", "agents", "--kind", "agent", "--skip-audit"}
				case "json":
					args = append(args, "--json")
				}
				var commandErr error
				output := captureStdout(t, func() {
					if strings.HasPrefix(operation, "new") {
						args = []string{"fresh", mode, "-P", "none"}
						wantHint = operation == "new"
						if operation == "new-dry" {
							args = append(args, "--dry-run")
						} else if operation == "new-failure" {
							args[0] = "keep"
						}
						commandErr = cmdNew(args)
					} else {
						commandErr = cmdInstall(args)
					}
				})
				if operation == "failure" || operation == "new-failure" {
					if commandErr == nil {
						t.Fatal("expected failure", output)
					}
				} else if commandErr != nil {
					t.Fatal(commandErr, output)
				}
				if got := strings.Count(output, `Not in active profile "work"`); (wantHint && got != 1) || (!wantHint && got != 0) {
					t.Fatalf("hint count = %d: %s", got, output)
				}
				if operation == "install" && !strings.Contains(output, "work team__review team__review__nested__review") {
					t.Fatal("must name root and nested skill", output)
				}
				if operation == "json" {
					var out installJSONOutput
					if err := json.Unmarshal([]byte(output), &out); err != nil || strings.Join(out.Skills, ",") != "review" {
						t.Fatalf("public JSON changed: %s (%v)", output, err)
					}
				}
				if wantHint {
					name := "team/review"
					if operation == "new" {
						name = "fresh"
					}
					if _, err := os.Stat(filepath.Join(cfg.Source, filepath.FromSlash(name), "SKILL.md")); err != nil {
						t.Fatal("publication missing", err)
					}
				}
				if wantHint {
					for _, line := range strings.Split(stripANSIWarnings(output), "\n") {
						if i := strings.Index(line, "skillshare profile add "); i >= 0 {
							command := strings.TrimSpace(line[i:])
							if !strings.HasSuffix(command, " "+mode) {
								t.Errorf("hint lost originating scope %s: %s", mode, command)
							}
							var copiedErr error
							captureStdout(t, func() { copiedErr = cmdProfile(strings.Fields(command)[2:]) })
							if copiedErr != nil {
								t.Fatal("copied hint failed inside project", copiedErr)
							}
						}
					}
				}
				if operation == "agent" {
					agents := cfg.EffectiveAgentsSource()
					if project {
						agents = projectCfg.EffectiveAgentsSource(root)
					}
					if _, err := os.Stat(filepath.Join(agents, "_agents", "reviewer.md")); err != nil {
						t.Fatal("agent success fixture did not publish", err)
					}
				}
			})
		}
	}
}

func TestGitInstallPublicationRoots(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
		repo := t.TempDir()
		paths := []string{"one/a", "two/b"}
		if tracked {
			paths = []string{"one/review", "two/review"}
		}
		for _, path := range paths {
			writeHintSkill(t, repo, path)
		}
		initGitRepoForTest(t, repo)
		source := &install.Source{Type: install.SourceTypeGitHTTPS, Raw: "file://" + filepath.ToSlash(repo), CloneURL: "file://" + filepath.ToSlash(repo), Name: "repo"}
		var summary installLogSummary
		var err error
		opts := install.InstallOptions{Track: tracked, Into: "team", All: true, SkipAudit: true}
		if tracked {
			opts.Name = "repo"
		}
		opts.DryRun = true
		captureStdout(t, func() { summary, err = dispatchInstall(source, cfg, opts) })
		if err != nil || len(summary.InstalledPaths) != 0 {
			t.Fatalf("dry git publication: %+v (%v)", summary, err)
		}
		opts.DryRun = false
		captureStdout(t, func() { summary, err = dispatchInstall(source, cfg, opts) })
		if err != nil || len(summary.InstalledPaths) == 0 {
			t.Fatalf("successful git roots missing: %+v (%v)", summary, err)
		}
		hint := installedProfileHint(cfg, cfg.Source, sourcewalk.Options{}, summary.InstalledPaths)
		want := `Not in active profile "work": skillshare profile add work team__a team__b`
		if tracked {
			want = `Not in active profile "work": skillshare profile add work team___repo__one__review team___repo__two__review`
		}
		if hint != want {
			t.Fatal("missing final git names", hint)
		}
		output := captureStdout(t, func() { summary, err = dispatchInstall(source, cfg, opts) })
		if tracked {
			if err != nil || len(summary.FailedSkills) != 0 || !strings.Contains(output, install.ErrSkipSameRepo.Error()) {
				t.Fatalf("tracked same-repo reinstall must skip: %+v (%v): %s", summary, err, output)
			}
		} else {
			// Plain --into installs use the existing-path refusal at this baseline.
			if err == nil || strings.Join(summary.FailedSkills, ",") != "a,b" || !strings.Contains(err.Error(), "a: already exists.") || !strings.Contains(err.Error(), "b: already exists.") {
				t.Fatalf("plain reinstall must report both existing paths: %+v (%v)", summary, err)
			}
		}
		if len(summary.InstalledPaths) != 0 || len(summary.InstalledSkills) != 0 || summary.SkillCount != 0 {
			t.Fatalf("skipped/failed reinstall must not publish again: %+v (%v)", summary, err)
		}
		if got := installedProfileHint(cfg, cfg.Source, cfg.SkillsWalk(), summary.InstalledPaths); got != "" {
			t.Fatal("skipped/failed reinstall must not hint", got)
		}
	}
}

func TestMoveCLIProfileCatalogByMode(t *testing.T) {
	for _, project := range []bool{false, true} {
		_, _, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [edit]\n", profile.State{Version: 1})
		mode, path := "-g", config.ConfigPath()
		if project {
			t.Chdir(root)
			mode, path = "-p", config.ProjectConfigPath(root)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var commandErr error
		output := captureStdout(t, func() { commandErr = cmdMove([]string{mode, "edit", "team", "--json", "--dry-run"}) })
		var out moveJSONOutput
		if err := json.Unmarshal([]byte(output), &out); err != nil || commandErr != nil || len(out.Moved) != 1 || len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], `profile "work"`) {
			t.Fatalf("CLI catalog missing: %s (%v, %v)", output, err, commandErr)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(before) != string(after) {
			t.Fatal("move warning rewrote profile", err)
		}
	}
}

func TestConfigInstallHintsPartialSuccess(t *testing.T) {
	for _, project := range []bool{false, true} {
		cfg, pc, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
		origin := writeHintSkill(t, t.TempDir(), "review")
		entries := []config.SkillEntry{{Name: "review", Group: "team", Source: origin}, {Name: "broken", Source: filepath.Join(root, "absent")}}
		mode := "-g"
		if project {
			t.Chdir(root)
			mode = "-p"
			pc.Skills = entries
			if err := pc.Save(root); err != nil {
				t.Fatal(err)
			}
		} else {
			store := install.LoadMetadataOrNew(cfg.Source)
			for _, entry := range entries {
				store.Set(entry.FullName(), &install.MetadataEntry{Source: entry.Source, Group: entry.Group})
			}
			if err := store.Save(cfg.Source); err != nil {
				t.Fatal(err)
			}
		}
		var commandErr error
		output := captureStdout(t, func() { commandErr = cmdInstall([]string{mode, "--skip-audit"}) })
		if commandErr != nil || strings.Count(output, `Not in active profile "work"`) != 1 || !strings.Contains(output, "work team__review") || strings.Contains(output, "work broken") {
			t.Fatalf("partial success hints: %s (%v)", output, commandErr)
		}
		if _, err := os.Stat(filepath.Join(cfg.Source, "team", "review", "SKILL.md")); err != nil {
			t.Fatal("successful publication missing", err)
		}
	}
}

func TestGitBatchHintKeepsOnlySuccessfulRoots(t *testing.T) {
	cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
	repo := t.TempDir()
	for _, name := range []string{"good", "bad"} {
		dir := writeHintSkill(t, repo, name)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	initGitRepoForTest(t, repo)
	// A destination conflict fails one entry without invalidating the other.
	writeHintSkill(t, cfg.Source, "team/bad")
	source := &install.Source{Type: install.SourceTypeGitHTTPS, Raw: "file://" + filepath.ToSlash(repo), CloneURL: "file://" + filepath.ToSlash(repo)}
	var summary installLogSummary
	var commandErr error
	captureStdout(t, func() {
		summary, commandErr = handleGitInstall(source, cfg, install.InstallOptions{Into: "team", All: true, SkipAudit: true})
	})
	if commandErr != nil || len(summary.FailedSkills) != 1 || len(summary.InstalledSkills) != 1 || len(summary.InstalledPaths) != 1 || summary.InstalledSkills[0] != "good" {
		t.Fatalf("batch effects/counts: %+v (%v)", summary, commandErr)
	}
	if got := installedProfileHint(cfg, cfg.Source, cfg.SkillsWalk(), summary.InstalledPaths); got != `Not in active profile "work": skillshare profile add work team__good` {
		t.Fatal("failed destination must not hint", got)
	}
}
