package server

import (
	"net/http"

	"skillshare/internal/mcp"
	"skillshare/internal/version"
)

// handleMCPCheck answers `skillshare mcp check --json` for the dashboard. It is read-only
// and writes no operation log entry. Findings that are errors still return 200; only a
// check that cannot run fails the request. ?dns=0 skips host lookups. ?live=1 also probes
// the servers, as `mcp check --live` does, and each server's live result lists the tool
// names it reported; repeated ?name= limits the check to those servers.
func (s *Server) handleMCPCheck(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := r.URL.Query()
	report, err := s.mcpService().Check(mcp.CheckOptions{
		SkipDNS:       query.Get("dns") == "0",
		Live:          query.Get("live") == "1",
		Names:         query["name"],
		ClientVersion: version.Version,
	})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, report)
}
