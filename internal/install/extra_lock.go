package install

import (
	"context"
	"crypto/sha256"
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
	sum := sha256.Sum256([]byte(canonical))
	dir := filepath.Join(utils.StateDir(), "extras-locks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, fmt.Sprintf("%x.lock", sum[:8])))
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
	return func() { _ = lock.Unlock() }, nil
}
