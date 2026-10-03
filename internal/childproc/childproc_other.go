//go:build !windows

// Package childproc ties child processes to skillshare's lifetime on Windows,
// where terminating a process does not reach the processes it started.
package childproc

import "os/exec"

// BindToProcess is a no-op outside Windows.
func BindToProcess() bool { return false }

// BindChild is a no-op outside Windows.
func BindChild(int) {}

// ShareJob is a no-op outside Windows.
func ShareJob(*exec.Cmd) {}

// StartDetached starts cmd; outside Windows nothing ties children to skillshare.
func StartDetached(cmd *exec.Cmd) error { return cmd.Start() }
