package mcp

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	// BackupSource keeps a config file's content before a sync rewrites it without the
	// settings 0.23.0 retired, and returns where. The CLI and the server keep it in the
	// file history.
	BackupSource func(path string) (string, error)
	// accounts are the source's, set while resolving its targets.
	accounts map[string]Account
	// account is the one this Service was scoped to, empty when it is the Agent's own.
	account string
}

func (s *Service) nativePath(target string) (string, error) {
	if !validTarget(target) {
		return "", fmt.Errorf("unsupported MCP target %q", target)
	}
	if format, ok := clientFormats[target]; ok {
		return s.additionalClientPath(target, format)
	}
	if s.ProjectRoot != "" {
		if target == "antigravity" {
			return filepath.Join(s.ProjectRoot, ".agents", "mcp_config.json"), nil
		}
		if target == "opencode" {
			return jsoncPath("opencode", s.ProjectRoot, filepath.Join(s.ProjectRoot, ".opencode"))
		}
		if target == "kilocode" {
			return jsoncPath("kilo", s.ProjectRoot, filepath.Join(s.ProjectRoot, ".kilo"))
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
	if dir, _ := s.configDir(target); dir != "" {
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
	case "antigravity":
		return filepath.Join(home, ".gemini", "config", "mcp_config.json"), nil
	case "grok":
		return filepath.Join(home, ".grok", "config.toml"), nil
	case "opencode", "kilocode":
		base := s.ConfigDirs["xdg"]
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		if target == "kilocode" {
			return jsoncPath("kilo", filepath.Join(base, "kilo"))
		}
		return jsoncPath("opencode", filepath.Join(base, "opencode"))
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

// jsoncPath resolves the config file of an OpenCode-format client. These clients read
// <name>.json and <name>.jsonc from every dir and merge them, so an existing file is
// reused wherever it is, and a second one is never created beside it: entries would
// mask each other. More than one existing file needs consolidating by the user.
func jsoncPath(name string, dirs ...string) (string, error) {
	var existing []string
	for _, dir := range dirs {
		for _, ext := range []string{".jsonc", ".json"} {
			path := filepath.Join(dir, name+ext)
			if _, err := os.Lstat(path); err == nil {
				existing = append(existing, path)
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	if len(existing) > 1 {
		return "", fmt.Errorf("%s all exist; consolidate them into one file before MCP sync", strings.Join(existing, " and "))
	}
	if len(existing) == 1 {
		return existing[0], nil
	}
	// OpenCode's docs create opencode.json, Kilo's create kilo.jsonc.
	if name == "kilo" {
		return filepath.Join(dirs[0], "kilo.jsonc"), nil
	}
	return filepath.Join(dirs[0], name+".json"), nil
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

// AccountPaths is ClientPaths for the source's accounts. A project has none: every
// account of an Agent reads the same project files.
func (s *Service) AccountPaths(accounts map[string]Account) map[string]string {
	out := map[string]string{}
	if s.ProjectRoot != "" {
		return out
	}
	scoped := s.withAccounts(accounts)
	for name := range accounts {
		account, agent := scoped.forTarget(name)
		if path, err := account.nativePath(agent); err == nil {
			out[name] = path
		}
	}
	return out
}

// ConfiguredClientPaths names each target's destination, accounts included, for dashboard labels.
func (s *Service) ConfiguredClientPaths(source *Source) map[string]string {
	paths := s.withAccounts(source.Accounts).ClientPaths()
	maps.Copy(paths, s.AccountPaths(source.Accounts))
	return paths
}

// withAccounts keeps account-aware resolution consistent without changing the caller's scope.
func (s *Service) withAccounts(accounts map[string]Account) *Service {
	scoped := *s
	scoped.accounts = accounts
	return &scoped
}

var accountEnv = map[string]string{"codex": "CODEX_HOME", "claude": "CLAUDE_CONFIG_DIR", "pi": "PI_CODING_AGENT_DIR", "omp": "PI_CODING_AGENT_DIR"}

// configDir keeps an account shell from redirecting the plain Agent to that account.
// Account-scoped services still use their explicit directory; unrelated overrides stand.
func (s *Service) configDir(agent string) (string, string) {
	dir := s.ConfigDirs[agent]
	if dir != "" && s.account == "" {
		for _, name := range sortedKeys(s.accounts) {
			account := s.accounts[name]
			if account.Agent == agent && sameDir(dir, account.Dir) {
				return "", name
			}
		}
	}
	return dir, ""
}

func sameDir(a, b string) bool {
	// Filesystem identity also recognizes case aliases on case-insensitive volumes.
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	if aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo) {
		return true
	}
	resolve := func(path string) string {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return real
		}
		return filepath.Clean(path)
	}
	return resolve(a) == resolve(b)
}

// DetectedAccounts lists the accounts whose config directory exists.
func DetectedAccounts(accounts map[string]Account) []string {
	out := []string{}
	for _, name := range sortedKeys(accounts) {
		if info, err := os.Stat(accounts[name].Dir); err == nil && info.IsDir() {
			out = append(out, name)
		}
	}
	return out
}

// ConfigDirsFromEnv reads the Agent directory overrides Agents themselves honor.
func ConfigDirsFromEnv() map[string]string {
	dirs := map[string]string{}
	for key, env := range map[string]string{"pi": "PI_CODING_AGENT_DIR", "omp": "PI_CODING_AGENT_DIR", "codex": "CODEX_HOME", "claude": "CLAUDE_CONFIG_DIR", "grok": "GROK_HOME", "copilot": "COPILOT_HOME", "cline": "CLINE_DIR", "cline-data": "CLINE_DATA_DIR", "cline-mcp": "CLINE_MCP_SETTINGS_PATH", "xdg": "XDG_CONFIG_HOME", "appdata": "APPDATA"} {
		value := strings.TrimSpace(os.Getenv(env))
		// The XDG spec says a relative XDG_CONFIG_HOME is invalid and must be ignored.
		if value == "" || (key == "xdg" || key == "appdata") && !filepath.IsAbs(value) {
			continue
		}
		dirs[key] = value
	}
	return dirs
}

// homeFolders are the Agents whose MCP file is not directly inside their own folder.
var homeFolders = map[string]string{"claude": ".claude", "junie": ".junie", "kiro": ".kiro", "cline": ".cline"}

// DetectedClients lists Agents that look installed: their native MCP file
// exists, or its own config directory does. A file placed directly in the home
// only counts when the file itself exists.
//
// A project's folders say nothing: .github, .vscode and .agents exist for unrelated
// reasons, and Claude's file sits in the root with no folder at all. There, an Agent
// counts when its project file exists or when it is installed for the user.
func (s *Service) DetectedClients(paths map[string]string) []string {
	if s.ProjectRoot != "" {
		user := *s
		user.ProjectRoot = ""
		installed := user.DetectedClients(user.ClientPaths())
		out := []string{}
		for _, target := range Targets {
			if paths[target] == "" {
				continue
			}
			if _, err := os.Lstat(paths[target]); err == nil || slices.Contains(installed, target) {
				out = append(out, target)
			}
		}
		return out
	}
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
		// The MCP file's folder, or the Agent's own home folder when that file sits deeper
		// or directly in the home. ~/.gemini is shared with Antigravity, so it proves nothing.
		dir := filepath.Dir(path)
		if marker := homeFolders[target]; marker != "" {
			dir = filepath.Join(home, marker)
		}
		if dir == filepath.Clean(home) || target == "gemini" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, target)
		}
	}
	return out
}
