package main

import (
	"fmt"
	"os"
)

// ErrNotInContainer is returned when execution is attempted outside a container
// without the RUNBOOK_ALLOW_EXECUTE override.
var ErrNotInContainer = fmt.Errorf(
	"runbook: refusing to execute outside a container\n" +
		"  Runbook commands are designed to run inside a devcontainer/Docker.\n" +
		"  Use --dry-run to parse without executing, or set\n" +
		"  RUNBOOK_ALLOW_EXECUTE=1 to override this safety check.",
)

// IsContainerEnv returns true if we're running inside a Docker container
// or if the RUNBOOK_ALLOW_EXECUTE env var is set.
func IsContainerEnv() bool {
	// Explicit override for testing or intentional host execution.
	if os.Getenv("RUNBOOK_ALLOW_EXECUTE") != "" {
		return true
	}
	// Standard Docker marker file.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Podman / other container runtimes.
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}
