package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// codexCLIEnv points plugin commands at a Codex CLI on this machine only, for when the
// config is shared with another OS or a background job starts with a minimal PATH.
const codexCLIEnv = "SKILLSHARE_CODEX_CLI"

// codexFinder looks for the Codex CLI. Its fields are the machine, so tests can stand in.
type codexFinder struct {
	goos         string
	home         string
	localAppData string
	getenv       func(string) string
	lookPath     func(string) (string, error)
}

func newCodexFinder() codexFinder {
	home, _ := os.UserHomeDir()
	return codexFinder{goos: runtime.GOOS, home: home, localAppData: os.Getenv("LOCALAPPDATA"), getenv: os.Getenv, lookPath: exec.LookPath}
}

// candidates are the places a Codex CLI is installed outside PATH: Homebrew, which a
// background job's PATH lacks, and the copy the Codex desktop app ships. A pattern may
// hold one *, for the Windows app's per-version folder.
func (f codexFinder) candidates() []string {
	switch f.goos {
	case "darwin":
		paths := []string{"/opt/homebrew/bin/codex", "/usr/local/bin/codex", "/Applications/Codex.app/Contents/Resources/codex"}
		if f.home != "" {
			paths = append(paths, filepath.Join(f.home, "Applications", "Codex.app", "Contents", "Resources", "codex"))
		}
		return paths
	case "windows":
		if f.localAppData != "" {
			return []string{filepath.Join(f.localAppData, "OpenAI", "Codex", "bin", "*", "codex.exe")}
		}
	}
	return nil
}

// find returns the Codex CLI to run and the places it looked. It returns "codex" when
// nothing is found, so the run still fails as missing, and looked then names the places.
func (f codexFinder) find() (string, []string) {
	if cli := strings.TrimSpace(f.getenv(codexCLIEnv)); cli != "" {
		return cli, []string{codexCLIEnv + "=" + cli}
	}
	looked := []string{"PATH"}
	if _, err := f.lookPath("codex"); err == nil {
		return "codex", looked
	}
	for _, pattern := range f.candidates() {
		looked = append(looked, pattern)
		if path := newest(pattern); path != "" {
			return path, looked
		}
	}
	return "codex", looked
}

// newest is the most recently modified file matching pattern: the Codex app leaves one
// folder per version it has installed.
func newest(pattern string) string {
	matches, _ := filepath.Glob(pattern)
	best, bestTime := "", int64(0)
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if t := info.ModTime().UnixNano(); best == "" || t > bestTime {
			best, bestTime = path, t
		}
	}
	return best
}
