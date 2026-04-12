// Package testutil provides utilities for integration testing
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	resolvedBinaryOnce sync.Once
	resolvedBinaryPath string
	resolvedBinaryErr  error
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

	var err error
	sb.BinaryPath, err = resolveTestBinaryPath()
	if err != nil {
		t.Fatalf("failed to resolve skillshare test binary: %v", err)
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

	// Point XDG variables into the sandbox so config.BaseDir()/DataDir()/
	// StateDir() resolve to sandbox paths.  Without this, CI runners that
	// set XDG_CONFIG_HOME (e.g. ubuntu-latest) cause the subprocess to
	// write files outside the sandbox.
	sb.SetEnv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	sb.SetEnv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	sb.SetEnv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	return sb
}

func resolveTestBinaryPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("SKILLSHARE_TEST_BINARY")); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("SKILLSHARE_TEST_BINARY %q is not usable: %w", envPath, err)
		}
		return envPath, nil
	}

	resolvedBinaryOnce.Do(func() {
		repoRoot, err := testRepoRoot()
		if err != nil {
			resolvedBinaryErr = err
			return
		}

		for _, candidate := range testBinaryCandidates(repoRoot) {
			if _, err := os.Stat(candidate); err == nil {
				resolvedBinaryPath = candidate
				return
			}
		}

		buildDir, err := os.MkdirTemp("", "skillshare-test-bin-*")
		if err != nil {
			resolvedBinaryErr = fmt.Errorf("create temp dir for test binary: %w", err)
			return
		}

		resolvedBinaryPath, resolvedBinaryErr = buildTestBinary(repoRoot, buildDir)
	})

	if resolvedBinaryErr != nil {
		return "", resolvedBinaryErr
	}
	if resolvedBinaryPath == "" {
		return "", fmt.Errorf("skillshare test binary path is empty")
	}
	return resolvedBinaryPath, nil
}

func testRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("determine internal/testutil path: runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("locate repo root from %s: %w", file, err)
	}
	return root, nil
}

func testBinaryCandidates(repoRoot string) []string {
	candidates := []string{
		filepath.Join(repoRoot, "bin", "skillshare"),
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(repoRoot, "bin", "skillshare.exe"))
	}
	return candidates
}

func buildTestBinary(repoRoot, buildDir string) (string, error) {
	binaryName := "skillshare"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(buildDir, binaryName)
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/skillshare")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return "", fmt.Errorf("go build %s failed: %w", binaryPath, err)
		}
		return "", fmt.Errorf("go build %s failed: %w: %s", binaryPath, err, trimmed)
	}
	return binaryPath, nil
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
