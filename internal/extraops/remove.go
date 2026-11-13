// Package extraops owns extras lifecycle operations shared by CLI, TUI and API.
package extraops

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

type RemoveRequest struct {
	Extras                                   []config.ExtraConfig
	Name, SourceDir, ExtrasRoot, ProjectRoot string
	TargetPath                               func(string) string
	Load                                     func() ([]config.ExtraConfig, error)       // required remotely; fresh read after root lock
	Save                                     func(remaining []config.ExtraConfig) error // fresh merge; remove only unchanged own entry
}
type RemoveResult struct {
	Remaining                 []config.ExtraConfig
	Restored                  int
	Remote, Commit            string
	DeletedDir, KeptEditedDir bool
	KeptOutputs, Warnings     []string
}

var lockExtrasRoot = install.LockExtrasRoot
var restoreFileTargets = syncpkg.RestoreExtraFileTargets
var removeExtraDir = os.RemoveAll

func namedExtra(entries []config.ExtraConfig, name string) (int, config.ExtraConfig) {
	for i, e := range entries {
		if e.Name == name {
			return i, e
		}
	}
	return -1, config.ExtraConfig{}
}

// Remove owns the remote root lock. Its caller must not acquire it again.
func Remove(req RemoveRequest) (result RemoveResult, retErr error) {
	if err := config.ValidateExtraName(req.Name); err != nil {
		return result, err
	}
	i, extra := namedExtra(req.Extras, req.Name)
	if i < 0 {
		return result, fmt.Errorf("extra %q not found", req.Name)
	}
	if req.Save == nil || req.TargetPath == nil {
		return result, errors.New("extra removal requires save and target callbacks")
	}
	if extra.Remote != "" {
		if req.Load == nil {
			return result, errors.New("remote extra removal requires a fresh load callback")
		}
		unlock, err := lockExtrasRoot(req.ExtrasRoot)
		if err != nil {
			return result, err
		}
		defer unlock()
		entries, err := req.Load()
		if err != nil {
			return result, err
		}
		index, fresh := namedExtra(entries, req.Name)
		if index < 0 || fresh.Remote != extra.Remote || fresh.Branch != extra.Branch || fresh.Source != "" {
			return result, fmt.Errorf("extra %q changed; retry removal", req.Name)
		}
		req.Extras, i, extra = entries, index, fresh
		expected := filepath.Join(req.ExtrasRoot, req.Name)
		if !utils.PathsEqual(expected, req.SourceDir) {
			return result, errors.New("remote extra removal source does not match its managed directory")
		}
		state := install.InspectRemoteExtra(req.ExtrasRoot, req.Name)
		if state.Err != nil {
			return result, state.Err
		}
		if state.Interrupted {
			return result, fmt.Errorf("extra %q is interrupted; reinstall it before removal", req.Name)
		}
		result.Remote, result.Commit = extra.Remote, state.Commit
	}
	// Local extras retain their existing single-file-only cleanup behavior.
	var err error
	result.Restored, err = restoreFileTargets(extra, req.SourceDir, req.TargetPath)
	if err != nil {
		return result, fmt.Errorf("target cleanup failed; source and config retained: %w", err)
	}
	if extra.Remote != "" && extra.File == "" {
		for _, target := range extra.Targets {
			resolved := req.TargetPath(target.Path)
			if target.EffectiveMode() == "copy" || target.Extension != "" {
				result.KeptOutputs = append(result.KeptOutputs, resolved)
				continue
			}
			_, kept, errs := syncpkg.ClearExtraTarget("", req.SourceDir, resolved, target.As, target.EffectiveMode(), nil)
			if len(errs) > 0 {
				return result, fmt.Errorf("target cleanup failed; source and config retained: %s", strings.Join(errs, "; "))
			}
			if kept {
				result.KeptOutputs = append(result.KeptOutputs, resolved)
			}
		}
	}
	result.Remaining = append(slices.Clone(req.Extras[:i]), req.Extras[i+1:]...)
	if extra.Remote == "" {
		return result, req.Save(result.Remaining)
	}
	if len(result.KeptOutputs) > 0 {
		result.Warnings = append(result.Warnings, "Copied, transformed or unowned target outputs were left in place")
	}
	return removeRemote(req, extra, result)
}

type removalState struct {
	path            string
	before, written install.ExtraStateSnapshot
	touched         bool
}

