package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func notOnPath(string) (string, error) { return "", exec.ErrNotFound }

func noEnv(string) string { return "" }

func writeExe(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// SKILLSHARE_CODEX_CLI is set on one machine only, so it wins over everything found.
func TestCodexFinderPrefersEnvironment(t *testing.T) {
	f := codexFinder{goos: "darwin", getenv: func(string) string { return "/opt/codex" }, lookPath: func(string) (string, error) { return "/usr/bin/codex", nil }}
	if bin, _ := f.find(); bin != "/opt/codex" {
		t.Fatalf("bin = %q", bin)
	}
}

// A codex on PATH runs by name, as before.
func TestCodexFinderUsesPath(t *testing.T) {
	home := t.TempDir()
	writeExe(t, filepath.Join(home, "Applications", "Codex.app", "Contents", "Resources", "codex"), time.Now())
	f := codexFinder{goos: "darwin", home: home, getenv: noEnv, lookPath: func(string) (string, error) { return "/usr/bin/codex", nil }}
	if bin, _ := f.find(); bin != "codex" {
		t.Fatalf("bin = %q", bin)
	}
}

// With only the macOS app installed, its bundled CLI runs.
func TestCodexFinderFindsMacApp(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "Applications", "Codex.app", "Contents", "Resources", "codex")
	writeExe(t, want, time.Now())
	f := codexFinder{goos: "darwin", home: home, getenv: noEnv, lookPath: notOnPath}
	if bin, _ := f.find(); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

// Since the ChatGPT app merger the bundle is ChatGPT.app; it is tried before Codex.app.
func TestCodexFinderFindsChatGPTApp(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "Applications", "ChatGPT.app", "Contents", "Resources", "codex")
	writeExe(t, want, time.Now())
	writeExe(t, filepath.Join(home, "Applications", "Codex.app", "Contents", "Resources", "codex"), time.Now())
	f := codexFinder{goos: "darwin", home: home, getenv: noEnv, lookPath: notOnPath}
	if bin, _ := f.find(); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

// ChatGPT 26.924 moved the CLI to the packaged entrypoint codex-cli/bin/codex.
func TestCodexFinderFindsPackagedEntrypoint(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "Applications", "ChatGPT.app", "Contents", "Resources", "codex-cli", "bin", "codex")
	writeExe(t, want, time.Now())
	f := codexFinder{goos: "darwin", home: home, getenv: noEnv, lookPath: notOnPath}
	if bin, _ := f.find(); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

// The Windows app keeps one folder per version it installed; the newest one runs.
func TestCodexFinderFindsNewestWindowsApp(t *testing.T) {
	local := t.TempDir()
	old := filepath.Join(local, "OpenAI", "Codex", "bin", "aaa", "codex.exe")
	want := filepath.Join(local, "OpenAI", "Codex", "bin", "bbb", "codex.exe")
	writeExe(t, want, time.Now())
	writeExe(t, old, time.Now().Add(-time.Hour))
	f := codexFinder{goos: "windows", localAppData: local, getenv: noEnv, lookPath: notOnPath}
	if bin, _ := f.find(); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

// The official PowerShell installer's CLI comes before the app's copy, like Homebrew on macOS.
func TestCodexFinderPrefersWindowsInstaller(t *testing.T) {
	local := t.TempDir()
	want := filepath.Join(local, "Programs", "OpenAI", "Codex", "bin", "codex.exe")
	writeExe(t, want, time.Now())
	writeExe(t, filepath.Join(local, "OpenAI", "Codex", "bin", "aaa", "codex.exe"), time.Now())
	f := codexFinder{goos: "windows", localAppData: local, getenv: noEnv, lookPath: notOnPath}
	if bin, _ := f.find(); bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

// Found nowhere, codex still runs by name so it fails as missing, and every place is named.
func TestCodexFinderListsWhereItLooked(t *testing.T) {
	local := t.TempDir()
	f := codexFinder{goos: "windows", localAppData: local, getenv: noEnv, lookPath: notOnPath}
	bin, looked := f.find()
	want := []string{"PATH", filepath.Join(local, "Programs", "OpenAI", "Codex", "bin", "codex.exe"), filepath.Join(local, "OpenAI", "Codex", "bin", "*", "codex.exe")}
	if bin != "codex" || !slices.Equal(looked, want) {
		t.Fatalf("bin = %q, looked = %v", bin, looked)
	}
}

// A missing Codex CLI is still a missing CLI, and its error names where Skillshare looked.
func TestMissingCodexNamesWhereItLooked(t *testing.T) {
	s := &Service{
		findCodex: func() (string, []string) {
			return "codex", []string{"PATH", "/Applications/Codex.app/Contents/Resources/codex"}
		},
		Run: func(context.Context, string, []string, string, ...string) ([]byte, error) {
			return nil, agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: "codex CLI is not installed"}
		},
	}
	s.ProjectRoot = t.TempDir()
	_, err := s.run(context.Background(), "codex", "--version")
	key, args := ErrorKey(err)
	if !errors.Is(err, ErrCLIMissing) || key != "plugins.error.codexMissing" || !strings.Contains(err.Error(), "/Applications/Codex.app") || args["looked"] != "PATH, /Applications/Codex.app/Contents/Resources/codex" {
		t.Fatalf("err = %v, key = %q, args = %v", err, key, args)
	}
}

// An account's own cli is what it asked for, even the bare name codex; the search does not replace it.
func TestCodexAccountCLIIsNotSearched(t *testing.T) {
	var ran string
	s := &Service{
		Accounts:  map[string]Account{"codex-work": {Agent: "codex", Dir: t.TempDir(), CLI: "codex"}},
		findCodex: func() (string, []string) { t.Fatal("searched for codex"); return "", nil },
		Run: func(_ context.Context, _ string, _ []string, bin string, _ ...string) ([]byte, error) {
			ran = bin
			return nil, nil
		},
	}
	if _, err := s.run(context.Background(), "codex-work", "--version"); err != nil || ran != "codex" {
		t.Fatalf("ran %q, err = %v", ran, err)
	}
}
