package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Account is a target that is another config directory of a built-in Agent.
type Account struct {
	Agent string `json:"agent"`
	Dir   string `json:"configDir"`
}

var accountAgents = []string{"claude", "codex", "pi", "omp"}

func (s *Service) lookupAccount(target string) (Account, bool) {
	if target == "git" {
		return Account{}, false // The Git hooks binding always names the native destination.
	}
	a, ok := s.Accounts[target]
	return a, s.ProjectRoot == "" && ok && a.Dir != "" && slices.Contains(accountAgents, a.Agent)
}

func (s *Service) agentOf(target string) string {
	if a, ok := s.lookupAccount(target); ok {
		return a.Agent
	}
	return target
}

func (s *Service) forTarget(target string) (*Service, string) {
	if a, ok := s.lookupAccount(target); ok {
		sc := *s
		sc.scopedAccount = target
		return &sc, a.Agent
	}
	return s, target
}

func (s *Service) accountAgents() map[string]string {
	out := map[string]string{}
	for key := range s.Accounts {
		if a, ok := s.lookupAccount(key); ok {
			out[key] = a.Agent
		}
	}
	return out
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func samePath(a, b string) bool { return canonicalPath(a) == canonicalPath(b) }

func (s *Service) envShadow(agent string) string {
	if s.ProjectRoot != "" || s.ConfigDirs[agent] == "" {
		return ""
	}
	for _, key := range sortedKeys(s.Accounts) {
		if a, ok := s.lookupAccount(key); ok && a.Agent == agent && samePath(s.ConfigDirs[agent], a.Dir) {
			return key
		}
	}
	return ""
}

func (s *Service) accountMissing(target string) bool {
	a, ok := s.lookupAccount(target)
	if !ok {
		return false
	}
	info, err := os.Stat(a.Dir)
	return err != nil || !info.IsDir()
}

func (s *Service) validateEntry(name string, e Entry, root string) error {
	scope := validateGlobal
	if root != "" || s.ProjectRoot != "" {
		scope = validateProject
	}
	return e.validate(name, s.accountAgents(), scope)
}

// parkReason classifies outputs this scope cannot safely plan. Path security
// failures inside the current base still reach the planner and remain errors.
func (s *Service) parkReason(r record) string {
	if r.Target == "git" {
		return "" // Git destinations and unavailable roots use their own ownership checks.
	}
	sc := s
	if r.Root != "" {
		sc = s.scoped(r.Root)
	}
	agent := sc.agentOf(r.Target)
	if _, ok := targetDef(agent); !ok {
		return fmt.Sprintf("target %s is not declared in targets", r.Target)
	}
	if sc.accountMissing(r.Target) {
		return fmt.Sprintf("config_dir %s does not exist on this machine", sc.Accounts[r.Target].Dir)
	}
	sc, agent = sc.forTarget(r.Target)
	dir, err := sc.configDir(agent)
	if err != nil {
		return ""
	}
	if !samePath(recordDir(r), dir) {
		reason := fmt.Sprintf("%s now resolves to %s", agent, dir)
		if sc.scopedAccount == "" && sc.ConfigDirs[agent] != "" && sc.envShadow(agent) == "" {
			reason += " (" + configDirEnv[agent] + ")"
		}
		return reason
	}
	return ""
}

// recordDir identifies the previous home for warnings, including script outputs.
func recordDir(r record) string {
	dir := filepath.Dir(r.Path)
	if r.Event == "" && filepath.Base(filepath.Dir(dir)) == "skillshare" {
		return filepath.Dir(filepath.Dir(filepath.Dir(dir)))
	}
	if r.Event == "" && (filepath.Base(dir) == "extensions" || filepath.Base(dir) == "plugins" || filepath.Base(dir) == "hooks") {
		return filepath.Dir(dir)
	}
	return dir
}

func (s *Service) targetDefs() []TargetDef {
	out := slices.Clone(Targets)
	for _, key := range sortedKeys(s.accountAgents()) {
		def, _ := targetDef(s.agentOf(key))
		def.Agent, def.Name = def.Name, key
		out = append(out, def)
	}
	return out
}

func (s *Service) targetKey(agent string) string {
	if s.scopedAccount != "" {
		return s.scopedAccount
	}
	return agent
}

// checkDesiredHomes includes both rendered bindings and old outputs scheduled
// for cleanup, since their lexical paths can now alias another target's home.
func (s *Service) checkDesiredHomes(d *desired) error {
	type claim struct{ target, root string }
	owners := map[string]claim{}
	check := func(target, root string) error {
		if target == "git" {
			return nil
		}
		sc := s
		if root != "" {
			sc = s.scoped(root)
		}
		sc, agent := sc.forTarget(target)
		dir, err := sc.configDir(agent)
		if err != nil {
			return err
		}
		dir = canonicalPath(dir)
		next := claim{target, root}
		if previous, ok := owners[dir]; ok && previous != next {
			label := func(c claim) string {
				if c.root != "" {
					return c.target + " in project " + c.root
				}
				return c.target
			}
			return fmt.Errorf("hooks: %s and %s both write %s; give each account its own config_dir", label(previous), label(next), dir)
		}
		owners[dir] = next
		return nil
	}
	for _, path := range sortedKeys(d.targets) {
		if err := check(d.targets[path], d.roots[path]); err != nil {
			return err
		}
		for _, w := range d.elements[path] {
			if err := check(w.target, w.root); err != nil {
				return err
			}
		}
	}
	for _, path := range sortedKeys(d.files) {
		w := d.files[path]
		if err := check(w.target, w.root); err != nil {
			return err
		}
	}
	return nil
}
