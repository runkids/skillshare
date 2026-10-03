//go:build windows

package childproc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const helperEnv = "CHILDPROC_TEST_HELPER"

// TestMain doubles as the helper: bind, start a plain and a detached child,
// print their PIDs and whether children may break away, then wait to be
// terminated. In "relay" mode it binds, starts the helper through ShareJob (as
// the hidden-console relaunch does), and passes its output on.
func TestMain(m *testing.M) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		os.Exit(m.Run())
	}
	if !BindToProcess() {
		fmt.Println("unbound")
		os.Exit(0)
	}
	if mode == "relay" {
		inner := exec.Command(os.Args[0])
		inner.Env = append(os.Environ(), helperEnv+"=1")
		inner.Stdout = os.Stdout
		ShareJob(inner)
		if err := inner.Run(); err != nil {
			fmt.Println("error", err)
		}
		os.Exit(0)
	}
	plain := exec.Command("ping", "-n", "120", "127.0.0.1")
	kept := exec.Command("ping", "-n", "120", "127.0.0.1")
	if err := plain.Start(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	if err := StartDetached(kept); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
	fmt.Println(plain.Process.Pid, kept.Process.Pid, breakawayAllowed())
	time.Sleep(time.Minute)
	os.Exit(0)
}

// terminateBoundHelper starts the helper in the given mode, kills it, and
// returns the children and whether they could break away from its job.
func terminateBoundHelper(t *testing.T, mode string) (plain, kept int, breakaway bool) {
	t.Helper()
	if !breakawayAllowed() {
		t.Skip("running inside a job that forbids breakaway (for example Task Scheduler)")
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), helperEnv+"="+mode)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	fields := strings.Fields(line)
	if len(fields) != 3 {
		_ = cmd.Process.Kill()
		t.Fatalf("helper said %q", line)
	}
	plain, _ = strconv.Atoi(fields[0])
	kept, _ = strconv.Atoi(fields[1])
	breakaway = fields[2] == "true"
	t.Cleanup(func() { kill(plain); kill(kept) })
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	time.Sleep(500 * time.Millisecond)
	return plain, kept, breakaway
}

func TestBindToProcess_TerminatingTheProcessStopsChildren(t *testing.T) {
	plain, _, _ := terminateBoundHelper(t, "1")
	if running(plain) {
		t.Fatal("child kept running after its parent was terminated")
	}
}

func TestShareJob_TerminatingTheParentStopsTheCopysChildren(t *testing.T) {
	plain, _, _ := terminateBoundHelper(t, "relay")
	if running(plain) {
		t.Fatal("child of the relaunched copy kept running after the parent was terminated")
	}
}

func TestShareJob_CopysDetachedChildOutlivesTheParent(t *testing.T) {
	_, kept, _ := terminateBoundHelper(t, "relay")
	if !running(kept) {
		t.Fatal("detached child of the relaunched copy stopped with the parent")
	}
}

func TestStartDetached_ChildOutlivesTerminatedProcess(t *testing.T) {
	_, kept, _ := terminateBoundHelper(t, "1")
	if !running(kept) {
		t.Fatal("detached child stopped with its parent")
	}
}

// The MSYS runtime (Git for Windows' sh.exe) breaks its children away whenever
// the job allows it, so the job must not allow it outside StartDetached.
func TestBindToProcess_ChildrenCannotBreakAway(t *testing.T) {
	_, _, breakaway := terminateBoundHelper(t, "1")
	if breakaway {
		t.Fatal("job allows breakaway after StartDetached returned")
	}
}

func running(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	return windows.GetExitCodeProcess(h, &code) == nil && code == 259 // STILL_ACTIVE
}

func kill(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}
