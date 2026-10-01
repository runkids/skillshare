package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type gitDestination struct {
	root, identity, base, hooksFile, includeTarget, includeValue string
}

// canonicalPath also canonicalizes existing ancestors of not-yet-created files.
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent, rest := abs, []string{}
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, rest[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		rest = append(rest, filepath.Base(parent))
		parent = next
	}
}

func (s *Service) gitDestination(root string) (gitDestination, error) {
	d := gitDestination{root: root}
	if s.ProjectRoot != "" {
		out, err := s.gitRunner().Run(s.ProjectRoot, nil, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
		if err != nil {
			return d, fmt.Errorf("project root is missing or is not a non-bare Git repository")
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) != 2 {
			return d, fmt.Errorf("Git did not resolve a repository top level and common directory")
		}
		top, err := canonicalPath(lines[0])
		if err != nil {
			return d, err
		}
		actual, err := canonicalPath(s.ProjectRoot)
		if err != nil {
			return d, err
		}
		if top != actual {
			return d, fmt.Errorf("project root must be the Git repository top level")
		}
		d.base, err = canonicalPath(lines[1])
		if err != nil {
			return d, err
		}
		d.identity = d.base
		d.hooksFile = filepath.Join(d.base, "skillshare", "hooks.gitconfig")
		d.includeTarget, d.includeValue = filepath.Join(d.base, "config"), "skillshare/hooks.gitconfig"
		return d, nil
	}
	home, err := s.home()
	if err != nil {
		return d, err
	}
	xdg := s.ConfigDirs["xdg"]
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	d.base, err = canonicalPath(filepath.Join(xdg, "git"))
	if err != nil {
		return d, err
	}
	d.identity = d.base
	d.hooksFile = filepath.Join(d.base, "skillshare", "hooks.gitconfig")
	d.includeTarget = s.GitGlobalConfig
	if d.includeTarget == "" {
		d.includeTarget = filepath.Join(home, ".gitconfig")
		if _, err := os.Lstat(d.includeTarget); os.IsNotExist(err) {
			if x := filepath.Join(xdg, "git", "config"); pathExists(x) {
				d.includeTarget = x
			}
		}
	}
	if !filepath.IsAbs(d.includeTarget) {
		return d, fmt.Errorf("GIT_CONFIG_GLOBAL must be an absolute file path")
	}
	// Resolve the parent, never the final symlink into permission to write it.
	parent, err := canonicalPath(filepath.Dir(d.includeTarget))
	if err != nil {
		return d, err
	}
	d.includeTarget = filepath.Join(parent, filepath.Base(d.includeTarget))
	d.includeValue = filepath.ToSlash(d.hooksFile)
	canonicalHome, err := canonicalPath(home)
	if err != nil {
		return d, err
	}
	if rel, err := filepath.Rel(canonicalHome, d.hooksFile); err == nil && filepath.IsLocal(rel) {
		d.includeValue = "~/" + filepath.ToSlash(rel)
	}
	return d, nil
}

func pathExists(path string) bool { _, err := os.Lstat(path); return err == nil }
