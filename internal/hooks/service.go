package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofrs/flock"
)

// Service is scoped to one Skillshare config; StateDir is shared across scopes so
// separate configs cannot silently take over each other's native registrations.
type Service struct {
	ConfigPath  string
	ProjectRoot string
	Home        string
	StateDir    string
	Platform    string
	// ConfigDirs contains explicitly resolved native Agent directory overrides.
	ConfigDirs    map[string]string
	Accounts      map[string]Account // global scope only
	scopedAccount string
	Git           GitRunner
	// GitGlobalConfig is the file override Git honors in GIT_CONFIG_GLOBAL.
	GitGlobalConfig string
}

// Mutation is shared by CLI and dashboard preview/save/sync flows. An empty
// mutation synchronizes the current source.
type Mutation struct {
	// Project is a root under the global config's hooks.projects. Set alone, it creates
	// an empty block; with Remove and no Name, it removes the block.
	Project string `json:"project,omitempty"`
	Name    string `json:"name,omitempty"`
	Entry   *Entry `json:"entry,omitempty"`
	Remove  bool   `json:"remove,omitempty"`
	// Replace explicitly takes over this entry's conflicting native outputs: an
	// identical unmanaged registration, an unmanaged file at an output path, or an
	// owned output edited outside Skillshare. It never overrides another config.
	Replace bool `json:"replace,omitempty"`
	// Adopt saves an imported entry as the owner of the native registrations it was
	// read from: identical unmanaged elements become its own without Replace, so the
	// next sync adopts them in place. It never takes over edited outputs or another
	// config's registrations.
	Adopt bool `json:"adopt,omitempty"`
	// Unmanage, with Remove and a Name, also forgets which native registrations the
	// hook wrote, so sync leaves them in place as the user's own hooks.
	Unmanage bool `json:"unmanage,omitempty"`
}

// Change deliberately carries no hook contents.
type Change struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	// Root is the hooks.projects root this change belongs to, empty for the config's own scope.
	Root    string `json:"root,omitempty"`
	Action  string `json:"action"` // add, update, remove, unchanged, adopt, release, conflict, inactive
	Message string `json:"message,omitempty"`
	// InactiveReason reports why generated Git output cannot run, independently of its file action.
	InactiveReason string `json:"inactiveReason,omitempty"`
	// Events names the native events this change writes into a shared hooks file.
	Events *EventChanges `json:"events,omitempty"`
}

