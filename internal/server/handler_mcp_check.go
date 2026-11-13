package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/mcpchecklog"
	"skillshare/internal/version"
)

// handleMCPCheck observes configuration and optional live contracts. It never accepts
// or persists trust. Root presence distinguishes all roots from this config's list.
func (s *Server) handleMCPCheck(w http.ResponseWriter, r *http.Request) {
	// Release s.mu before the lookups and probes: every other API request waits on it.
	s.mu.RLock()
	service := s.mcpService()
	s.mu.RUnlock()
	query := r.URL.Query()
	var root *string
	if query.Has("root") {
		value := query.Get("root")
		root = &value
	}
	report, err := service.Check(mcp.CheckOptions{
		Root:          root,
		SkipDNS:       query.Get("dns") == "0",
		Live:          query.Get("live") == "1",
		Names:         query["name"],
		ClientVersion: version.Version,
	})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, mcpCheckWire(report))
}

// handleMCPProbe probes the server in the request, as the dashboard's form describes it
// before it is saved, the way ?live=1 probes a saved one. It saves and syncs nothing.
func (s *Server) handleMCPProbe(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	if body.Mutation.Server == nil {
		writeError(w, 400, "server is required")
		return
	}
	s.mu.RLock()
	service, ok := s.mcpProjectService(w, body.Mutation.Project)
	s.mu.RUnlock()
	if !ok {
		return
	}
	// A project server starts in its root, as a live check of a saved one does.
	root := ""
	if body.Mutation.Project != "" {
		root = service.ProjectRoot
	}
	result, err := mcp.ProbeDraft(*body.Mutation.Server, root, mcp.CheckOptions{ClientVersion: version.Version})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, result)
}

// handleMCPAcceptTools delegates revision/source/store validation and persistence
// to the same service used by the CLI. No server lock is held during its probes.
func (s *Server) handleMCPAcceptTools(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string  `json:"name"`
		Root     *string `json:"root"`
		Revision string  `json:"revision"`
	}
	var raw json.RawMessage
	err := decodeJSONWith(w, r, defaultJSONBodyLimit, func(d *json.Decoder) error {
		if err := d.Decode(&raw); err != nil {
			return err
		}
		if err := d.Decode(new(any)); err != io.EOF {
			if err != nil {
				return err
			}
			return errors.New("expected one request")
		}
		return nil
	})
	if errors.Is(err, errBodyTooLarge) {
		return
	}
	var fields map[string]json.RawMessage
	if err != nil || mcp.DecodeContractJSON(raw, &fields) != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "invalid MCP check request")
		return
	}
	for key := range fields {
		if key != "name" && key != "root" && key != "revision" {
			writeError(w, http.StatusBadRequest, "invalid MCP check request")
			return
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || body.Root == nil || strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Revision) == "" {
		writeError(w, http.StatusBadRequest, "name, root and revision are required")
		return
	}
	s.mu.RLock()
	service := s.mcpService()
	s.mu.RUnlock()
	start := time.Now()
	report, checkErr := service.Check(mcp.CheckOptions{
		Live: true, AcceptTools: true, Names: []string{body.Name}, Root: body.Root,
		Revision: body.Revision, ClientVersion: version.Version,
	})
	// As with other API scan logs, logging is best effort and cannot undo a
	// completed approval. The shared writer includes only counts and outcomes.
	_ = mcpchecklog.WriteAudit(service.ConfigPath, start, report, checkErr)
	status := http.StatusUnprocessableEntity
	accepted := false
	if checkErr == nil && report != nil {
		switch report.ToolsAcceptance {
		case mcp.ToolsAcceptanceAccepted:
			status, accepted = http.StatusOK, true
		case mcp.ToolsAcceptanceStale:
			status = http.StatusConflict
		}
	}
	logStatus := "error"
	if accepted {
		logStatus = "ok"
	}
	// Report labels have passed the service's complete credential boundary.
	// Never log the raw request name: even a refused identity can contain a secret.
	var names []string
	if report != nil {
		for _, server := range report.Servers {
			names = append(names, server.Name)
		}
	}
	s.mu.RLock()
	s.writeOpsLog("mcp check --accept-tools", logStatus, start, map[string]any{"names": names}, "")
	s.mu.RUnlock()
	response := struct {
		Accepted bool                `json:"accepted"`
		Report   *mcpCheckWireReport `json:"report"`
		Error    string              `json:"error,omitempty"`
	}{Accepted: accepted, Report: mcpCheckWire(report)}
	if !accepted {
		response.Error = "inventory was not accepted; review the check report"
		if status == http.StatusConflict {
			response.Error = "review is stale; probe again before accepting"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

// The API carries snapshots as exact text. Decoding them into JavaScript values
// would round large numbers and alter the inventory a user is reviewing.
type mcpCheckWireTool struct {
	Hash           string `json:"hash"`
	InputHash      string `json:"inputHash"`
	ProseHash      string `json:"proseHash"`
	OutputHash     string `json:"outputHash"`
	MetadataHash   string `json:"metadataHash"`
	DefinitionText string `json:"definitionText"`
}
type mcpCheckWireContract struct {
	*mcp.ContractReview
	Tools map[string]mcpCheckWireTool `json:"tools"`
}
type mcpCheckWireServer struct {
	mcp.CheckServer
	Contract *mcpCheckWireContract `json:"contract,omitempty"`
}
type mcpCheckWireReport struct {
	*mcp.CheckReport
	Servers []mcpCheckWireServer `json:"servers"`
}

func mcpCheckWire(report *mcp.CheckReport) *mcpCheckWireReport {
	if report == nil {
		return nil
	}
	out := &mcpCheckWireReport{CheckReport: report, Servers: make([]mcpCheckWireServer, 0, len(report.Servers))}
	for _, server := range report.Servers {
		item := mcpCheckWireServer{CheckServer: server}
		if c := server.Contract; c != nil {
			tools := make(map[string]mcpCheckWireTool, len(c.Tools))
			for name, tool := range c.Tools {
				tools[name] = mcpCheckWireTool{tool.Hash, tool.InputHash, tool.ProseHash, tool.OutputHash, tool.MetadataHash, string(tool.Definition)}
			}
			item.Contract = &mcpCheckWireContract{ContractReview: c, Tools: tools}
		}
		out.Servers = append(out.Servers, item)
	}
	return out
}
