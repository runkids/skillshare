package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"skillshare/internal/utils"
)

// extraLockOnWait is a test hook for observing contention without sleeps.
var extraLockOnWait = func() {}

// canonicalExtraRoot resolves existing ancestors without creating the extras root.
func canonicalExtraRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := abs
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		// A dangling link is not a missing path: guessing its identity would
		// let another spelling acquire a different lock for the same root.
		if _, statErr := os.Lstat(path); statErr == nil {
			return "", fmt.Errorf("cannot resolve extras root %s: %w", path, err)
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

// LockExtrasRoot serializes supported writers of remote extras under one root.
func LockExtrasRoot(root string) (func(), error) {
	canonical, err := canonicalExtraRoot(root)
	if err != nil {
		return nil, err
	}
	// The administrative root and permanent lock may precede audit, but no
	// managed content or publication records are written by this acquisition.
	if err := os.MkdirAll(canonical, 0755); err != nil {
		return nil, err
	}
	resolved, err := canonicalExtraRoot(canonical)
	if err != nil || resolved != canonical {
		return nil, fmt.Errorf("extras root changed while acquiring its lock")
	}
	file := filepath.Join(canonical, utils.ExtrasRootLockFileName)
	before, err := os.Lstat(file)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if before != nil && (utils.IsLinkMode(file, before.Mode()) || !before.Mode().IsRegular() || before.Size() != 0) {
		return nil, fmt.Errorf("unsafe extras root lock %s", file)
	}
	lock := flock.New(file, flock.SetFlag(extraLockOpenFlags))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		extraLockOnWait()
		ctx, cancel := context.WithTimeout(context.Background(), checkoutLockWait)
		defer cancel()
		ok, err = lock.TryLockContext(ctx, 200*time.Millisecond)
		if err != nil && ctx.Err() == nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("another skillshare process is installing extras in %s; retry when it finishes", root)
		}
	}
	after, err := os.Lstat(file)
	if err != nil || utils.IsLinkMode(file, after.Mode()) || !after.Mode().IsRegular() || after.Size() != 0 || before != nil && !os.SameFile(before, after) {
		_ = lock.Unlock()
		return nil, fmt.Errorf("extras root lock changed while acquiring it")
	}
	return func() { _ = lock.Unlock() }, nil
}
