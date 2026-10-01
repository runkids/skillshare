package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// prepareGitWrite holds Git's native lock through the final reread, backup,
// journal and rename. Git performs include edits on a private staged file, so
// concurrent config writers cannot slip between the revision check and write.
func (f *filePlan) prepareGitWrite(newFile bool) (func() error, func(), error) {
	if err := checkPath(f.base, f.path); err != nil {
		return nil, nil, err
	}
	f.gitCreatedDirs = missingDirs(filepath.Dir(f.path))
	if err := os.MkdirAll(filepath.Dir(f.path), 0755); err != nil {
		return nil, nil, err
	}
	lock, err := os.OpenFile(f.path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, nil, fmt.Errorf("Git config lock unavailable: %w", err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_ = lock.Close()
			_ = os.Remove(f.path + ".lock")
		}
	}
	fail := func(err error) (func() error, func(), error) { release(); return nil, nil, err }
	if _, _, _, _, err := f.refresh(); err != nil {
		return fail(err)
	}
	if f.gitGuard != nil {
		if err := f.gitGuard.check(); err != nil {
			return fail(err)
		}
	}
	data, exists, mode, err := safeRead(f.path)
	if err != nil {
		return fail(err)
	}
	if !exists {
		mode = 0644
	}
	var after []byte
	if f.include != nil {
		stage, err := os.CreateTemp(filepath.Dir(f.path), ".hooks-config-*")
		if err != nil {
			return fail(err)
		}
		stagePath := stage.Name()
		defer os.Remove(stagePath)
		if _, err = stage.Write(data); err != nil {
			_ = stage.Close()
			return fail(err)
		}
		if err = stage.Close(); err != nil {
			return fail(err)
		}
		args := []string{"config", "--file", stagePath}
		if f.include.Add {
			args = append(args, "--add", "include.path", f.include.Value)
		} else {
			args = append(args, "--fixed-value", "--unset", "include.path", f.include.Value)
		}
		if _, err = f.gitService.gitRunner().Run("", nil, args...); err != nil {
			return fail(fmt.Errorf("Git include edit failed: %w", err))
		}
		after, err = os.ReadFile(stagePath)
		if err != nil {
			return fail(err)
		}
		if !f.include.Add && newFile && emptyGitConfig(after) {
			f.remove = true
		}
	} else if f.gitSection.Restore {
		after, err = f.gitService.restoreGitSection(data, *f.gitSection)
		if err != nil {
			return fail(err)
		}
	} else {
		var fragments []gitSectionFragment
		after, fragments, err = f.gitService.removeGitSection(data, f.gitSection.Name)
		if err != nil {
			return fail(err)
		}
		f.gitSection.Fragments = fragments
	}
	if f.gitSection != nil {
		rows, err := f.gitService.gitParseBytes(after)
		if err != nil {
			return fail(err)
		}
		f.gitSectionAfter = gitValuesDigest(sectionRows(rows, f.gitSection.Name, true))
	}
	commit := func() error {
		if f.remove {
			return os.Remove(f.path)
		}
		if err := lock.Chmod(mode.Perm()); err != nil {
			return err
		}
		if _, err := lock.Write(after); err != nil {
			return err
		}
		if err := lock.Sync(); err != nil {
			return err
		}
		if err := lock.Close(); err != nil {
			return err
		}
		return os.Rename(f.path+".lock", f.path)
	}
	return commit, release, nil
}

func emptyGitConfig(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "[include]" {
			return false
		}
	}
	return true
}

func gitWritePriority(f *filePlan) int {
	if f.target != "git" {
		return 0
	}
	if f.include != nil {
		if f.include.Add {
			return 10
		}
		return -30
	}
	if f.gitSection != nil {
		return -10
	}
	if filepath.Base(f.path) == "hooks.gitconfig" {
		if f.remove {
			return 20
		}
		return 0
	}
	if f.remove {
		return 30
	}
	return -20
}

const (
	kindGitInclude = "gitinclude"
	kindGitSection = "gitsection"
)

type gitIncludeOp struct {
	Value string `json:"value"`
	Add   bool   `json:"add"`
}

type gitSectionFragment struct {
	Text     string `json:"text"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}
type gitSectionOp struct {
	Name      string               `json:"name"`
	Fragments []gitSectionFragment `json:"fragments"`
	Restore   bool                 `json:"restore,omitempty"`
}

func (f *filePlan) refreshGit() ([]byte, bool, os.FileMode, *nativeDoc, error) {
	if err := checkPath(f.base, f.path); err != nil {
		return nil, false, 0, nil, err
	}
	_, exists, mode, err := safeRead(f.path)
	if err != nil {
		return nil, false, 0, nil, err
	}
	rows, err := f.gitService.gitFileIncludes(f.path)
	if err != nil {
		return nil, false, 0, nil, err
	}
	if gitValuesDigest(rows) != f.section {
		return nil, false, 0, nil, fmt.Errorf("Git include values changed since preview: %w", ErrStaleRevision)
	}
	if !exists {
		mode = 0644
	}
	return f.before, exists, mode, nil, nil
}

func gitIncludePlan(s *Service, d gitDestination, rows []gitConfigValue, value string, add bool) *filePlan {
	f := &filePlan{path: d.includeTarget, target: "git", root: d.root, base: filepath.Dir(d.includeTarget), kind: kindGitInclude, mode: 0644, section: gitValuesDigest(rows), gitService: s}
	_, f.exists, _, _ = safeRead(d.includeTarget)
	for _, row := range rows {
		f.before = append(f.before, []byte(includeLines(row.Value))...)
	}
	f.after = append([]byte(nil), f.before...)
	if value != "" {
		f.include = &gitIncludeOp{Value: value, Add: add}
		if add {
			f.after = append(f.after, []byte(includeLines(value))...)
		} else {
			f.after = nil
			for _, row := range rows {
				if row.Key != "include.path" || row.Value != value {
					f.after = append(f.after, []byte(includeLines(row.Value))...)
				}
			}
		}
	}
	return f
}
