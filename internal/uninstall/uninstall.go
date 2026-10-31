// Package uninstall moves resolved skills, tracked repos and agents out of
// their source directory into the trash and removes what the install recorded
// about them, keeping a copy of that record beside the trash entry so restore
// can put it back. Callers resolve names and report the results.
package uninstall

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	gosync "sync"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/trash"
)

// Item is one resolved skill, group directory or tracked repo.
type Item struct {
	Name      string // source-relative slash path; its metadata key
	Path      string // logical path below the source
	TrashName string // name of the trash entry, which decides where restore puts it
	Repo      bool   // tracked repo: refused while dirty unless forced
	Gitignore string // .gitignore entry to drop once removed; "" for none
}

// Options describes the source the items live in.
type Options struct {
	SourceDir    string
	Follow       *sourcewalk.Follow
	TrashDir     string
	Store        *install.MetadataStore // pruned and saved to SourceDir
	GitignoreDir string                 // "" skips .gitignore cleanup
	Force        bool                   // skip the dirty check of tracked repos
}

// ErrDirty refuses a tracked repo with uncommitted changes.
var ErrDirty = errors.New("uncommitted changes")

// ErrInsideRepo refuses a skill or folder inside a tracked repo's checkout.
var ErrInsideRepo = errors.New("inside a tracked repo; uninstall the repo instead")

// StatusError refuses a tracked repo whose git status could not be read.
type StatusError struct{ Err error }

func (e *StatusError) Error() string { return fmt.Sprintf("failed to check git status: %v", e.Err) }
func (e *StatusError) Unwrap() error { return e.Err }

// TrashError reports an item that passed its checks but could not be moved.
type TrashError struct{ Err error }

func (e *TrashError) Error() string { return fmt.Sprintf("failed to move to trash: %v", e.Err) }
func (e *TrashError) Unwrap() error { return e.Err }

// Result is the outcome for one item; a nil Err means it is in the trash.
type Result struct {
	Item Item
	Err  error
}

// Outcome holds the per-item results in input order and the cleanup steps
// that failed after items were already removed.
type Outcome struct {
	Results      []Result
	GitignoreErr error
	SaveErr      error
}

// Preflight reports why each item cannot be uninstalled, without changing
// anything and regardless of Force, so a caller can warn or confirm first.
// The error is a move-out refusal (including ErrInsideRepo), ErrDirty or a
// *StatusError. Move-out comes first: a linked folder is never told to retry
// with force.
func Preflight(items []Item, o Options) []error {
	errs := make([]error, len(items))
	const maxDirtyWorkers = 8
	sem := make(chan struct{}, maxDirtyWorkers)
	var wg gosync.WaitGroup
	for i, item := range items {
		if errs[i] = checkMoveOut(item, o); errs[i] != nil || !item.Repo {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			errs[i] = checkDirty(item.Path)
		}()
	}
	wg.Wait()
	return errs
}

func checkDirty(repoPath string) error {
	dirty, err := git.IsDirty(repoPath)
	if err != nil {
		return &StatusError{Err: err}
	}
	if dirty {
		return ErrDirty
	}
	return nil
}

// Run uninstalls the items in order: move-out check, dirty check for tracked
// repos unless forced, move to trash. For everything that was removed it then
// drops the .gitignore entries and prunes the metadata store.
func Run(items []Item, o Options) Outcome {
	out := Outcome{Results: make([]Result, len(items))}
	names := map[string]bool{}
	var gitignore []string
	for i, item := range items {
		out.Results[i] = Result{Item: item, Err: moveOut(item, o)}
		if out.Results[i].Err != nil {
			continue
		}
		names[item.Name] = true
		if item.Gitignore != "" {
			gitignore = append(gitignore, item.Gitignore)
		}
	}
	if len(names) == 0 {
		return out
	}

	if o.GitignoreDir != "" && len(gitignore) > 0 {
		_, out.GitignoreErr = install.RemoveFromGitIgnoreBatch(o.GitignoreDir, gitignore)
	}
	o.Store.RemoveByNames(names)
	out.SaveErr = o.Store.Save(o.SourceDir)
	return out
}

