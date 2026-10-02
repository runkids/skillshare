//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

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
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		// The relaunch could not start; run here as before.
		return
	}
	os.Exit(0)
}
