package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOMPAccountPaths(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	cfg, root, err := loadWithTargets(t, "  omp-work:\n    agent: omp\n    config_dir: $ROOT/.omp-work/agent\n")
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["omp-work"]
	if got := target.SkillsConfig().Path; got != filepath.Join(root, ".omp-work", "agent", "skills") {
		t.Fatalf("account skills: %q", got)
	}
	instructions, ok := TargetInstructions("omp-work", target, false)
	if !ok || instructions.Path != filepath.Join(target.ConfigDir, "AGENTS.md") {
		t.Fatalf("account instructions: %+v %t", instructions, ok)
	}
	fileRoot, ok := TargetFileRoot("omp-work", target, false, "")
	if !ok || filepath.Clean(fileRoot) != filepath.Clean(target.ConfigDir) {
		t.Fatalf("account file root: %q %t", fileRoot, ok)
	}
	project, ok := LookupProjectTarget("oh-my-pi")
	if !ok || project.Path != filepath.Join(".omp", "skills") {
		t.Fatalf("project skills alias: %+v %t", project, ok)
	}
}

func TestOMPGlobalSkillsPaths(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	// Repeated lookups must not cache an environment-selected default.
	first, second := t.TempDir(), t.TempDir()
	for _, tc := range []struct{ dir, want string }{
		{first, filepath.Join(first, "skills")},
		{second, filepath.Join(second, "skills")},
		{"relative/agent", filepath.Join(cwd, "relative", "agent", "skills")},
		{"~/literal-agent", filepath.Join(cwd, "~", "literal-agent", "skills")},
		{" agent ", filepath.Join(cwd, " agent ", "skills")},
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
		for _, alias := range []string{"omp", "oh-my-pi"} {
			project, ok := LookupProjectTarget(alias)
			if !ok || project.Path != filepath.Join(".omp", "skills") {
				t.Fatalf("project %s = %+v, %t", alias, project, ok)
			}
		}
	}
	if !MatchesTargetName("oh-my-pi", "omp") {
		t.Fatal("OMP alias no longer matches")
	}
}

func TestOMPLoadSkillsPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	for _, tc := range []struct{ name, body, want string }{
		{"default", "  omp: {}\n", filepath.Join(dir, "skills")},
		{"nested", "  omp:\n    skills:\n      path: $ROOT/pinned\n", "$ROOT/pinned"},
		{"legacy", "  omp:\n    path: $ROOT/legacy\n", "$ROOT/legacy"},
		{"saved default", "  omp:\n    skills:\n      path: ~/.omp/agent/skills\n", normalizeTargetPath("~/.omp/agent/skills")},
		{"filters", "  omp:\n    skills:\n      include: [team__*]\n      exclude: [team__draft]\n      mode: copy\n", filepath.Join(dir, "skills")},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}
}

func TestOMPAccountSkillsPathsWithOverride(t *testing.T) {
	accountDir := t.TempDir()
	for _, agent := range []string{"omp", "oh-my-pi"} {
		for _, override := range []string{t.TempDir(), accountDir} {
			t.Run(agent+"/"+filepath.Base(override), func(t *testing.T) {
				t.Setenv("PI_CODING_AGENT_DIR", override)
				for _, explicit := range []bool{false, true} {
					body := fmt.Sprintf("  omp-work:\n    agent: %s\n    config_dir: %q\n", agent, accountDir)
					if explicit {
						body += "    skills:\n      path: $ROOT/pinned-account\n"
					}
					cfg, root, err := loadWithTargets(t, body)
					if err != nil {
						t.Fatal(err)
					}
					target := cfg.Targets["omp-work"]
					want := filepath.Join(accountDir, "skills")
					if explicit {
						want = filepath.Join(root, "pinned-account")
					}
					if got := target.SkillsConfig().Path; got != want {
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