// checkMoveOut refuses an item that may not leave the source: one behind a
// link the policy does not allow, or one inside a tracked repo's checkout.
// Force skips neither.
func checkMoveOut(item Item, o Options) error {
	if err := sourcefs.CheckSkillMoveOut(o.SourceDir, item.Path, o.Follow); err != nil {
		return err
	}
	rel, err := filepath.Rel(o.SourceDir, item.Path)
	if err != nil {
		return nil
	}
	// A tracked repo is a _-prefixed git checkout; removing part of it would
	// leave it dirty for update. A followed source link is the user's own
	// checkout, whose skills may go one at a time.
	if _, inside := install.TrackedAncestor(o.SourceDir, filepath.ToSlash(rel), o.Follow); inside {
		return ErrInsideRepo
	}
	return nil
}

func moveOut(item Item, o Options) error {
	if err := checkMoveOut(item, o); err != nil {
		return err
	}
	if item.Repo && !o.Force {
		if err := checkDirty(item.Path); err != nil {
			return err
		}
	}
	saved := o.Store.Extract(item.Name)
	if item.TrashName != item.Name {
		saved.MovePath(item.Name, item.TrashName) // restore puts it at TrashName
	}
	trashPath, err := trash.MoveToTrash(item.Path, item.TrashName, o.TrashDir)
	if err != nil {
		return &TrashError{Err: err}
	}
	// On failure the item reports an error and keeps its metadata, so a
	// restore from the trash still finds it in the store.
	if err := saveMetadata(trashPath, saved); err != nil {
		return &TrashError{Err: err}
	}
	return nil
}

// Agent is one resolved agent file.
type Agent struct {
	Name string // source-relative slash path without ".md"; its metadata key
	File string
}

// Agents moves each agent file, with its legacy sidecar, to the agent trash
// and removes its metadata entry. errs holds one entry per agent in input
// order. store may be nil; it is saved to sourceDir when anything was removed.
func Agents(agents []Agent, sourceDir, trashDir string, store *install.MetadataStore) (errs []error, saveErr error) {
	errs = make([]error, len(agents))
	removed := false
	for i, a := range agents {
		sidecar := filepath.Join(filepath.Dir(a.File), filepath.Base(a.Name)+install.MetaFileName)
		saved := install.NewMetadataStore()
		if store != nil && store.Has(a.Name) {
			saved.Set(a.Name, store.Get(a.Name))
		}
		trashPath, err := trash.MoveAgentToTrash(a.File, sidecar, a.Name, trashDir)
		if errs[i] = err; err != nil {
			continue
		}
		if errs[i] = saveMetadata(trashPath, saved); errs[i] != nil {
			continue
		}
		removed = true
		if store != nil {
			store.Remove(a.Name)
		}
	}
	if removed && store != nil {
		saveErr = store.Save(sourceDir)
	}
	return errs, saveErr
}

// Restore moves a trashed skill, folder or tracked repo back into destDir and
// records again what its uninstall removed from the store. store may be nil;
// it is then loaded from destDir.
func Restore(entry *trash.TrashEntry, destDir string, follow *sourcewalk.Follow, store *install.MetadataStore) error {
	if err := trash.Restore(entry, destDir, follow); err != nil {
		return err
	}
	return restoreMetadata(entry.Path, destDir, store)
}

// RestoreAgent is Restore for a trashed agent.
func RestoreAgent(entry *trash.TrashEntry, destDir string, store *install.MetadataStore) error {
	if err := trash.RestoreAgent(entry, destDir); err != nil {
		return err
	}
	return restoreMetadata(entry.Path, destDir, store)
}

func saveMetadata(trashPath string, saved *install.MetadataStore) error {
	if saved.IsEmpty() {
		return nil
	}
	data, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if err := os.WriteFile(trash.MetadataFile(trashPath), data, 0o644); err != nil {
		return fmt.Errorf("moved to trash but failed to save its metadata: %w", err)
	}
	return nil
}

func restoreMetadata(trashPath, sourceDir string, store *install.MetadataStore) error {
	file := trash.MetadataFile(trashPath)
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil // trashed as a local item or before metadata was kept
	}
	if err != nil {
		return fmt.Errorf("restored but failed to read its metadata: %w", err)
	}
	var saved install.MetadataStore
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("restored but failed to parse its metadata: %w", err)
	}
	if store == nil {
		if store, err = install.LoadMetadata(sourceDir); err != nil {
			return fmt.Errorf("restored but failed to load metadata: %w", err)
		}
	}
	store.Merge(&saved)
	if err := store.Save(sourceDir); err != nil {
		return fmt.Errorf("restored but failed to save its metadata: %w", err)
	}
	return os.Remove(file)
}
