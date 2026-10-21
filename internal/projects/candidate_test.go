package projects

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func projectCandidateFixture(t *testing.T, body string) (string, *config.Config, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	data := []byte(strings.NewReplacer("$ROOT/", dir+string(filepath.Separator), "$ROOT", dir).Replace("# project collection\ntargets: {}\nprojects:\n" + body))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	return path, cfg, dir, data
}

func TestPrepareProjects_EffectiveAffected(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mutate     func(map[string]config.ManagedProject, string)
		want       []string
	}{
		{"anchor owner", "  $ROOT/a: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/z: *shared\n", func(p map[string]config.ManagedProject, root string) {
			p[filepath.Join(root, "a")].Skills.Mode = "copy"
		}, []string{"a", "z"}},
		{"alias", "  $ROOT/a: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/z: *shared\n", func(p map[string]config.ManagedProject, root string) {
			p[filepath.Join(root, "z")].Skills.Mode = "copy"
		}, []string{"z"}},
		{"remove owner", "  $ROOT/a: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/z: *shared\n", func(p map[string]config.ManagedProject, root string) {
			delete(p, filepath.Join(root, "a"))
		}, []string{"a"}},
		{"merge", "  $ROOT/a: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/z:\n    <<: *shared\n    name: last\n", func(p map[string]config.ManagedProject, root string) {
			p[filepath.Join(root, "a")].Skills.Mode = "copy"
		}, []string{"a", "z"}},
		{"add and remove sorted", "  $ROOT/z:\n    targets: [claude]\n    skills: {}\n", func(p map[string]config.ManagedProject, root string) {
			delete(p, filepath.Join(root, "z"))
			p[filepath.Join(root, "a")] = config.ManagedProject{Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{}}
		}, []string{"a", "z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, cfg, root, original := projectCandidateFixture(t, tc.body)
			c, err := PrepareProjects(path, cfg, func(p map[string]config.ManagedProject) error { tc.mutate(p, root); return nil })
			if err != nil {
				t.Fatal(err)
			}
			want := make([]string, len(tc.want))
			for i, name := range tc.want {
				want[i] = filepath.Join(root, name)
			}
			if !reflect.DeepEqual(c.Affected, want) {
				t.Fatalf("Affected = %v, want %v", c.Affected, want)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
				t.Fatal("preparation wrote config")
			}
			if !strings.Contains(string(c.Save.Bytes), "# project collection") {
				t.Fatal("candidate lost comment")
			}
			if c.Save.Expanded == nil {
				t.Fatal("missing expanded candidate")
			}
			if err := c.Save.Write(); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, c.Save.Bytes) {
				t.Fatal("Write did not persist exact candidate bytes")
			}
		})
	}
}

