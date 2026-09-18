package codexskills

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T, root, rel string) Skill {
	t.Helper()
	dir := filepath.Join(root, rel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: gem\ndescription: Find gems.\n---\nRead references when needed.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return Skill{Name: "gem", Path: path, Scope: "user", Enabled: true}
}

func TestPlanFourCopiesPrefersSource(t *testing.T) {
	root := t.TempDir()
	keep := fixture(t, root, "source/gem")
	copies := []Skill{fixture(t, root, "copy/archive/references/gem"), fixture(t, root, "source/archive/references/gem"), fixture(t, root, "copy/gem"), keep}
	p, err := BuildPlan(copies, filepath.Join(root, "source"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 || p.Groups[0].Keep != keep.Path || len(p.Groups[0].Disable) != 3 || len(p.Skipped) != 0 {
		t.Fatalf("bad plan: %+v", p)
	}
	for i, j := 0, len(copies)-1; i < j; i, j = i+1, j-1 {
		copies[i], copies[j] = copies[j], copies[i]
	}
	again, err := BuildPlan(copies, filepath.Join(root, "source"))
	if err != nil || !reflect.DeepEqual(p, again) {
		t.Fatalf("unstable plan: %+v, %v", again, err)
	}
}

func TestPlanSkipsDifferentFolders(t *testing.T) {
	for _, file := range []string{"SKILL.md", "references/usage.md", "agents/openai.yaml", "scripts/run.sh"} {
		t.Run(file, func(t *testing.T) {
			root := t.TempDir()
			a, b := fixture(t, root, "source/gem"), fixture(t, root, "copy/gem")
			path := filepath.Join(filepath.Dir(b.Path), file)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("different"), 0644); err != nil {
				t.Fatal(err)
			}
			p, err := BuildPlan([]Skill{a, b}, filepath.Join(root, "source"))
			if err != nil || len(p.Groups) != 0 || len(p.Skipped) != 1 {
				t.Fatalf("unsafe plan: %+v %v", p, err)
			}
		})
	}
}

func TestPlanPermissionAndDependencyBoundaries(t *testing.T) {
	for _, kind := range []string{"empty-directory", "executable", "parent-reference", "symlink", "plugin", "repo", "system"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" && (kind == "executable" || kind == "symlink") {
				t.Skip("Unix permissions/symlinks")
			}
			root := t.TempDir()
			a, b := fixture(t, root, "source/gem"), fixture(t, root, "copy/gem")
			switch kind {
			case "empty-directory":
				if err := os.Mkdir(filepath.Join(filepath.Dir(b.Path), "assets"), 0755); err != nil {
					t.Fatal(err)
				}
			case "executable":
				if err := os.Chmod(b.Path, 0755); err != nil {
					t.Fatal(err)
				}
			case "parent-reference":
				for _, s := range []Skill{a, b} {
					if err := os.WriteFile(s.Path, []byte("Read ../shared/SKILL.md"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			case "symlink":
				for _, s := range []Skill{a, b} {
					if err := os.Symlink(s.Path, filepath.Join(filepath.Dir(s.Path), "reference.md")); err != nil {
						t.Fatal(err)
					}
				}
			case "plugin":
				b.PluginID = "example@marketplace"
			default:
				b.Scope = kind
			}
			p, err := BuildPlan([]Skill{a, b}, filepath.Join(root, "source"))
			if err != nil || len(p.Groups) != 0 || len(p.Skipped) != 1 {
				t.Fatalf("unsafe plan: %+v %v", p, err)
			}
		})
	}
}

func TestPlanIgnoresDisabledEntriesAndCanonicalAliases(t *testing.T) {
	root := t.TempDir()
	a, b := fixture(t, root, "source/gem"), fixture(t, root, "copy/gem")
	b.Enabled = false
	all := []Skill{a, b, a}
	if runtime.GOOS != "windows" {
		link := filepath.Join(root, "alias")
		if err := os.Symlink(filepath.Dir(a.Path), link); err != nil {
			t.Fatal(err)
		}
		alias := a
		alias.Path = filepath.Join(link, "SKILL.md")
		all = append(all, alias)
	}
	p, err := BuildPlan(all, filepath.Join(root, "source"))
	if err != nil || len(p.Groups) != 0 || len(p.Skipped) != 0 {
		t.Fatalf("bad plan: %+v %v", p, err)
	}
}

type fakeCatalog struct {
	skills      []Skill
	calls       []string
	failAt      int
	ignoreWrite bool
	listError   error
}

func (f *fakeCatalog) List(string) ([]Skill, error) {
	return append([]Skill(nil), f.skills...), f.listError
}
func (f *fakeCatalog) Disable(path string) error {
	f.calls = append(f.calls, path)
	if len(f.calls) == f.failAt {
		return errors.New("write failed")
	}
	if !f.ignoreWrite {
		for i := range f.skills {
			if f.skills[i].Path == path {
				f.skills[i].Enabled = false
			}
		}
	}
	return nil
}

func applyFixture(t *testing.T) (*fakeCatalog, Plan, string, string) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "source")
	f := &fakeCatalog{skills: []Skill{fixture(t, root, "source/gem"), fixture(t, root, "copy-a/gem"), fixture(t, root, "copy-b/gem")}}
	other := fixture(t, root, "other")
	other.Name = "other"
	f.skills = append(f.skills, other)
	p, err := BuildPlan(f.skills, src)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("# user setting\nmodel = \"example\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return f, p, src, config
}

func TestApplyVerifiesAndIsIdempotent(t *testing.T) {
	f, p, src, config := applyFixture(t)
	result, err := Apply(f, "/workspace", src, config, p)
	if err != nil || !result.Verified || len(result.Applied) != 2 {
		t.Fatalf("apply: %+v %v", result, err)
	}
	backup, err := os.ReadFile(result.Backup)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(original) {
		t.Fatal("backup differs from original config")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(result.Backup)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("backup must be private")
		}
	}
	again, err := BuildPlan(f.skills, src)
	if err != nil {
		t.Fatal(err)
	}
	again, err = Apply(f, "/workspace", src, config, again)
	if err != nil || !again.Verified || len(again.Groups) != 0 || again.Backup != "" || len(f.calls) != 2 {
		t.Fatalf("not idempotent: %+v %v", again, err)
	}
}

