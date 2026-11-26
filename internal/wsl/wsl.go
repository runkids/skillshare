// Package wsl lets skillshare running inside Windows Subsystem for Linux open
// folders, files and URLs on the Windows desktop, where the user's browser runs.
package wsl

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// osRelease is the kernel release file; a WSL kernel names Microsoft in it.
var osRelease = "/proc/sys/kernel/osrelease"

// windowsExplorer is where explorer.exe sits on the default C: mount, for a WSL setup
// with appendWindowsPath=false, which leaves Windows folders off the Linux PATH.
var windowsExplorer = "/mnt/c/Windows/explorer.exe"

// Explorer returns explorer.exe when skillshare runs inside WSL and Windows
// programs can be started from it. A container on a WSL kernel has no
// explorer.exe, so it is not treated as WSL.
func Explorer() (string, bool) {
	if !Kernel() {
		return "", false
	}
	for _, name := range []string{"explorer.exe", windowsExplorer} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	// wsl.conf can mount drives somewhere other than /mnt; wslpath knows where C: is.
	if out, err := exec.Command("wslpath", "-u", `C:\Windows\explorer.exe`).Output(); err == nil {
		if path, err := exec.LookPath(strings.TrimSpace(string(out))); err == nil {
			return path, true
		}
	}
	return "", false
}

// Kernel reports whether skillshare runs on a WSL kernel, inside a distro or a container.
func Kernel() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	release, err := os.ReadFile(osRelease)
	return err == nil && strings.Contains(strings.ToLower(string(release)), "microsoft")
}

// WindowsPath converts a Linux path to the form Windows programs read, such as
// \\wsl.localhost\Ubuntu\root\.claude.json or C:\Users\me for /mnt/c/Users/me.
func WindowsPath(path string) (string, error) {
	out, err := exec.Command("wslpath", "-w", path).Output()
	if err != nil {
		return "", fmt.Errorf("wslpath -w %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}
