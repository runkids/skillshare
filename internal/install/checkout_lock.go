package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// checkoutLockWait bounds how long an update waits for another process to
// finish updating the same checkout. The dashboard waits while holding its
// server lock, so keep it short.
var checkoutLockWait = time.Minute

// LockCheckout holds an exclusive lock for updating the git checkout at
// repoPath until the returned unlock runs. A dashboard's own mutex cannot see a
// second dashboard or the CLI, and two git processes pulling into one checkout
// fail each other with spurious "local changes" and index-pack errors.
func LockCheckout(repoPath string) (func(), error) {
	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		// A .git file points at the real directory, as in a linked worktree.
		if gitDir, err = gitOutput(repoPath, "rev-parse", "--absolute-git-dir"); err != nil {
			return nil, fmt.Errorf("failed to locate git directory of %s: %w", repoPath, err)
		}
	}
	lock := flock.New(filepath.Join(gitDir, "skillshare-update.lock"))
	ctx, cancel := context.WithTimeout(context.Background(), checkoutLockWait)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 200*time.Millisecond)
	if err != nil && ctx.Err() == nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("another skillshare process is still updating %s; retry when it finishes", repoPath)
	}
	return func() { _ = lock.Unlock() }, nil
}
