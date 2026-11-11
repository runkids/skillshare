// TODO: Move doctor check logic from cmd/skillshare/ to internal/doctor/
// so the server can call it directly instead of shelling out to the CLI binary.
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"

	"skillshare/internal/config"

	"skillshare/internal/theme"
)

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	args := []string{"doctor", "--json"}

	if s.IsProjectMode() {
		args = append(args, "-p")
	} else {
		args = append(args, "-g")
	}

	path, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot determine binary path: "+err.Error())
		return
	}

	cmd := exec.Command(path, args...)
	if s.IsProjectMode() {
		cmd.Dir = s.projectRoot
	}

	// Forward the server's resolved theme so the subprocess doesn't
	// fall back to "no-TTY" warning (it has no terminal). Uses an
	// internal env var so doctor reports "auto-detected", not "set via
	// SKILLSHARE_THEME" — the user didn't set anything.
	tm := theme.Get()
	globalConfigPath := s.configPath()
	if s.IsProjectMode() {
		globalConfigPath = config.ReadOnlyConfigPath()
	}
	cmd.Env = append(os.Environ(),
		"SKILLSHARE_CONFIG="+globalConfigPath,
		"_SKILLSHARE_THEME_FORWARDED="+string(tm.Mode),
	)

	output, err := cmd.Output()
	if err != nil {
		// doctor --json writes valid JSON to stdout even on error (exit 1)
		if len(output) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write(output)
			return
		}
		writeError(w, http.StatusInternalServerError, "doctor check failed: "+err.Error())
		return
	}

	// Validate JSON before proxying
	var raw json.RawMessage
	if json.Unmarshal(output, &raw) != nil {
		writeError(w, http.StatusInternalServerError, "doctor produced invalid JSON")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}
