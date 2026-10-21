package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// LockConfig serializes project mutations across CLI and dashboard processes.
// Callers reload after acquisition and hold the lock through detach and save.
// Other config writers do not participate. Reads and previews must not call it.
// The lock lives in the state dir, like the MCP and hooks locks, so it never
// lands in a config folder that skillshare push commits.
func LockConfig(ctx context.Context, path string) (func(), error) {
	dest, err := utils.ConfigWritePath(path, false)
	if err != nil {
		return nil, err
	}
	file, err := lockPath(dest)
	if err != nil {
		return nil, fmt.Errorf("project config lock %s: %w", dest, err)
	}
	lock := flock.New(file)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		_ = lock.Close()
		if err == nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("project config lock %s: %w", dest, err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = lock.Close() }) }, nil
}

// lockPath keys the lock by the resolved config file, so links to one config
// share a lock and separate configs do not.
func lockPath(dest string) (string, error) {
	dir := filepath.Join(config.StateDir(), "projects")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(filepath.Clean(dest)))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".lock"), nil
}
