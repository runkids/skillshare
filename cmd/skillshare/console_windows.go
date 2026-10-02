//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"

	"skillshare/internal/childproc"
)

const hiddenConsoleEnv = "SKILLSHARE_HIDDEN_CONSOLE"

// relaunchInHiddenConsole reruns skillshare inside a windowless console when it
// was started without one (a scheduled task wrapper, pythonw). Without a console,
// every console program it starts (git, native agent CLIs) opens its own window;
// inside one, they all inherit it. Runs from a terminal never take this path.
// bound reports whether childproc.BindToProcess succeeded; the relaunched process
// then shares that job, and otherwise it gets one of its own.
func relaunchInHiddenConsole(bound bool) {
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
	if bound {
		childproc.ShareJob(cmd)
	}
	if err := cmd.Start(); err != nil {
		// The relaunch could not start; run here as before.
		return
	}
	if !bound {
		childproc.BindChild(cmd.Process.Pid)
	}
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
