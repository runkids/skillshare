package install

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestLockCheckout_WaitsForOtherHolder(t *testing.T) {
	repo := initTestRepo(t)
	other := flock.New(filepath.Join(repo, ".git", "skillshare-update.lock"))
	if err := other.Lock(); err != nil {
		t.Fatal(err)
	}
	prev := checkoutLockWait
	checkoutLockWait = 300 * time.Millisecond
	t.Cleanup(func() { checkoutLockWait = prev })

	if _, err := LockCheckout(repo); err == nil || !strings.Contains(err.Error(), "still updating") {
		t.Fatalf("expected busy error while another process holds the lock, got %v", err)
	}
	if err := other.Unlock(); err != nil {
		t.Fatal(err)
	}
	unlock, err := LockCheckout(repo)
	if err != nil {
		t.Fatalf("lock after release failed: %v", err)
	}
	unlock()
}
