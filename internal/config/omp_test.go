package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOMPGlobalSkillsPaths(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	// Repeated lookups must not cache an environment-selected default.
	first, second := t.TempDir(), t.TempDir()
	for _, tc := range []struct{ dir, want string }{
		{first, filepath.Join(first, "skills")},
		{second, filepath.Join(second, "skills")},
		{"relative/agent", normalizeTargetPath("~/.omp/agent/skills")},
		{"~/literal-agent", normalizeTargetPath("~/.omp/agent/skills")},
		{" agent", normalizeTargetPath("~/.omp/agent/skills")},
		{" \t" + first + " \n", filepath.Join(first, "skills")},
		{" \t ", normalizeTargetPath("~/.omp/agent/skills")},
		{"", normalizeTargetPath("~/.omp/agent/skills")},
	} {
		dir, want := tc.dir, tc.want
		t.Setenv("PI_CODING_AGENT_DIR", dir)
		target, ok := LookupGlobalTarget("omp")
		if !ok || target.Path != want {
			t.Fatalf("env %q: global = %+v, %t; want %s", dir, target, ok, want)
		}
		if got := RuntimeScanPaths("omp", false); !reflect.DeepEqual(got, []string{want}) {
			t.Fatalf("env %q: runtime scans = %v; want [%s]", dir, got, want)
		}
	}
}

func TestOMPLoadSkillsPaths(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, body, want string
		inferred, unset  bool
	}{
		{"default", "  omp: {}\n", filepath.Join(dir, "skills"), true, false},
		{"unset", "  omp: {}\n", normalizeTargetPath("~/.omp/agent/skills"), true, true},
		{"explicit override", fmt.Sprintf("  omp:\n    skills:\n      path: %q\n", filepath.Join(dir, "skills")), filepath.Join(dir, "skills"), false, false},
		{"nested", "  omp:\n    skills:\n      path: $ROOT/pinned\n", "$ROOT/pinned", false, false},
		{"legacy", "  omp:\n    path: $ROOT/legacy\n", "$ROOT/legacy", false, false},
		{"saved default", "  omp:\n    skills:\n      path: ~/.omp/agent/skills\n", normalizeTargetPath("~/.omp/agent/skills"), false, false},
		{"filters", "  omp:\n    skills:\n      include: [team__*]\n      exclude: [team__draft]\n      mode: copy\n", filepath.Join(dir, "skills"), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := dir
			if tc.unset {
				initial = ""
			}
			t.Setenv("PI_CODING_AGENT_DIR", initial)
			cfg, root, err := loadWithTargets(t, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			target := cfg.Targets["omp"]
			skills := target.SkillsConfig()
			if want := strings.ReplaceAll(tc.want, "$ROOT", root); skills.Path != want {
				t.Fatalf("loaded path = %q; want %q", skills.Path, want)
			}
			if tc.name == "filters" && (skills.Mode != "copy" || !reflect.DeepEqual(skills.Include, []string{"team__*"}) || !reflect.DeepEqual(skills.Exclude, []string{"team__draft"})) {
				t.Fatalf("changed filter configuration: %+v", skills)
			}
			if tc.name == "filters" {
				target.EnsureSkills().Include = append(skills.Include, "shared")
				cfg.Targets["omp"] = target
				skills = target.SkillsConfig()
			}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			next := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", next)
			reloaded, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if tc.inferred {
				skills.Path = filepath.Join(next, "skills")
			}
			if target := reloaded.Targets["omp"]; !reflect.DeepEqual(target.SkillsConfig(), skills) {
				t.Fatalf("reloaded skills = %+v; want %+v", target.SkillsConfig(), skills)
			}
		})
	}
}

func TestOMPAccountSkillsPathsWithOverride(t *testing.T) {
	accountDir := t.TempDir()
	for _, agent := range []string{"omp", "oh-my-pi"} {
		for _, override := range []string{"", t.TempDir(), accountDir} {
			t.Run(agent+"/"+filepath.Base(override), func(t *testing.T) {
				t.Setenv("PI_CODING_AGENT_DIR", override)
				for _, explicit := range []bool{false, true} {
					body := fmt.Sprintf("  omp: {}\n  omp-work:\n    agent: %s\n    config_dir: %q\n", agent, accountDir)
					if explicit {
						body += "    skills:\n      path: $ROOT/pinned-account\n"
					}
					cfg, root, err := loadWithTargets(t, body)
					if err != nil {
						t.Fatal(err)
					}
					plain := normalizeTargetPath("~/.omp/agent/skills")
					if override != "" && override != accountDir {
						plain = filepath.Join(override, "skills")
					}
					plainTarget := cfg.Targets["omp"]
					if got := plainTarget.SkillsConfig().Path; got != plain {
						t.Fatalf("default skills = %q; want %q", got, plain)
					}
					target := cfg.Targets["omp-work"]
					want := filepath.Join(accountDir, "skills")
					if explicit {
						want = filepath.Join(root, "pinned-account")
					}
					if got := target.SkillsConfig().Path; filepath.Clean(got) != want {
						t.Fatalf("account skills = %q; want %q", got, want)
					}
					if !explicit {
						if got := TargetScanPaths("omp-work", target, false); !reflect.DeepEqual(got, []string{want}) {
							t.Fatalf("account scans = %v; want [%s]", got, want)
						}
					}
					instructions, ok := TargetInstructions("omp-work", target, false)
					if !ok || instructions.Path != filepath.Join(accountDir, "AGENTS.md") {
						t.Fatalf("account instructions = %+v, %t", instructions, ok)
					}
					fileRoot, ok := TargetFileRoot("omp-work", target, false, "")
					if !ok || filepath.Clean(fileRoot) != filepath.Clean(accountDir) {
						t.Fatalf("account file root = %q, %t", fileRoot, ok)
					}
				}
				if got, want := AgentConfigDir(agent), normalizeTargetPath("~/.omp/agent"); got != want {
					t.Fatalf("canonical config dir = %q; want %q", got, want)
				}
			})
		}
	}
}

// Account identity follows symlinks, just as MCP and hooks do.
func TestOMPAccountShellSymlink(t *testing.T) {
	account := t.TempDir()
	link := filepath.Join(t.TempDir(), "account")
	if err := os.Symlink(account, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", link)
	cfg, _, err := loadWithTargets(t, fmt.Sprintf("  omp: {}\n  omp-work:\n    agent: omp\n    config_dir: %q\n    skills:\n      include: [work]\n", account))
	if err != nil {
		t.Fatal(err)
	}
	want := normalizeTargetPath("~/.omp/agent/skills")
	plainTarget := cfg.Targets["omp"]
	if got := plainTarget.SkillsConfig().Path; got != want {
		t.Fatalf("default skills = %q; want %q", got, want)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	next := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", next)
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	plainTarget = cfg.Targets["omp"]
	if got := plainTarget.SkillsConfig().Path; got != filepath.Join(next, "skills") {
		t.Fatalf("reloaded default skills = %q", got)
	}
	accountTarget := cfg.Targets["omp-work"]
	if got := accountTarget.SkillsConfig(); got.Path != filepath.Join(account, "skills") || !reflect.DeepEqual(got.Include, []string{"work"}) {
		t.Fatalf("reloaded account = %+v", got)
	}
}
