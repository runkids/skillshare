package install

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/projectdir"
	"skillshare/internal/utils"
)

var errSimulatedDeath = errors.New("simulated process death")

// ErrExtraConfigNotWritten marks a callback refusal known to precede its write.
// A refusing fresh merge must not capture another writer's bytes for rollback.
var ErrExtraConfigNotWritten = errors.New("extra config write refused")

var extraBoundary = func(string) error { return nil }
var extraRename = os.Rename
var extraWriteMarker = writeExtraJournal
var extraWriteMeta = func(store *MetadataStore, root string) error { return store.Save(root) }
var extraWriteLock = func(lock *Lock, dir string) error { return lock.Save(dir) }
var extraWriteIgnore = addGitIgnoreEntries
var extraCleanup = os.RemoveAll

type extraParticipant struct {
	path            string
	before, written extraFileSnapshot
	didWrite        bool
	uncertain       bool
}

func (p *extraParticipant) snapshot() error {
	var err error
	p.before, err = snapshotExtraFile(p.path)
	return err
}
func (p *extraParticipant) capture() error {
	var err error
	p.written, err = snapshotExtraFile(p.path)
	p.didWrite = err == nil
	p.uncertain = err != nil
	return err
}

func (p *extraParticipant) write(fn func() error) error {
	writeErr := fn()
	if errors.Is(writeErr, ErrExtraConfigNotWritten) {
		return writeErr
	}
	if err := p.capture(); err != nil {
		return fmt.Errorf("capture post-write state: %w", err)
	}
	return writeErr
}
func (p *extraParticipant) restore() (bool, error) {
	if !p.didWrite {
		return true, nil
	}
	return restoreExtraSnapshotIfUnchanged(p.path, p.before, p.written)
}

func extraIgnoreTarget(req RemoteExtraRequest) (string, []string) {
	if req.ProjectRoot == "" {
		return "", nil
	}
	dir, prefix := projectdir.GitignoreTarget(req.ProjectRoot, req.ExtrasRoot)
	if dir == "" {
		return "", nil
	}
	return dir, []string{path.Join(prefix, req.Name) + "/", path.Join(prefix, extraStagingDir) + "/", path.Join(prefix, MetadataFileName), path.Join(prefix, utils.ExtrasRootLockFileName)}
}

