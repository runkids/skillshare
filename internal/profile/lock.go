package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// Lock serializes profile mutations and skill sync for one config.
func Lock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("profile lock %s: %w", path, err)
	}
	lock := flock.New(path)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil || !ok {
		_ = lock.Close()
		if err == nil {
			err = ctx.Err()
			if err == nil {
				err = fmt.Errorf("lock unavailable")
			}
		}
		return nil, fmt.Errorf("another sync or profile switch is running for this config (lock %s): %w", path, err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = lock.Close() }) }, nil
}