func TestApplyRefusesChangedPreview(t *testing.T) {
	f, p, src, config := applyFixture(t)
	// Even if all folders change identically, the old preview is stale.
	for _, s := range f.skills {
		if s.Name == "gem" {
			if err := os.WriteFile(s.Path, []byte("changed"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	result, err := Apply(f, "/workspace", src, config, p)
	if err == nil || len(f.calls) != 0 || result.Backup != "" {
		t.Fatalf("stale plan applied: %+v %v", result, err)
	}
}

func TestApplyReportsPartialWriteAndVerificationFailure(t *testing.T) {
	for _, kind := range []string{"partial", "ignored-write", "load-error", "backup-error"} {
		t.Run(kind, func(t *testing.T) {
			f, p, src, config := applyFixture(t)
			switch kind {
			case "partial":
				f.failAt = 2
			case "ignored-write":
				f.ignoreWrite = true
			case "load-error":
				f.listError = errors.New("catalog unavailable")
			case "backup-error":
				config = filepath.Join(config, "missing", "config.toml")
			}
			result, err := Apply(f, "/workspace", src, config, p)
			if err == nil || result.Verified {
				t.Fatalf("unexpected success: %+v %v", result, err)
			}
			if kind == "partial" && (len(result.Applied) != 1 || !strings.Contains(err.Error(), "unverified")) {
				t.Fatalf("partial write lost: %+v %v", result, err)
			}
			if (kind == "load-error" || kind == "backup-error") && len(f.calls) != 0 {
				t.Fatal("wrote before preflight completed")
			}
		})
	}
}
