package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTargetFileRoot_PiGlobalIsConfigDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	root, ok := TargetFileRoot("pi", TargetConfig{}, false, "")
	if !ok || root != filepath.Join(home, ".pi", "agent") {
		t.Errorf("pi global root = %q, %v", root, ok)
	}
}

func TestTargetFileRoot_OmpGlobalIsSkillsParent(t *testing.T) {
	home, _ := os.UserHomeDir()
	root, ok := TargetFileRoot("omp", TargetConfig{}, false, "")
	if !ok || root != filepath.Join(home, ".omp", "agent") {
		t.Errorf("omp global root = %q, %v", root, ok)
	}
}

func TestTargetFileRoot_PiProject(t *testing.T) {
	proj := t.TempDir()
	root, ok := TargetFileRoot("pi", TargetConfig{}, true, proj)
	if !ok || root != filepath.Join(proj, ".pi") {
		t.Errorf("pi project root = %q, %v", root, ok)
	}
}

func TestTargetFileRoot_AccountUsesItsConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pi-work")
	root, ok := TargetFileRoot("pi-work", TargetConfig{Agent: "pi", ConfigDir: dir}, false, "")
	if !ok || root != dir {
		t.Errorf("account root = %q, %v", root, ok)
	}
}

func TestTargetFileRoot_TooBroadIsRefused(t *testing.T) {
	home, _ := os.UserHomeDir()
	proj := t.TempDir()
	cases := map[string]struct {
		name    string
		tc      TargetConfig
		project bool
	}{
		"home":         {"myagent", TargetConfig{Path: filepath.Join(home, "skills")}, false},
		"fs root":      {"myagent", TargetConfig{Path: string(filepath.Separator) + "skills"}, false},
		"project root": {"openclaw", TargetConfig{}, true},
		"none":         {"myagent", TargetConfig{}, false},
	}
	for label, c := range cases {
		if root, ok := TargetFileRoot(c.name, c.tc, c.project, proj); ok {
			t.Errorf("%s: root = %q, want refused", label, root)
		}
	}
}

func TestTargetFiles_BuiltinThenUserWithoutDuplicates(t *testing.T) {
	tc := TargetConfig{Files: []string{"prompts/review.md", "./APPEND_SYSTEM.md", "prompts/review.md"}}
	want := []TargetFile{{Path: "APPEND_SYSTEM.md", Builtin: true}, {Path: "prompts/review.md"}}
	if got := TargetFiles("pi", tc, false); !reflect.DeepEqual(got, want) {
		t.Errorf("files = %+v, want %+v", got, want)
	}
}

func targetFileReason(err error) string {
	var pe *TargetFilePathError
	if errors.As(err, &pe) {
		return pe.Reason
	}
	return ""
}

func TestValidateTargetFilePath_Reasons(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "prompts"), 0755)
	cases := map[string]string{
		"":             TargetFileEmpty,
		"  ":           TargetFileEmpty,
		"a\x00b":       TargetFileEmpty,
		"/etc/passwd":  TargetFileAbsolute,
		"~/x.md":       TargetFileAbsolute,
		"../x.md":      TargetFileOutside,
		"a/../../x.md": TargetFileOutside,
		"prompts":      TargetFileIsDir,
		"notes/":       TargetFileIsDir,
	}
	for rel, want := range cases {
		if _, err := ValidateTargetFilePath(root, rel); targetFileReason(err) != want {
			t.Errorf("%q: err = %v, want %s", rel, err, want)
		}
	}
	if _, err := ValidateTargetFilePath("", "x.md"); targetFileReason(err) != TargetFileNoRoot {
		t.Errorf("no root: err = %v", err)
	}
}

func TestValidateTargetFilePath_SymlinkedAncestorEscapes(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := ValidateTargetFilePath(root, "link/x.md"); targetFileReason(err) != TargetFileOutside {
		t.Errorf("err = %v, want outside", err)
	}
}

func TestValidateTargetFilePath_FinalSymlinkAllowed(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	src := filepath.Join(outside, "shared.md")
	os.WriteFile(src, []byte("x"), 0644)
	if err := os.Symlink(src, filepath.Join(root, "SYSTEM.md")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	abs, err := ValidateTargetFilePath(root, "SYSTEM.md")
	if err != nil || abs != filepath.Join(root, "SYSTEM.md") {
		t.Errorf("abs = %q, err = %v", abs, err)
	}
}

func TestValidateTargetFilePath_SubfolderOK(t *testing.T) {
	root := t.TempDir()
	abs, err := ValidateTargetFilePath(root, " prompts/review.md ")
	if err != nil || abs != filepath.Join(root, "prompts", "review.md") {
		t.Errorf("abs = %q, err = %v", abs, err)
	}
}

func TestConfig_TargetFilesRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	raw := "source: " + filepath.Join(tmp, "skills") + "\ntargets:\n  pi:\n    path: " + filepath.Join(tmp, "pi") + "\n    files: [prompts/review.md]\n"
	os.WriteFile(cfgPath, []byte(raw), 0644)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	again, err := Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := again.Targets["pi"].Files; !reflect.DeepEqual(got, []string{"prompts/review.md"}) {
		t.Errorf("files = %v", got)
	}
}

func TestProjectConfig_TargetFilesRoundTrip(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, ".skillshare", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("targets:\n  - name: pi\n    files: [SYSTEM.md]\n"), 0644)
	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save: %v", err)
	}
	again, err := LoadProject(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	resolved, err := ResolveProjectTargets(root, again)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := resolved["pi"].Files; !reflect.DeepEqual(got, []string{"SYSTEM.md"}) {
		t.Errorf("files = %v", got)
	}
}

func TestTargetFileRoot_EnvironmentHomes(t *testing.T) {
	for _, tc := range []struct{ name, env string }{{"deepseek-harness", "DSH_HOME"}, {"gitlab-duo", "GLAB_CONFIG_DIR"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(tc.env, dir)
			root, ok := TargetFileRoot(tc.name, TargetConfig{}, false, "")
			if !ok || root != dir {
				t.Fatalf("file root = %q, %t; want %q", root, ok, dir)
			}
		})
	}
}
