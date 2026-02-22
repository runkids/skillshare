package server

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

// newTestServer creates an isolated Server for handler testing.
// It sets up a temp source directory and config file, returning the server
// and the source directory path for test setup.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "")
	return s, sourceDir
}

// newTestServerWithTargets creates a test server with pre-configured targets.
func newTestServerWithTargets(t *testing.T, targets map[string]string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n"
	for name, path := range targets {
		os.MkdirAll(path, 0755)
		raw += "  " + name + ":\n    path: " + path + "\n"
	}
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "")
	return s, sourceDir
}

// addSkill creates a skill directory with SKILL.md in the source directory.
func addSkill(t *testing.T, sourceDir, name string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
}