func TestPrepareProjects_RejectsEffectiveDuplicate(t *testing.T) {
	path, cfg, root, original := projectCandidateFixture(t, "  $ROOT/a: &shared\n    targets: [claude]\n    skills: {}\n  $ROOT/z: *shared\n")
	c, err := PrepareProjects(path, cfg, func(p map[string]config.ManagedProject) error {
		v := p[filepath.Join(root, "a")]
		v.Name = "duplicate"
		p[filepath.Join(root, "a")] = v
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "both named") || c.Err != err {
		t.Fatalf("duplicate accepted: %v", err)
	}
	if c.Save == nil || c.Save.Write() == nil {
		t.Fatal("invalid candidate can be written")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
		t.Fatal("invalid candidate changed disk")
	}
	if cfg.Projects[filepath.Join(root, "a")].Name != "" {
		t.Fatal("invalid mutation changed caller")
	}
}

func TestPrepareProjects_ValidatesMissingProjects(t *testing.T) {
	for _, kind := range []string{"skills", "agents"} {
		for _, field := range []string{"include", "exclude", "mode"} {
			t.Run(kind+"/"+field, func(t *testing.T) {
				path, cfg, root, original := projectCandidateFixture(t, "  $ROOT/missing:\n    targets: [claude]\n    skills: {}\n    agents: {}\n")
				c, err := PrepareProjects(path, cfg, func(p map[string]config.ManagedProject) error {
					rc := p[filepath.Join(root, "missing")].Skills
					if kind == "agents" {
						rc = p[filepath.Join(root, "missing")].Agents
					}
					switch field {
					case "include":
						rc.Include = []string{"["}
					case "exclude":
						rc.Exclude = []string{"["}
					case "mode":
						rc.Mode = "invalid"
					}
					return nil
				})
				want := "invalid " + field + " pattern"
				if field == "mode" {
					want = "invalid sync mode"
				}
				if err == nil || !strings.Contains(err.Error(), want) || c.Err != err {
					t.Fatalf("invalid %s accepted: %v", field, err)
				}
				if c.Save == nil || c.Save.Write() == nil {
					t.Fatal("project validation error did not block Save.Write")
				}
				if got, _ := os.ReadFile(path); !bytes.Equal(got, original) {
					t.Fatal("invalid candidate changed disk")
				}
			})
		}
	}
	t.Run("naming", func(t *testing.T) {
		path, cfg, root, _ := projectCandidateFixture(t, "  $ROOT/missing:\n    targets: [claude]\n    skills: {}\n")
		_, err := PrepareProjects(path, cfg, func(p map[string]config.ManagedProject) error {
			p[filepath.Join(root, "missing")].Skills.TargetNaming = "prefixed"
			return nil
		})
		if err == nil {
			t.Fatal("prefixed project naming without copy mode accepted")
		}
	})
}

func TestPrepareProjects_DeepCopyOnSuccessAndMutationError(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "error"}[fail], func(t *testing.T) {
			path, cfg, root, _ := projectCandidateFixture(t, "  $ROOT/a:\n    targets: [claude]\n    skills:\n      include: [keep]\n      exclude: [skip]\n      enabled: false\n    agents:\n      include: [agent]\n      exclude: [other]\n      enabled: false\n")
			p := cfg.Projects[filepath.Join(root, "a")]
			mutationErr := errors.New("mutation failed")
			c, err := PrepareProjects(path, cfg, func(projects map[string]config.ManagedProject) error {
				v := projects[filepath.Join(root, "a")]
				v.Targets[0] = "cursor"
				for _, rc := range []*config.ResourceTargetConfig{v.Skills, v.Agents} {
					rc.Include[0] = "changed"
					rc.Exclude[0] = "changed"
					*rc.Enabled = true
				}
				projects[filepath.Join(root, "a")] = v
				if fail {
					return mutationErr
				}
				return nil
			})
			if fail && (err != mutationErr || c.Err != err) {
				t.Fatalf("mutation error contract: %v", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			if p.Targets[0] != "claude" || p.Skills.Include[0] != "keep" || p.Skills.Exclude[0] != "skip" || *p.Skills.Enabled || p.Agents.Include[0] != "agent" || p.Agents.Exclude[0] != "other" || *p.Agents.Enabled {
				t.Fatalf("mutation changed caller: %+v", p)
			}
			if fail && c.Save != nil {
				t.Fatal("failed mutation prepared a save")
			}
		})
	}
}

func TestPrepareProjects_NoOpNormalization(t *testing.T) {
	path, cfg, root, _ := projectCandidateFixture(t, "  $ROOT/a:\n    targets: []\n    skills:\n      include: []\n      exclude: []\n    agents: {}\n")
	c, err := PrepareProjects(path, cfg, func(map[string]config.ManagedProject) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Affected) != 0 {
		t.Fatalf("serialization normalization reported as edit: %v", c.Affected)
	}
	if c.Save.Expanded.Projects[filepath.Join(root, "a")].Skills == nil {
		t.Fatal("empty enabled resource lost")
	}
}

func TestPrepareProjects_InitialProjectsAndAgentSymlinkFilters(t *testing.T) {
	dir := t.TempDir()
	c, err := PrepareProjects(filepath.Join(dir, "config.yaml"), &config.Config{}, func(p map[string]config.ManagedProject) error {
		p[dir] = config.ManagedProject{Targets: []string{"claude"}, Agents: &config.ResourceTargetConfig{Mode: "symlink", Include: []string{"agent*"}}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Affected, []string{dir}) {
		t.Fatalf("initial affected = %v", c.Affected)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("preparation created config: %v", err)
	}
}