func removeRemote(req RemoveRequest, extra config.ExtraConfig, result RemoveResult) (RemoveResult, error) {
	modified, entry, err := install.ExtraLocallyModified(req.ExtrasRoot, req.Name)
	if err != nil {
		return result, err
	}
	if entry != nil {
		result.Commit = entry.Commit
	}
	configPath := config.ReadOnlyConfigPath()
	if req.ProjectRoot != "" {
		configPath = config.ProjectConfigPath(req.ProjectRoot)
	}
	realConfig, err := utils.ConfigWritePath(configPath, req.ProjectRoot != "")
	if err != nil {
		return result, err
	}
	paths := []string{realConfig, filepath.Join(req.ExtrasRoot, install.MetadataFileName)}
	lockDir, ignoreDir, ignoreEntry := "", "", ""
	if req.ProjectRoot != "" {
		lockDir = projectdir.Resolve(req.ProjectRoot)
		paths = append(paths, filepath.Join(lockDir, install.LockFileName))
		var prefix string
		ignoreDir, prefix = projectdir.GitignoreTarget(req.ProjectRoot, req.ExtrasRoot)
		if ignoreDir != "" {
			ignoreEntry = path.Join(prefix, req.Name) + "/"
			paths = append(paths, filepath.Join(ignoreDir, ".gitignore"))
		}
	}
	states := make([]removalState, len(paths))
	for i, p := range paths {
		before, err := install.SnapshotExtraState(p)
		if err != nil {
			return result, err
		}
		states[i] = removalState{path: p, before: before}
	}
	backup, temp := "", ""
	if !modified {
		temp, err = os.MkdirTemp("", "skillshare-extra-remove-*")
		if err != nil {
			return result, err
		}
		backup = filepath.Join(temp, "source")
		if err := install.StageExtraTree(req.SourceDir, backup); err != nil {
			_ = os.RemoveAll(temp)
			return result, err
		}
	}
	keepBackup := false
	defer func() {
		if temp != "" && !keepBackup {
			_ = os.RemoveAll(temp)
		}
	}()
	capture := func(index int, write func() error) error {
		writeErr := write()
		// The merge refused before writing. Its fresh conflicting entry belongs
		// to another writer and must not become our rollback read-back.
		if index == 0 && errors.Is(writeErr, install.ErrExtraConfigNotWritten) {
			return writeErr
		}
		states[index].touched = true
		states[index].written, err = install.SnapshotExtraState(states[index].path)
		if err != nil {
			return fmt.Errorf("capture %s: %w", states[index].path, err)
		}
		return writeErr
	}
	sourceTouched := false
	rollback := func(cause error) (RemoveResult, error) {
		var errs []string
		for i := len(states) - 1; i >= 0; i-- {
			state := states[i]
			if !state.touched {
				continue
			}
			ok, err := install.RestoreExtraStateIfUnchanged(state.path, state.before, state.written)
			if err != nil {
				errs = append(errs, err.Error())
			} else if !ok {
				errs = append(errs, state.path+" changed by another writer and was retained")
			}
		}
		if sourceTouched {
			if err := os.RemoveAll(req.SourceDir); err == nil {
				if err := install.StageExtraTree(backup, req.SourceDir); err != nil {
					errs = append(errs, err.Error())
				}
			} else {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			keepBackup = backup != ""
			return result, fmt.Errorf("removing %s failed: %w; rollback incomplete: %s; source backup: %s", req.Name, cause, strings.Join(errs, "; "), backup)
		}
		return result, fmt.Errorf("removing %s failed; source and records restored: %w", req.Name, cause)
	}
	if err := capture(0, func() error { return req.Save(result.Remaining) }); err != nil {
		return rollback(err)
	}
	if err := capture(1, func() error {
		store, _, err := install.ReadMetadata(req.ExtrasRoot)
		if err != nil {
			return err
		}
		store.Remove(req.Name)
		return store.Save(req.ExtrasRoot)
	}); err != nil {
		return rollback(err)
	}
	if lockDir != "" {
		if err := capture(2, func() error {
			lock, err := install.LoadLock(lockDir)
			if err != nil {
				return err
			}
			lock.DropExtraPin(req.Name)
			return lock.Save(lockDir)
		}); err != nil {
			return rollback(err)
		}
	}
	if modified {
		result.KeptEditedDir = true
		return result, nil
	}
	if ignoreDir != "" {
		if err := capture(len(states)-1, func() error { _, err := install.RemoveFromGitIgnore(ignoreDir, ignoreEntry); return err }); err != nil {
			return rollback(err)
		}
	}
	// A managed destination link is never followed or deleted as fetched content.
	info, err := os.Lstat(req.SourceDir)
	if err != nil {
		return rollback(err)
	}
	if utils.IsLinkMode(req.SourceDir, info.Mode()) || !info.IsDir() {
		return rollback(fmt.Errorf("unsafe remote extra destination %s", req.SourceDir))
	}
	sourceTouched = true
	if err := removeExtraDir(req.SourceDir); err != nil {
		return rollback(err)
	}
	result.DeletedDir = true
	return result, nil
}

// WithoutUnchangedExtra freshly merges a removal without overwriting other
// entries or removing an own entry another writer has changed.
func WithoutUnchangedExtra(current []config.ExtraConfig, expected config.ExtraConfig) ([]config.ExtraConfig, error) {
	i, entry := namedExtra(current, expected.Name)
	if i < 0 || !reflect.DeepEqual(entry, expected) {
		return nil, fmt.Errorf("%w: extra %q changed; retry removal", install.ErrExtraConfigNotWritten, expected.Name)
	}
	return append(slices.Clone(current[:i]), current[i+1:]...), nil
}
