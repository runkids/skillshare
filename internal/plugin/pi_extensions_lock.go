package plugin

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// piLockStale is proper-lockfile's default stale age, which Pi 0.99.2 uses for
// settings.json.lock: a lock directory whose mtime is older than this is removed
// and taken by the next Pi that wants the settings.
const piLockStale = 10 * time.Second

// piLockRefresh is how often a held lock's mtime is renewed, well below the
// stale age, as proper-lockfile itself does.
var piLockRefresh = 2 * time.Second

// piLockMargin is how fresh the lock must still be when the file is written, so
// Pi cannot see it as stale before the write finishes.
const piLockMargin = 3 * time.Second

// errPiLockLost means Pi's lock directory was replaced or touched by someone else
// while Skillshare held it; nothing was written.
var errPiLockLost = errors.New("Pi took over its settings lock during the change; nothing was written")

// piNativeLock is Pi's settings.json.lock held the way proper-lockfile holds it:
// a directory whose mtime is renewed while held. Acquisition may reclaim an
// unchanged, empty stale directory; renewal/release only touch the owned inode.
type piNativeLock struct {
	path string
	ours os.FileInfo

	mu    sync.Mutex
	mtime time.Time // the mtime last set; anything else means another owner
	lost  bool

	stop chan struct{}
	done chan struct{}
}

func acquirePiNativeLock(path string) (*piNativeLock, error) {
	if err := os.Mkdir(path, 0o755); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		if err := reclaimPiNativeLock(path); err != nil {
			return nil, fmt.Errorf("%w (Pi holds %s)", ErrPiExtensionsBusy, path)
		}
		// Another writer may acquire after removal. Never remove its new lock.
		if err := os.Mkdir(path, 0o755); err != nil {
			if os.IsExist(err) {
				return nil, ErrPiExtensionsBusy
			}
			return nil, err
		}
	}
	ours, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	l := &piNativeLock{path: path, ours: ours, mtime: ours.ModTime(), stop: make(chan struct{}), done: make(chan struct{})}
	go l.renew()
	return l, nil
}

// piBeforeLockReclaim lets tests refresh/replace the observed directory.
var piBeforeLockReclaim = func(string) {}

func reclaimPiNativeLock(path string) error {
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || time.Since(before.ModTime()) <= piLockStale {
		return ErrPiExtensionsBusy
	}
	// Keep the observed inode alive through the final ownership check. On
	// Windows this handle must share deletion, unlike os.OpenRoot's first handle.
	anchor, err := openPiLockAnchor(path)
	if err != nil {
		return err
	}
	defer anchor.Close()
	anchored, err := anchor.Stat()
	if err != nil || !os.SameFile(before, anchored) {
		return ErrPiExtensionsBusy
	}
	if _, err := anchor.ReadDir(1); err != io.EOF {
		return ErrPiExtensionsBusy
	}
	piBeforeLockReclaim(path)
	current, err := os.Lstat(path)
	if err != nil || !current.IsDir() || !os.SameFile(before, current) || !before.ModTime().Equal(current.ModTime()) || time.Since(current.ModTime()) <= piLockStale {
		return ErrPiExtensionsBusy
	}
	// Directory-only, nonrecursive removal: even a racing file/symlink is not
	// unlinked. As in proper-lockfile, the final stat/remove is not an atomic CAS.
	return removePiLockDirectory(path, anchor)
}

func (l *piNativeLock) renew() {
	defer close(l.done)
	t := time.NewTicker(piLockRefresh)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			l.touch()
		}
	}
}

// owned reports whether the directory is still the one made here, with the mtime
// set here. Callers hold l.mu.
func (l *piNativeLock) owned() (os.FileInfo, bool) {
	now, err := os.Lstat(l.path)
	if err != nil || !os.SameFile(l.ours, now) || !now.ModTime().Equal(l.mtime) {
		return nil, false
	}
	return now, true
}

func (l *piNativeLock) touch() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lost {
		return
	}
	if _, ok := l.owned(); !ok {
		l.lost = true
		return
	}
	t := time.Now()
	if err := os.Chtimes(l.path, t, t); err != nil {
		l.lost = true
		return
	}
	now, err := os.Lstat(l.path)
	if err != nil || !os.SameFile(l.ours, now) {
		l.lost = true
		return
	}
	l.mtime = now.ModTime()
}

// verify refuses unless the lock is still ours and fresh enough that Pi cannot
// treat it as stale before the write completes.
func (l *piNativeLock) verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lost {
		return fmt.Errorf("%w: %w", ErrPiExtensionsBusy, errPiLockLost)
	}
	if _, ok := l.owned(); !ok {
		l.lost = true
		return fmt.Errorf("%w: %w", ErrPiExtensionsBusy, errPiLockLost)
	}
	if time.Since(l.mtime) > piLockStale-piLockMargin {
		return fmt.Errorf("%w: %w", ErrPiExtensionsBusy, errPiLockLost)
	}
	return nil
}

func (l *piNativeLock) release() {
	close(l.stop)
	<-l.done
	l.mu.Lock()
	defer l.mu.Unlock()
	// Remove only a directory still provably ours: the same one, with the mtime set
	// here. Anything else may be another owner's lock and is left alone.
	if _, ok := l.owned(); ok {
		_ = os.Remove(l.path)
	}
}
