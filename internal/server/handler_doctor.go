// TODO: Move doctor check logic from cmd/skillshare/ to internal/doctor/
// so the server can call it directly instead of shelling out to the CLI binary.
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"skillshare/internal/config"

	"skillshare/internal/theme"
)

// doctorStreamEnv must match the CLI's: it makes doctor --json print each
// check as a {"check": ...} line before the full result.
const doctorStreamEnv = "_SKILLSHARE_DOCTOR_STREAM"

// doctorCommand runs this binary's doctor --json in the server's mode.
func (s *Server) doctorCommand(ctx context.Context) (*exec.Cmd, error) {
	args := []string{"doctor", "--json"}

	if s.IsProjectMode() {
		args = append(args, "-p")
	} else {
		args = append(args, "-g")
	}

	path, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine binary path: %w", err)
	}

	cmd := exec.CommandContext(ctx, path, args...)
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
	return cmd, nil
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	cmd, err := s.doctorCommand(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

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

// handleDoctorStream sends each check as a "check" event while doctor runs,
// then the full result as "done", so one slow check does not hold back the rest.
func (s *Server) handleDoctorStream(w http.ResponseWriter, r *http.Request) {
	cmd, err := s.doctorCommand(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cmd.Env = append(cmd.Env, doctorStreamEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "doctor check failed: "+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, "doctor check failed: "+err.Error())
		return
	}
	send, ok := initSSE(w)
	if !ok {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return
	}

	final, scanErr := relayDoctorChecks(stdout, send)
	waitErr := cmd.Wait()
	if r.Context().Err() != nil {
		return
	}

	// doctor --json writes valid JSON to stdout even on error (exit 1)
	var raw json.RawMessage
	if scanErr == nil && json.Unmarshal(final, &raw) == nil {
		send("done", raw)
		return
	}
	msg := "doctor produced invalid JSON"
	if scanErr != nil {
		msg = "doctor check failed: " + scanErr.Error()
	} else if waitErr != nil {
		msg = "doctor check failed: " + waitErr.Error()
	}
	send("error", map[string]string{"error": msg})
}

// relayDoctorChecks sends each {"check": ...} line as a "check" event and
// returns the rest of the output, which is doctor's full JSON result.
func relayDoctorChecks(out io.Reader, send func(string, any)) ([]byte, error) {
	var final bytes.Buffer
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		var streamed struct {
			Check json.RawMessage `json:"check"`
		}
		if bytes.HasPrefix(line, []byte(`{"check":`)) && json.Unmarshal(line, &streamed) == nil {
			send("check", streamed.Check)
			continue
		}
		final.Write(line)
		final.WriteByte('\n')
	}
	return final.Bytes(), scanner.Err()
}
