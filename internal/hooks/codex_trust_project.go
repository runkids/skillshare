package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
)

// Resolve both the home and an existing config.toml symlink before Codex is started.
func projectTrustDestinationInside(root, home string) bool {
	root = canonicalPath(root)
	rootInfo, err := os.Stat(root)
	if err != nil {
		return true
	}
	for _, path := range []string{canonicalPath(home), canonicalCodexHooksPath(filepath.Join(home, "config.toml"))} {
		for dir := path; ; dir = filepath.Dir(dir) {
			if info, err := os.Stat(dir); err == nil && os.SameFile(rootInfo, info) {
				return true
			}
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	return false
}

// A checkout has one native hooks file, but a distinct trust decision in each
// user's Codex home. Never resolve those homes from the checkout's config.
func (s *Service) projectTrustHomes(state ledger, owner string) (map[string]*codexHome, error) {
	path, err := s.nativePath("codex")
	if err != nil {
		return nil, err
	}
	records, owned := map[string]record{}, map[string]record{}
	for key, r := range state.Records {
		if r.Path != path || r.Target != "codex" {
			continue
		}
		records[key] = r
		if r.Owner == owner && r.Root == "" {
			owned[key] = r
		}
	}
	homes := map[string]*codexHome{}
	if len(owned) == 0 {
		return homes, nil
	}
	if canonicalPath(path) != filepath.Join(canonicalPath(s.ProjectRoot), ".codex", "hooks.json") {
		return nil, fmt.Errorf("project hook path redirects outside its native location; review in Codex /hooks")
	}
	global := *s
	global.ProjectRoot = ""
	if global.Accounts == nil {
		global.Accounts = map[string]Account{}
		if _, err := os.Stat(config.ReadOnlyConfigPath()); !os.IsNotExist(err) {
			cfg, err := config.LoadReadOnly()
			if err != nil {
				return nil, err
			}
			for name, target := range cfg.Targets {
				if target.Agent == "codex" && target.ConfigDir != "" {
					global.Accounts[name] = Account{Agent: "codex", Dir: target.ConfigDir}
				}
			}
		}
	}
	targets := []string{"codex"}
	for _, name := range sortedKeys(global.Accounts) {
		if a, ok := global.lookupAccount(name); ok && a.Agent == "codex" && name != "codex" {
			targets = append(targets, name)
		}
	}
	for _, target := range targets {
		sc, agent := global.forTarget(target)
		dir, err := sc.configDir(agent)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("%s: Codex home must be absolute", target)
		}
		key := canonicalPath(dir)
		if homes[key] == nil {
			homes[key] = &codexHome{target: target, path: path, home: dir, records: records, owned: owned}
		}
	}
	return homes, nil
}
