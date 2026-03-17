// TODO: Move doctor check logic from cmd/skillshare/ to internal/doctor/
// so the server can call it directly instead of shelling out to the CLI binary.
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
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

	cmd.Env = append(os.Environ(), "SKILLSHARE_CONFIG="+s.configPath())

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
