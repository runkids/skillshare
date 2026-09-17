package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gofrs/flock"
)

// Service is scoped to one Skillshare config; StateDir is shared across scopes
// so separate manifests cannot silently acquire the same native server.
type Service struct {
	ConfigPath  string
	ProjectRoot string
	Home        string
	StateDir    string
	Platform    string
	// ConfigDirs contains explicitly resolved native client directory overrides.
	ConfigDirs map[string]string
}

func (s *Service) nativePath(target string) (string, error) {
	if !validTarget(target) {
		return "", fmt.Errorf("unsupported MCP target %q", target)
	}
	if s.ProjectRoot != "" {
		if target == "opencode" {
			return openCodePath(s.ProjectRoot)
		}
		if target == "grok" {
			return filepath.Join(s.ProjectRoot, ".grok", "config.toml"), nil
		}
		paths := map[string][]string{"claude": {".mcp.json"}, "codex": {".codex", "config.toml"}, "cursor": {".cursor", "mcp.json"}, "vscode": {".vscode", "mcp.json"}}
		return filepath.Join(append([]string{s.ProjectRoot}, paths[target]...)...), nil
	}
	home := s.Home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	if dir := s.ConfigDirs[target]; dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s config directory must be absolute", target)
		}
		file := "config.toml"
		if target == "claude" {
			file = ".claude.json"
		}
		return filepath.Join(dir, file), nil
	}
	switch target {
	case "grok":
		return filepath.Join(home, ".grok", "config.toml"), nil
	case "opencode":
		base := s.ConfigDirs["xdg"]
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		return openCodePath(filepath.Join(base, "opencode"))
	case "claude":
		return filepath.Join(home, ".claude.json"), nil
	case "codex":
		return filepath.Join(home, ".codex", "config.toml"), nil
	case "cursor":
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	default:
		platform := s.Platform
		if platform == "" {
			platform = runtime.GOOS
		}
		switch platform {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json"), nil
		case "windows":
			base := s.ConfigDirs["appdata"]
			if base == "" {
				base = filepath.Join(home, "AppData", "Roaming")
			}
			return filepath.Join(base, "Code", "User", "mcp.json"), nil
		default:
			base := s.ConfigDirs["xdg"]
			if base == "" {
				base = filepath.Join(home, ".config")
			}
			return filepath.Join(base, "Code", "User", "mcp.json"), nil
		}
	}
}

func openCodePath(dir string) (string, error) {
	// Prefer the existing JSONC file; never create a second configuration that
	// would mask entries in an existing file. Ambiguous pairs need consolidation.
	var existing []string
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		path := filepath.Join(dir, name)
		if _, err := os.Lstat(path); err == nil {
			existing = append(existing, path)
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if len(existing) > 1 {
		return "", fmt.Errorf("both opencode.json and opencode.jsonc exist in %s; consolidate them before MCP sync", dir)
	}
	if len(existing) == 1 {
		return existing[0], nil
	}
	return filepath.Join(dir, "opencode.json"), nil
}

func (s *Service) statePath() string { return filepath.Join(s.StateDir, "mcp", "state.json") }

func (s *Service) lock() (*flock.Flock, error) {
	if s.StateDir == "" {
		return nil, fmt.Errorf("MCP state directory is required")
	}
	dir := filepath.Join(s.StateDir, "mcp")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "operation.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("another MCP operation is running; retry when it completes")
	}
	return lock, nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ClientPaths exposes native destinations for the current scope. A client whose
// path cannot be resolved is omitted; preview reports it when that client is used.
func (s *Service) ClientPaths() map[string]string {
	out := map[string]string{}
	for _, target := range Targets {
		if path, err := s.nativePath(target); err == nil {
			out[target] = path
		}
	}
	return out
}

// ConfigDirsFromEnv reads the Agent directory overrides Agents themselves honor.
func ConfigDirsFromEnv() map[string]string {
	dirs := map[string]string{}
	for key, env := range map[string]string{"codex": "CODEX_HOME", "claude": "CLAUDE_CONFIG_DIR", "grok": "GROK_HOME", "xdg": "XDG_CONFIG_HOME", "appdata": "APPDATA"} {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			dirs[key] = value
		}
	}
	return dirs
}

// DetectedClients lists Agents that look installed: their native MCP file
// exists, or its own config directory does. A file placed directly in the home
// or project root only counts when the file itself exists.
func (s *Service) DetectedClients(paths map[string]string) []string {
	home := s.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	out := []string{}
	for _, target := range Targets {
		path := paths[target]
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			out = append(out, target)
			continue
		}
		dir := filepath.Dir(path)
		if dir == filepath.Clean(home) || (s.ProjectRoot != "" && dir == filepath.Clean(s.ProjectRoot)) {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, target)
		}
	}
	return out
}
