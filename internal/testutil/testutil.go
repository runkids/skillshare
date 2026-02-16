// Package testutil provides utilities for integration testing
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// Sandbox represents an isolated test environment
type Sandbox struct {
	T          *testing.T
	Root       string            // Root temp directory
	Home       string            // Simulated home directory
	ConfigPath string            // Config file path
	SourcePath string            // Skills source directory
	OrigEnv    map[string]string // Original environment
	BinaryPath string            // Path to skillshare binary
}

// NewSandbox creates a new isolated test environment
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()

	root := t.TempDir()
	home := filepath.Join(root, "home")

	sb := &Sandbox{
		T:          t,
		Root:       root,
		Home:       home,
		ConfigPath: filepath.Join(home, ".config", "skillshare", "config.yaml"),
		SourcePath: filepath.Join(home, ".config", "skillshare", "skills"),
		OrigEnv:    make(map[string]string),
	}

	// Get binary path from environment or use default
	sb.BinaryPath = os.Getenv("SKILLSHARE_TEST_BINARY")
	if sb.BinaryPath == "" {
		// Try to find it relative to the project root
		// This works when running from the project directory
		cwd, _ := os.Getwd()
		candidates := []string{
			filepath.Join(cwd, "bin", "skillshare"),
			filepath.Join(cwd, "..", "..", "bin", "skillshare"),
			filepath.Join(cwd, "bin", "skillshare.exe"),
			filepath.Join(cwd, "..", "..", "bin", "skillshare.exe"),
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				sb.BinaryPath = path
				break
			}
		}
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(home, ".config", "skillshare"),
		filepath.Join(home, ".config", "skillshare", "skills"),
		filepath.Join(home, ".local", "share", "skillshare", "backups"),
		filepath.Join(home, ".local", "share", "skillshare", "trash"),
		filepath.Join(home, ".local", "state", "skillshare", "logs"),
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".cursor"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Override environment
	sb.SetEnv("HOME", home)
	sb.SetEnv("SKILLSHARE_CONFIG", sb.ConfigPath)

	return sb
}

// SetEnv sets an environment variable, saving the original
func (sb *Sandbox) SetEnv(key, value string) {
	sb.T.Helper()
	sb.OrigEnv[key] = os.Getenv(key)
	os.Setenv(key, value)
}