// EventChanges is the event-level detail of a change to a shared hooks file: events
// the entry newly occupies, events whose registrations change, and events it leaves.
type EventChanges struct {
	Added   []string `json:"added,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// Plan is an optimistic-concurrency-protected preview.
type Plan struct {
	Revision string `json:"revision"`
	// Fingerprint identifies only what the preview shows about hooks: the proposed
	// entries and projects, replacements, native hook sections and files, ownership
	// and the resulting changes. Unrelated config or native settings leave it unchanged.
	Fingerprint string   `json:"fingerprint"`
	SourcePath  string   `json:"sourcePath"`
	Blocked     bool     `json:"blocked"`
	Changes     []Change `json:"changes"`
	// Warnings are advisory, such as event names an Agent does not document; they
	// never block a sync.
	Warnings []string `json:"warnings"`

	source     *Source
	state      ledger
	stateBytes []byte
	files      []*filePlan
	// adopted are the ledger keys an Adopt mutation claims, recorded even without sync.
	adopted     []string
	gitGuards   []*gitGuard
	gitInactive map[string]string
}

// FileDiff is one native file's text before and after a planned sync.
type FileDiff struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Root   string `json:"root,omitempty"`
	// Before is the current text, "" when the file is missing; After is exactly what
	// sync writes, "" when it removes the file.
	Before string `json:"before"`
	After  string `json:"after"`
}

// Files returns each native file the plan would change, with its full text.
func (p *Plan) Files() []FileDiff {
	out := []FileDiff{}
	for _, f := range p.files {
		if f.changed() && string(f.before) != string(f.after) {
			out = append(out, FileDiff{Target: f.target, Path: f.path, Root: f.root, Before: string(f.before), After: string(f.after)})
		}
	}
	return out
}

// Result reports applied files individually; a multi-file operation is not a
// filesystem transaction and may leave completed files when a later one fails.
type Result struct {
	Plan      *Plan    `json:"plan,omitempty"`
	Applied   []string `json:"applied"`
	BackupIDs []string `json:"backupIds"`
}

// SourceInfo is the declaration shown by List.
type SourceInfo struct {
	Path       string           `json:"path"`
	ConfigPath string           `json:"configPath"`
	Entries    map[string]Entry `json:"entries"`
	// Projects are roots a global config publishes into, keyed by absolute path.
	Projects map[string]Project `json:"projects,omitempty"`
}

// Backup is metadata for one native write.
type Backup struct {
	Root   string `json:"root,omitempty"`
	ID     string `json:"id"`
	Target string `json:"target"`
	Path   string `json:"path"`
	Time   string `json:"time,omitempty"`
}

// Unmanaged names native hooks no Skillshare config owns, including additional
// sources Skillshare never edits.
type Unmanaged struct {
	Project string   `json:"project,omitempty"`
	Target  string   `json:"target"`
	Path    string   `json:"path"`
	Names   []string `json:"names"`
}

// Inventory is everything the list views need.
type Inventory struct {
	Git          *GitInfo            `json:"git,omitempty"`
	ProjectGit   map[string]*GitInfo `json:"projectGit,omitempty"`
	Source       SourceInfo          `json:"source"`
	Targets      []TargetDef         `json:"targets"`
	Paths        map[string]string   `json:"paths"`
	Plan         *Plan               `json:"plan,omitempty"`
	PreviewError string              `json:"previewError,omitempty"`
	Backups      []Backup            `json:"backups"`
	Unmanaged    []Unmanaged         `json:"unmanaged"`
	// ProjectConfigs are hooks.projects roots that have their own Skillshare config;
	// the global config cannot manage them.
	ProjectConfigs []string `json:"projectConfigs"`
	// ProjectPaths are each hooks.projects root's native destinations.
	ProjectPaths map[string]map[string]string `json:"projectPaths"`
}

// ImportRequest reads native hooks without executing them. Content, when set,
// replaces reading the Agent's file in the current scope.
type ImportRequest struct {
	From string `json:"from"`
	// Root reads a hooks.projects root's native files instead of the current scope's.
	Root    string `json:"root,omitempty"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
}

// Candidate is one importable entry.
type Candidate struct {
	Name     string   `json:"name"`
	Entry    Entry    `json:"entry"`
	Problems []string `json:"problems"`
	Warnings []string `json:"warnings"`
}

// ConfigDirsFromEnv reads the Agent directory overrides Agents themselves honor.
func ConfigDirsFromEnv() map[string]string {
	dirs := map[string]string{}
	for key, env := range configDirEnv {
		value := strings.TrimSpace(os.Getenv(env))
		if value == "" || !filepath.IsAbs(value) {
			continue
		}
		dirs[key] = value
	}
	return dirs
}

// configDirEnv maps ConfigDirs keys to the environment variables Agents document.
var configDirEnv = map[string]string{
	"claude":  "CLAUDE_CONFIG_DIR",
	"codex":   "CODEX_HOME",
	"copilot": "COPILOT_HOME",
	"pi":      "PI_CODING_AGENT_DIR",
	"omp":     "PI_CODING_AGENT_DIR", // OMP is a Pi fork and reads the same variable.
	"xdg":     "XDG_CONFIG_HOME",
}

func (s *Service) home() (string, error) {
	if s.Home != "" {
		return s.Home, nil
	}
	return os.UserHomeDir()
}

// configDir is the Agent's own config directory in this scope. A project scope never
// falls back to a global path.
func (s *Service) configDir(target string) (string, error) {
	if target == "git" {
		d, err := s.gitDestination("")
		return d.base, err
	}
	if s.ProjectRoot != "" {
		dirs := map[string]string{"claude": ".claude", "codex": ".codex", "gemini": ".gemini", "qwen": ".qwen", "copilot": ".github", "cursor": ".cursor", "droid": ".factory", "antigravity": ".agents", "pi": ".pi", "omp": ".omp", "amp": ".amp", "opencode": ".opencode"}
		dir, ok := dirs[target]
		if !ok {
			return "", fmt.Errorf("unsupported hooks Agent %q", target)
		}
		return filepath.Join(s.ProjectRoot, dir), nil
	}
	if s.scopedAccount != "" {
		return s.Accounts[s.scopedAccount].Dir, nil
	}
	if dir := s.ConfigDirs[target]; dir != "" && s.envShadow(target) == "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%s config directory must be absolute", target)
		}
		return filepath.Clean(dir), nil
	}
	return s.defaultConfigDir(target)
}

