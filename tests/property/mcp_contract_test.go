package property

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"hegel.dev/go/hegel"
	"skillshare/internal/mcp"
)

// This separate module keeps Hegel's native test engine out of production
// dependencies. All MCP traffic is an independently authored inert fixture.
func propertyServer(t *testing.T) (*atomic.Value, string, *atomic.Int64) {
	t.Helper()
	current := &atomic.Value{}
	inventoryRequests := &atomic.Int64{}
	current.Store(`{"name":"read","inputSchema":{"type":"object"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage
			Method string
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			w.WriteHeader(400)
			return
		}
		var result any = map[string]any{"tools": []json.RawMessage{json.RawMessage(current.Load().(string))}}
		if req.Method == "server/discover" {
			result = map[string]any{"supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{"tools": map[string]any{}}, "_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]string{"name": "property-fixture", "version": "1"}}}
		}
		if req.Method == "tools/list" {
			inventoryRequests.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return current, srv.URL, inventoryRequests
}

func TestContractPropertyNumericDirectionsUseValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current, endpoint, _ := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(-1000000, 1000000))
		delta := hegel.Draw(ht, hegel.Integers(1, 1000))
		s := propertyService(t, endpoint)
		maximum := func(x int) string {
			return fmt.Sprintf(`{"name":"read","inputSchema":{"type":"object","properties":{"x":{"type":"integer","maximum":%d}}}}`, x)
		}
		current.Store(maximum(n))
		before, err := review(s, "")
		if err != nil {
			ht.Fatal(err)
		}
		accepted, err := review(s, before.Servers[0].Contract.Revision)
		if err != nil || accepted.ToolsAcceptance != mcp.ToolsAcceptanceAccepted {
			ht.Fatal("could not establish explicit baseline")
		}
		current.Store(maximum(n + delta))
		after, err := review(s, "")
		if err != nil {
			ht.Fatal(err)
		}
		found := false
		for _, change := range after.Servers[0].Contract.Changes {
			if change.Path == "/inputSchema/properties/x/maximum" {
				found = true
				if change.Grade != "LOOSENED" {
					ht.Fatalf("larger upper bound grade=%s", change.Grade)
				}
			}
		}
		if !found {
			ht.Fatal("bound change disappeared")
		}
	}, hegel.WithTestCases(40), hegel.WithDatabase(""))
}

func propertyService(t *testing.T, endpoint string) *mcp.Service {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("mcp:\n  targets: []\n  servers:\n    fixture:\n      targets: []\n      url: "+endpoint+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return &mcp.Service{ConfigPath: path, Home: root, StateDir: filepath.Join(root, "state"), Platform: "linux"}
}

func review(s *mcp.Service, revision string) (*mcp.CheckReport, error) {
	opts := mcp.CheckOptions{Live: true, SkipDNS: true, Names: []string{"fixture"}}
	if revision != "" {
		opts.AcceptTools, opts.Revision = true, revision
	}
	return s.Check(opts)
}