// Cleanup restores original environment
func (sb *Sandbox) Cleanup() {
	for key, value := range sb.OrigEnv {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

// CreateSkill creates a test skill in the source directory
func (sb *Sandbox) CreateSkill(name string, files map[string]string) string {
	sb.T.Helper()

	skillDir := filepath.Join(sb.SourcePath, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		sb.T.Fatalf("failed to create skill directory: %v", err)
	}

	for filename, content := range files {
		path := filepath.Join(skillDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			sb.T.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	return skillDir
}

// CreateNestedSkill creates a test skill at a nested path in the source directory.
// The relPath can use / as separator, e.g., "personal/writing/email" or "_team-repo/frontend/ui"
func (sb *Sandbox) CreateNestedSkill(relPath string, files map[string]string) string {
	sb.T.Helper()

	skillDir := filepath.Join(sb.SourcePath, filepath.FromSlash(relPath))
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		sb.T.Fatalf("failed to create nested skill directory: %v", err)
	}

	for filename, content := range files {
		path := filepath.Join(skillDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			sb.T.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	return skillDir
}

// CreateTarget creates a target directory
func (sb *Sandbox) CreateTarget(name string) string {
	sb.T.Helper()

	var path string
	switch name {
	case "claude":
		path = filepath.Join(sb.Home, ".claude", "skills")
	case "codex":
		path = filepath.Join(sb.Home, ".codex", "skills")
	case "cursor":
		path = filepath.Join(sb.Home, ".cursor", "skills")
	case "gemini":
		path = filepath.Join(sb.Home, ".gemini", "antigravity", "skills")
	case "opencode":
		path = filepath.Join(sb.Home, ".config", "opencode", "skills")
	default:
		path = filepath.Join(sb.Home, "."+name, "skills")
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		sb.T.Fatalf("failed to create target: %v", err)
	}

	return path
}

// WriteConfig writes a config file
func (sb *Sandbox) WriteConfig(cfg string) {
	sb.T.Helper()

	dir := filepath.Dir(sb.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		sb.T.Fatalf("failed to create config directory: %v", err)
	}

	if err := os.WriteFile(sb.ConfigPath, []byte(cfg), 0644); err != nil {
		sb.T.Fatalf("failed to write config: %v", err)
	}
}

// FileExists checks if a file exists
func (sb *Sandbox) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsSymlink checks if path is a symlink
func (sb *Sandbox) IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget returns the target of a symlink
func (sb *Sandbox) SymlinkTarget(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

// ReadFile reads and returns file contents
func (sb *Sandbox) ReadFile(path string) string {
	sb.T.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		sb.T.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(content)
}

// WriteFile writes content to a file
func (sb *Sandbox) WriteFile(path, content string) {
	sb.T.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		sb.T.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		sb.T.Fatalf("failed to write file %s: %v", path, err)
	}
}

// CreateSymlink creates a symbolic link
func (sb *Sandbox) CreateSymlink(target, link string) {
	sb.T.Helper()
	dir := filepath.Dir(link)
	if err := os.MkdirAll(dir, 0755); err != nil {
		sb.T.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.Symlink(target, link); err != nil {
		sb.T.Fatalf("failed to create symlink %s -> %s: %v", link, target, err)
	}
}

// SetupProjectDir creates a project directory with .skillshare/ structure.
// Returns the project root path.
func (sb *Sandbox) SetupProjectDir(targets ...string) string {
	sb.T.Helper()
	projectRoot := filepath.Join(sb.Root, "project")
	skillshareDir := filepath.Join(projectRoot, ".skillshare")
	skillsDir := filepath.Join(skillshareDir, "skills")

	for _, dir := range []string{skillsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			sb.T.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	// Write empty .gitignore
	if err := os.WriteFile(filepath.Join(skillshareDir, ".gitignore"), []byte(""), 0644); err != nil {
		sb.T.Fatalf("failed to create .gitignore: %v", err)
	}

	// Build config
	cfg := "targets:\n"
	for _, t := range targets {
		cfg += "  - " + t + "\n"
	}

	if err := os.WriteFile(filepath.Join(skillshareDir, "config.yaml"), []byte(cfg), 0644); err != nil {
		sb.T.Fatalf("failed to write config: %v", err)
	}

	// Create target directories
	knownPaths := map[string]string{
		"claude":      ".claude/skills",
		"claude-code": ".claude/skills", // legacy alias
		"cursor":      ".cursor/skills",
		"codex":       ".agents/skills",
	}
	for _, t := range targets {
		if p, ok := knownPaths[t]; ok {
			os.MkdirAll(filepath.Join(projectRoot, p), 0755)
		}
	}

	return projectRoot
}

// CreateProjectSkill creates a skill inside .skillshare/skills/.
func (sb *Sandbox) CreateProjectSkill(projectRoot, name string, files map[string]string) string {
	sb.T.Helper()
	skillDir := filepath.Join(projectRoot, ".skillshare", "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		sb.T.Fatalf("failed to create project skill: %v", err)
	}
	for filename, content := range files {
		path := filepath.Join(skillDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			sb.T.Fatalf("failed to write %s: %v", path, err)
		}
	}
	return skillDir
}

// WriteProjectConfig writes a config.yaml to .skillshare/config.yaml.
func (sb *Sandbox) WriteProjectConfig(projectRoot, cfg string) {
	sb.T.Helper()
	dir := filepath.Join(projectRoot, ".skillshare")
	if err := os.MkdirAll(dir, 0755); err != nil {
		sb.T.Fatalf("failed to create .skillshare: %v", err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		sb.T.Fatalf("failed to write project config: %v", err)
	}
}

// ListDir returns the names of files in a directory
func (sb *Sandbox) ListDir(path string) []string {
	sb.T.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		sb.T.Fatalf("failed to read directory %s: %v", path, err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}
