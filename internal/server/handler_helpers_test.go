package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// newTestServer creates an isolated Server for handler testing.
// It sets up a temp source directory and config file, returning the server
// and the source directory path for test setup.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	homeDir := filepath.Join(tmp, "home")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	if os.Getenv("HOME") == "" {
		os.MkdirAll(homeDir, 0755)
		t.Setenv("HOME", homeDir)
	}

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newTestServerWithTargets creates a test server with pre-configured targets.
func newTestServerWithTargets(t *testing.T, targets map[string]string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	homeDir := filepath.Join(tmp, "home")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	if os.Getenv("HOME") == "" {
		os.MkdirAll(homeDir, 0755)
		t.Setenv("HOME", homeDir)
	}

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

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newManagedProjectServer creates a project-mode server with one configured target.
func newManagedProjectServer(t *testing.T, targetName string) (*Server, string, string, string) {
	t.Helper()

	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	sourceDir := filepath.Join(tmp, "source")
	targetPath := filepath.Join(tmp, "targets", targetName)

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare"), 0755); err != nil {
		t.Fatalf("failed to create project config dir: %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	projectCfgPath := filepath.Join(projectRoot, ".skillshare", "config.yaml")
	raw := "targets:\n- name: " + targetName + "\n  path: " + targetPath + "\n"
	if err := os.WriteFile(projectCfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	projectCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}

	targets, err := config.ResolveProjectTargets(projectRoot, projectCfg)
	if err != nil {
		t.Fatalf("failed to resolve project targets: %v", err)
	}

	cfg := &config.Config{
		Source:  sourceDir,
		Targets: targets,
	}
	s := NewProject(cfg, projectCfg, projectRoot, "127.0.0.1:0", "", "")
	return s, projectRoot, sourceDir, targetPath
}

// addSkill creates a skill directory with SKILL.md in the source directory.
func addSkill(t *testing.T, sourceDir, name string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
}

func addTrackedRepo(t *testing.T, sourceDir, relPath string) {
	t.Helper()
	repoDir := filepath.Join(sourceDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create tracked repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("tracked repo"), 0644); err != nil {
		t.Fatalf("failed to seed tracked repo: %v", err)
	}
}

// addSkillMeta writes a metadata entry into the centralized .metadata.json store.
func addSkillMeta(t *testing.T, sourceDir, name, source string) {
	t.Helper()
	store := install.LoadMetadataOrNew(sourceDir)
	store.Set(name, &install.MetadataEntry{Source: source})
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("addSkillMeta: %v", err)
	}
}

func addAgent(t *testing.T, agentsDir, relPath string) {
	t.Helper()
	agentPath := filepath.Join(agentsDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(agentPath), 0o755); err != nil {
		t.Fatalf("create agent dir: %v", err)
	}
	if err := os.WriteFile(agentPath, []byte("---\nname: "+strings.TrimSuffix(filepath.Base(relPath), ".md")+"\n---\n# agent"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
}

func addAgentMeta(t *testing.T, agentsDir, relPath, source string) {
	t.Helper()
	store := install.LoadMetadataOrNew(agentsDir)
	key := strings.TrimSuffix(filepath.ToSlash(relPath), ".md")
	store.Set(key, &install.MetadataEntry{
		Source: source,
		Kind:   install.MetadataKindAgent,
		Subdir: filepath.ToSlash(relPath),
	})
	if err := store.Save(agentsDir); err != nil {
		t.Fatalf("addAgentMeta: %v", err)
	}
}