func publishRemoteExtra(req RemoteExtraRequest, candidate *extraCandidate) error {
	root, dest := req.ExtrasRoot, filepath.Join(req.ExtrasRoot, req.Name)
	stage := filepath.Join(root, extraStagingDir)
	incoming := filepath.Join(stage, "incoming-"+req.Name)
	previous := filepath.Join(stage, "previous-"+req.Name)
	marker := extraJournalPath(root, req.Name)
	ignoreDir, ignoreEntries := extraIgnoreTarget(req)
	configFile := extraParticipant{path: req.ConfigPath}
	metaFile := extraParticipant{path: filepath.Join(root, MetadataFileName)}
	lockFile := extraParticipant{}
	ignoreFile := extraParticipant{}
	participants := []*extraParticipant{&configFile, &metaFile}
	if req.ProjectRoot != "" {
		lockFile.path = filepath.Join(req.LockDir, LockFileName)
		participants = append(participants, &lockFile)
		if ignoreDir != "" {
			ignoreFile.path = filepath.Join(ignoreDir, ".gitignore")
			participants = append(participants, &ignoreFile)
		}
	}
	for _, p := range participants {
		if err := p.snapshot(); err != nil {
			return err
		}
	}
	var addedIgnore []string
	if ignoreDir != "" {
		for _, entry := range ignoreEntries {
			present, err := GitignoreContains(ignoreFile.path, entry)
			if err != nil {
				return err
			}
			if !present {
				addedIgnore = append(addedIgnore, entry)
			}
		}
	}
	var oldPin LockEntry
	var hadPin bool
	if req.ProjectRoot != "" {
		lock, err := LoadLock(req.LockDir)
		if err != nil {
			return err
		}
		oldPin, hadPin = lock.Extras[req.Name]
	}
	var old map[string]string
	_, err := os.Lstat(dest)
	hadPrevious := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if hadPrevious {
		old, err = fingerprintExtraTree(dest)
		if err != nil {
			return err
		}
	}
	candidateHash, err := fingerprintExtraTree(candidate.staged)
	if err != nil {
		return err
	}
	journal := extraJournal{Version: 1, Name: req.Name, Remote: req.RawSource, Branch: req.Branch, Commit: candidate.fetch.Commit, HadPrevious: hadPrevious, Old: old}
	if err := extraWriteMarker(root, journal); err != nil {
		return err
	}
	step := "marker"
	movedOld, installed := false, false
	rollback := func(cause error) error {
		if errors.Is(cause, errSimulatedDeath) {
			return fmt.Errorf("publishing %s interrupted at %s: %w", req.Name, step, cause)
		}
		var failures []string
		var preserved []string
		for _, p := range []*extraParticipant{&configFile, &lockFile, &ignoreFile, &metaFile} {
			if p.uncertain {
				failures = append(failures, "cannot compare "+p.path)
				continue
			}
			if p.path == "" || !p.didWrite {
				continue
			}
			restored, e := p.restore()
			if e != nil {
				failures = append(failures, e.Error())
				continue
			}
			if restored {
				continue
			}
			switch p {
			case &configFile:
				e = req.RevertConfig()
			case &lockFile:
				var l *Lock
				l, e = LoadLock(req.LockDir)
				if e == nil {
					if hadPin {
						l.SetExtraPin(req.Name, oldPin)
					} else {
						l.DropExtraPin(req.Name)
					}
					e = extraWriteLock(l, req.LockDir)
				}
			case &ignoreFile:
				// Staging and metadata entries are shared by other extras; a
				// concurrent writer may now rely on them.
				for _, entry := range addedIgnore {
					if entry == ignoreEntries[0] {
						_, e = RemoveFromGitIgnore(ignoreDir, entry)
					}
				}
			case &metaFile:
				e = fmt.Errorf("%s: metadata changed during publication", p.path)
			}
			if e != nil {
				failures = append(failures, e.Error())
			} else {
				preserved = append(preserved, p.path+" changed by another writer and was kept; only this install's change was reverted")
			}
		}
		canRestoreTree := true
		if installed {
			current, exists, e := existingExtraFingerprint(dest)
			if e != nil || !exists || !sameExtraFingerprint(current, candidateHash) {
				failures = append(failures, "live tree changed during publication")
				canRestoreTree = false
			} else if e := extraCleanup(dest); e != nil {
				failures = append(failures, e.Error())
				canRestoreTree = false
			}
		}
		if movedOld && canRestoreTree {
			current, exists, e := existingExtraFingerprint(previous)
			if e != nil || !exists || !sameExtraFingerprint(current, old) {
				failures = append(failures, "previous tree changed during publication")
			} else if e := extraRename(previous, dest); e != nil {
				failures = append(failures, e.Error())
			}
		}
		if e := extraCleanup(incoming); e != nil {
			failures = append(failures, e.Error())
		}
		if len(failures) == 0 {
			if e := os.Remove(marker); e != nil && !os.IsNotExist(e) {
				failures = append(failures, e.Error())
			}
		}
		if len(failures) != 0 {
			return fmt.Errorf("publishing %s failed at %s: %w; rollback incomplete, marker retained: %s; run skillshare check extras", req.Name, step, cause, strings.Join(failures, "; "))
		}
		if len(preserved) != 0 {
			return fmt.Errorf("publishing %s failed at %s: %w; %s", req.Name, step, cause, strings.Join(preserved, "; "))
		}
		return fmt.Errorf("publishing %s failed at %s: %w", req.Name, step, cause)
	}
	boundary := func(s string) error { step = s; return extraBoundary(s) }
	if err := boundary("marker"); err != nil {
		return rollback(err)
	}
	step = "incoming"
	if err := stageFetchedTree(candidate.staged, "", incoming); err != nil {
		return rollback(err)
	}
	if err := boundary("incoming"); err != nil {
		return rollback(err)
	}
	if ignoreDir != "" {
		step = "ignore"
		if err := ignoreFile.write(func() error { return extraWriteIgnore(ignoreDir, ignoreEntries) }); err != nil {
			return rollback(err)
		}
	}
	if err := boundary("ignore"); err != nil {
		return rollback(err)
	}
	if hadPrevious {
		step = "previous"
		if err := extraRename(dest, previous); err != nil {
			return rollback(err)
		}
		movedOld = true
	}
	if err := boundary("previous"); err != nil {
		return rollback(err)
	}
	step = "live"
	if err := extraRename(incoming, dest); err != nil {
		return rollback(err)
	}
	installed = true
	if err := boundary("live"); err != nil {
		return rollback(err)
	}
	step = "metadata"
	if err := metaFile.write(func() error { return writeExtraMetadata(req, candidate, req.RawSource, req.Branch) }); err != nil {
		return rollback(err)
	}
	if err := boundary("metadata"); err != nil {
		return rollback(err)
	}
	if req.ProjectRoot != "" {
		step = "lock"
		if err := lockFile.write(func() error { return writeExtraPin(req, candidate, req.RawSource) }); err != nil {
			return rollback(err)
		}
	}
	if err := boundary("lock"); err != nil {
		return rollback(err)
	}
	step = "config"
	if err := configFile.write(req.WriteConfig); err != nil {
		return rollback(err)
	}
	if err := boundary("config"); err != nil {
		return rollback(err)
	}
	if err := boundary("cleanup"); err != nil {
		return rollback(err)
	}
	if hadPrevious {
		if err := extraCleanup(previous); err != nil {
			return fmt.Errorf("publication cleanup failed; marker retained: %w", err)
		}
		movedOld = false
	}
	if err := boundary("cleaned-previous"); err != nil {
		return fmt.Errorf("publication cleanup interrupted; marker retained: %w", err)
	}
	if err := os.Remove(marker); err != nil {
		return fmt.Errorf("publication cleanup: %w", err)
	}
	_ = os.Remove(stage)
	return nil
}

