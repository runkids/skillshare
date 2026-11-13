package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// StateDir returns Skillshare's platform-specific state directory.
func StateDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillshare")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "skillshare")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "skillshare")
}
