package plugin

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOMPExtensionsNativeLockHashVectors(t *testing.T) {
	// Independently generated with libxxhash.so.0 XXH64 via Python ctypes.
	vectors := map[string]string{
		"":                     "omp-file-lock-dd4458733774f8b02ff557a2e70c6e06",
		"/tmp/config.yml.lock": "omp-file-lock-71327ba360366b2a3eb5de2fa0093fa0",
		"/a/" + strings.Repeat("b", 100) + ".lock": "omp-file-lock-84bf27afdf270489c51bc843e1a7d324",
		"/tmp/日本/config.yml.lock":                  "omp-file-lock-8e86d6a4347e8cc9794efc489b074c6b",
	}
	for path, want := range vectors {
		if got := ompLockName(path); got != want {
			t.Fatalf("%q: %s != %s", path, got, want)
		}
	}
}

func TestOMPExtensionsNativeLockInterop(t *testing.T) {
	bun, module := os.Getenv("OMP_LOCK_TEST_BUN"), os.Getenv("OMP_LOCK_TEST_MODULE")
	if bun == "" || module == "" {
		t.Skip("set OMP_LOCK_TEST_BUN and OMP_LOCK_TEST_MODULE to the pinned native FileLock addon")
	}
	path := filepath.Join(t.TempDir(), "config.yml")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// This imports only the lock addon; no OMP bootstrap, hook or extension loader.
	script := `const {FileLock}=await import(process.env.OMP_LOCK_TEST_MODULE); const lock=FileLock.tryAcquire(process.env.OMP_LOCK_RESOURCE+".lock"); console.log(lock.acquired?"ready":"busy"); if(lock.acquired && process.env.OMP_LOCK_HOLD){process.stdin.once("data",()=>{lock.release();process.exit(0)});}else{lock.release();}`
	command := func(hold bool) *exec.Cmd {
		cmd := exec.CommandContext(ctx, bun, "-e", script)
		cmd.Env = append(os.Environ(), "OMP_LOCK_RESOURCE="+path)
		if hold {
			cmd.Env = append(cmd.Env, "OMP_LOCK_HOLD=1")
		}
		return cmd
	}
	unlock, err := ompAcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd := command(false)
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "busy" {
		unlock()
		t.Fatalf("native did not see Go holder: %v %s", err, output)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	cmd = command(true)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatal("native holder did not acquire after Go release")
	}
	if release, err := ompAcquireLock(path); !errors.Is(err, ErrOMPExtensionsBusy) {
		if release != nil {
			release()
		}
		t.Fatalf("Go did not see native holder: %v", err)
	}
	if _, err := stdin.Write([]byte("release\n")); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	unlock, err = ompAcquireLock(path)
	if err != nil {
		t.Fatalf("native did not release: %v", err)
	}
	unlock()
}