func (s *Service) defaultConfigDir(target string) (string, error) {
	home, err := s.home()
	if err != nil {
		return "", err
	}
	xdg := s.ConfigDirs["xdg"]
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	switch target {
	case "claude", "codex", "gemini", "qwen", "copilot", "cursor":
		return filepath.Join(home, "."+target), nil
	case "droid":
		return filepath.Join(home, ".factory"), nil
	case "antigravity":
		return filepath.Join(home, ".gemini", "config"), nil
	case "pi":
		return filepath.Join(home, ".pi", "agent"), nil
	case "omp":
		return filepath.Join(home, ".omp", "agent"), nil
	case "amp":
		return filepath.Join(xdg, "amp"), nil
	case "opencode":
		return filepath.Join(xdg, "opencode"), nil
	}
	return "", fmt.Errorf("unsupported hooks Agent %q", target)
}

// nativePath is the shared hooks file of a command Agent, or the folder that holds
// Skillshare's files for Copilot and the code Agents.
func (s *Service) nativePath(target string) (string, error) {
	if target == "git" {
		d, err := s.gitDestination("")
		return d.hooksFile, err
	}
	dir, err := s.configDir(target)
	if err != nil {
		return "", err
	}
	switch target {
	case "claude", "gemini", "qwen":
		return filepath.Join(dir, "settings.json"), nil
	case "codex", "cursor", "droid", "antigravity":
		return filepath.Join(dir, "hooks.json"), nil
	case "copilot":
		return filepath.Join(dir, "hooks"), nil
	case "pi", "omp":
		return filepath.Join(dir, "extensions"), nil
	case "amp", "opencode":
		return filepath.Join(dir, "plugins"), nil
	}
	return "", fmt.Errorf("unsupported hooks Agent %q", target)
}

// codePath is the Skillshare-owned file of a code Agent or Copilot for one entry.
func (s *Service) codePath(target, entry string) (string, error) {
	dir, err := s.nativePath(target)
	if err != nil {
		return "", err
	}
	if target == "copilot" {
		return filepath.Join(dir, "skillshare-"+entry+".json"), nil
	}
	return filepath.Join(dir, "skillshare-"+entry+".ts"), nil
}

// scriptDir holds an entry's script files for a command Agent.
func (s *Service) scriptDir(target, entry string) (string, error) {
	if target == "git" {
		d, err := s.gitDestination("")
		return filepath.Join(d.base, "skillshare", "files", entry), err
	}
	dir, err := s.configDir(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hooks", "skillshare", entry), nil
}

// Paths exposes native destinations for the current scope.
func (s *Service) Paths() map[string]string {
	out := map[string]string{}
	for _, t := range Targets {
		if path, err := s.nativePath(t.Name); err == nil {
			out[t.Name] = path
		}
	}
	for key := range s.accountAgents() {
		sc, agent := s.forTarget(key)
		if path, err := sc.nativePath(agent); err == nil {
			out[key] = path
		}
	}
	return out
}

func (s *Service) platform() string {
	if s.Platform != "" {
		return s.Platform
	}
	return runtime.GOOS
}

func (s *Service) stateDir() string { return filepath.Join(s.StateDir, "hooks") }

func (s *Service) statePath() string { return filepath.Join(s.stateDir(), "state.json") }

func (s *Service) lock() (*flock.Flock, error) {
	if s.StateDir == "" {
		return nil, fmt.Errorf("hooks state directory is required")
	}
	if err := os.MkdirAll(s.stateDir(), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(s.stateDir(), "operation.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("another hooks operation is running; retry when it completes")
	}
	return lock, nil
}