func writeExtraMetadata(req RemoteExtraRequest, candidate *extraCandidate, remote, branch string) error {
	hashes, err := extraTreeHashes(candidate.staged)
	if err != nil {
		return err
	}
	store, _, err := ReadMetadata(req.ExtrasRoot)
	if err != nil {
		return err
	}
	store.Set(req.Name, &MetadataEntry{Kind: "extra", Source: remote, RepoURL: candidate.fetch.RepoPath, Subdir: candidate.fetch.Subdir, Branch: branch, Commit: candidate.fetch.Commit, TreeHash: candidate.treeHash, FileHashes: hashes, InstalledAt: time.Now()})
	// RepoPath is private staging; persist the source's clone URL instead.
	source, err := ParseSourceWithOptions(remote, req.Opts.ParseOptions)
	if err != nil {
		return err
	}
	store.Get(req.Name).RepoURL = source.CloneURL
	if candidate.audit.Overridden && candidate.audit.Audit != nil {
		keys := candidate.audit.Audit.AcceptKeysAtOrAbove(candidate.audit.Threshold)
		if len(keys) > 0 {
			if store.AuditAccepted == nil {
				store.AuditAccepted = make(map[string][]string)
			}
			merged := make(map[string]bool)
			for _, key := range store.AuditAccepted[req.Name] {
				merged[key] = true
			}
			for _, key := range keys {
				merged[key] = true
			}
			all := make([]string, 0, len(merged))
			for key := range merged {
				all = append(all, key)
			}
			sort.Strings(all)
			store.AuditAccepted[req.Name] = all
		}
	}
	return extraWriteMeta(store, req.ExtrasRoot)
}

func writeExtraPin(req RemoteExtraRequest, candidate *extraCandidate, remote string) error {
	lock, err := LoadLock(req.LockDir)
	if err != nil {
		return err
	}
	lock.SetExtraPin(req.Name, LockEntry{Source: remote, Commit: candidate.fetch.Commit, TreeHash: candidate.treeHash})
	return extraWriteLock(lock, req.LockDir)
}
