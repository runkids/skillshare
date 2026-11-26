package wsl

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeWSL points the kernel release at release and puts stub Windows interop
// programs on PATH when interop is true.
func fakeWSL(t *testing.T, release string, interop bool) {
	t.Helper()
	dir := t.TempDir()
	osReleasePath := filepath.Join(dir, "osrelease")
	if err := os.WriteFile(osReleasePath, []byte(release+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldRelease, oldExplorer := osRelease, windowsExplorer
	osRelease, windowsExplorer = osReleasePath, filepath.Join(dir, "no-drive", "explorer.exe")
	t.Cleanup(func() { osRelease, windowsExplorer = oldRelease, oldExplorer })

	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if interop {
		for name, script := range map[string]string{
			"explorer.exe": "#!/bin/sh\nexit 1\n",
			"wslpath":      "#!/bin/sh\necho \"win($1 $2)\"\n",
		} {
			if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("PATH", bin)
}

func TestExplorerOnWSLWithInterop(t *testing.T) {
	fakeWSL(t, "5.15.167.4-microsoft-standard-WSL2", true)
	if path, ok := Explorer(); !ok || filepath.Base(path) != "explorer.exe" {
		t.Fatalf("Explorer() = %q, %v; want explorer.exe, true", path, ok)
	}
}

func TestExplorerOffWSL(t *testing.T) {
	fakeWSL(t, "6.10.14-linuxkit", true)
	if _, ok := Explorer(); ok {
		t.Fatal("Explorer() reported WSL on a non-WSL kernel")
	}
}

func TestExplorerOnWSLKernelWithoutInterop(t *testing.T) {
	fakeWSL(t, "5.15.167.4-microsoft-standard-WSL2", false)
	if _, ok := Explorer(); ok {
		t.Fatal("Explorer() reported WSL without explorer.exe on PATH")
	}
}

func TestExplorerOffPATHOnDefaultDrive(t *testing.T) {
	fakeWSL(t, "5.15.167.4-microsoft-standard-WSL2", false)
	drive := filepath.Join(t.TempDir(), "explorer.exe")
	if err := os.WriteFile(drive, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	windowsExplorer = drive
	if path, ok := Explorer(); !ok || path != drive {
		t.Fatalf("Explorer() = %q, %v; want %q, true", path, ok, drive)
	}
}

func TestWindowsPath(t *testing.T) {
	fakeWSL(t, "5.15.167.4-microsoft-standard-WSL2", true)
	got, err := WindowsPath("/root/.claude.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := "win(-w /root/.claude.json)"; got != want {
		t.Fatalf("WindowsPath() = %q, want %q", got, want)
	}
}
