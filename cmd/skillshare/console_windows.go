//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const hiddenConsoleEnv = "SKILLSHARE_HIDDEN_CONSOLE"

// relaunchInHiddenConsole reruns skillshare inside a windowless console when it
// was started without one (a scheduled task wrapper, pythonw). Without a console,
// every console program it starts (git, native agent CLIs) opens its own window;
// inside one, they all inherit it. Runs from a terminal never take this path.
func relaunchInHiddenConsole() {
	if _, err := windows.GetConsoleCP(); err == nil || os.Getenv(hiddenConsoleEnv) != "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), hiddenConsoleEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Start(); err != nil {
		// The relaunch could not start; run here as before.
		return
	}
	bindToThisProcess(cmd.Process.Pid)
	err = cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillshare: hidden-console relaunch failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// bindToThisProcess puts the relaunched process in a kill-on-close job, so a
// caller that terminates the PID it started also stops the work. Its own
// children break away silently: git, the background UI server, browsers and
// editors behave as they would without the relaunch. Best effort: if the job
// cannot be set up, the relaunch still runs, only without this guarantee.
func bindToThisProcess(pid int) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return
	}
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(job)
		return
	}
	defer windows.CloseHandle(proc)
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		windows.CloseHandle(job)
	}
	// The job handle stays open until this process exits; closing it kills the child.
}
