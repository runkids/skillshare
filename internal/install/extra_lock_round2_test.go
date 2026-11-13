package install

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLockExtrasRoot_DifferentStateDirsSubprocess(t *testing.T) {
	if os.Getenv("SKILLSHARE_EXTRA_LOCK_CHILD") == "1" {
		extraLockOnWait = func() { fmt.Println("WAITING") }
		unlock, err := LockExtrasRoot(os.Getenv("SKILLSHARE_EXTRA_LOCK_ROOT"))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("ACQUIRED")
		unlock()
		return
	}
	for _, alias := range []bool{false, true} {
		t.Run(fmt.Sprintf("alias=%t", alias), func(t *testing.T) {
			if alias && runtime.GOOS == "windows" {
				t.Skip("requires symlink privilege")
			}
			base := t.TempDir()
			t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state-a"))
			root := filepath.Join(base, "uncreated", "nested", "extras")
			other := root
			if alias {
				link := filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(base, link); err != nil {
					t.Fatal(err)
				}
				other = filepath.Join(link, "uncreated", "nested", "extras")
			}
			// Resolving a missing canonical identity remains a no-create operation.
			if _, err := canonicalExtraRoot(root); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(root); !os.IsNotExist(err) {
				t.Fatalf("canonicalization created root: %v", err)
			}
			unlock, err := LockExtrasRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			defer unlock()
			cmd := exec.Command(os.Args[0], "-test.run=^TestLockExtrasRoot_DifferentStateDirsSubprocess$")
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				switch key {
				case "HOME", "XDG_STATE_HOME", "TMPDIR", "SKILLSHARE_EXTRA_LOCK_CHILD", "SKILLSHARE_EXTRA_LOCK_ROOT":
				default:
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "HOME="+filepath.Join(base, "home-b"), "XDG_STATE_HOME="+filepath.Join(base, "state-b"), "TMPDIR="+t.TempDir(), "SKILLSHARE_EXTRA_LOCK_CHILD=1", "SKILLSHARE_EXTRA_LOCK_ROOT="+other)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			lines := make(chan string, 8)
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					lines <- scanner.Text()
				}
				close(lines)
			}()
			select {
			case line := <-lines:
				if line != "WAITING" {
					t.Fatalf("different environment bypassed held root lock: %q", line)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child did not reach lock contention barrier")
			}
			unlock()
			select {
			case line := <-lines:
				if line != "ACQUIRED" {
					t.Fatalf("child did not acquire after release: %q", line)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("child did not acquire after release")
			}
		})
	}
}
