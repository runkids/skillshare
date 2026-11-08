package property

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestContractPropertyInvalidOrUnauditableInventoriesNeverCreateTrust(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current, endpoint, inventoryRequests := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		variant := hegel.Draw(ht, hegel.Integers(0, 5))
		var descriptor string
		switch variant {
		case 0:
			descriptor = `{"name":"read","\u006eame":"other","inputSchema":{"type":"object"}}`
		case 1:
			descriptor = `{"name":"read","description":"\ud800","inputSchema":{"type":"object"}}`
		case 2:
			descriptor = `{"name":"read"}`
		case 3:
			descriptor = `{"name":"read","inputSchema":{"type":"object","$comment":42}}`
		case 4:
			descriptor = `{"name":"read","inputSchema":{"type":"object"},"extension":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}"
		case 5:
			descriptor = `{"name":"read","inputSchema":{"type":"object"},"description":"` + strings.Repeat(`\n`, 9000) + `end"}`
		}
		s := propertyService(t, endpoint)
		current.Store(descriptor)
		before := inventoryRequests.Load()
		report, err := review(s, "")
		if err != nil {
			ht.Fatalf("check could not report refusal: %v", err)
		}
		if inventoryRequests.Load() == before {
			ht.Fatal("malformed-inventory check never requested the live inventory")
		}
		if report.Summary.Errors == 0 {
			ht.Fatalf("malformed/incomplete variant %d reported no error", variant)
		}
		refused := false
		for _, server := range report.Servers {
			if server.Contract != nil && server.Contract.Revision != "" {
				ht.Fatal("invalid inventory exposed approval token")
			}
			for _, finding := range server.Findings {
				if finding.Level != "error" {
					continue
				}
				switch variant {
				case 0, 1:
					refused = refused || (finding.Check == "live" && strings.Contains(finding.Message, "tools/list") && strings.Contains(finding.Message, "not a JSON-RPC response"))
				case 2, 3, 4:
					refused = refused || (finding.Check == "live" && strings.Contains(finding.Message, "tool inventory rejected"))
				case 5:
					refused = refused || (finding.Check == "tool-change" && strings.Contains(finding.Message, "tool contract audit was incomplete"))
				}
			}
		}
		if !refused {
			ht.Fatalf("variant %d did not report its inventory or audit refusal", variant)
		}
		if _, err := os.Stat(filepath.Join(s.StateDir, "mcp", "tool-pins.json")); !os.IsNotExist(err) {
			ht.Fatal("invalid observation wrote trust")
		}
	}, hegel.WithTestCases(40), hegel.WithDatabase(""))
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

func TestContractPropertyNumericEnumSpellingsNeverClaimDirection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current, endpoint, _ := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 1000000))
		s := propertyService(t, endpoint)
		current.Store(fmt.Sprintf(`{"name":"read","inputSchema":{"type":"object","properties":{"x":{"enum":[%d,%d.0]}}}}`, n, n))
		before, err := review(s, "")
		if err != nil {
			ht.Fatal(err)
		}
		accepted, err := review(s, before.Servers[0].Contract.Revision)
		if err != nil || accepted.ToolsAcceptance != mcp.ToolsAcceptanceAccepted {
			ht.Fatal("could not establish explicit baseline")
		}
		current.Store(fmt.Sprintf(`{"name":"read","inputSchema":{"type":"object","properties":{"x":{"enum":[%d]}}}}`, n))
		after, err := review(s, "")
		if err != nil {
			ht.Fatal(err)
		}
		found := false
		for _, change := range after.Servers[0].Contract.Changes {
			if strings.HasSuffix(change.Path, "/enum") {
				found = true
				if change.Grade != "UNKNOWN" {
					ht.Fatalf("equal numeric value set received %s", change.Grade)
				}
			}
		}
		if !found {
			ht.Fatal("lexical enum change disappeared")
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

func TestContractPropertyEquivalentSpellingAndExactNumbers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current, endpoint, _ := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		n := int64(hegel.Draw(ht, hegel.Integers(0, 1000))) + 9007199254740992
		s := propertyService(t, endpoint)
		current.Store(fmt.Sprintf(`{"name":"read","description":"Read a bounded value","inputSchema":{"type":"object","properties":{"limit":{"type":"integer","maximum":%d}}}}`, n))
		a, err := review(s, "")
		if err != nil || a.Servers[0].Contract == nil {
			ht.Fatalf("first review failed: %v", err)
		}
		first := a.Servers[0].Contract
		if first.Status != "untrusted" || first.Revision == "" {
			ht.Fatalf("first trust state: %+v", first)
		}
		current.Store(fmt.Sprintf(`{ "inputSchema": { "properties":{"limit":{"maximum":%d,"type":"integer"}},"type":"object"},"description":"Read a bounded value","name":"read" }`, n))
		b, err := review(s, "")
		if err != nil || b.Servers[0].Contract.Revision != first.Revision {
			ht.Fatal("equivalent spelling changed revision")
		}
		current.Store(fmt.Sprintf(`{"name":"read","description":"Read a bounded value","inputSchema":{"type":"object","properties":{"limit":{"type":"integer","maximum":%d}}}}`, n+1))
		c, err := review(s, "")
		if err != nil || c.Servers[0].Contract.Tools["read"].Hash == first.Tools["read"].Hash {
			ht.Fatal("distinct bounds beyond 2^53 collapsed")
		}
		if _, err := os.Stat(filepath.Join(s.StateDir, "mcp", "tool-pins.json")); !os.IsNotExist(err) {
			ht.Fatal("observation wrote trust state")
		}
	}, hegel.WithTestCases(40), hegel.WithDatabase(""))
}

func TestContractPropertyStaleApprovalPreservesEveryByte(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	current, endpoint, _ := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 1000000))
		s := propertyService(t, endpoint)
		current.Store(fmt.Sprintf(`{"name":"read","description":"Read item %d","inputSchema":{"type":"object"}}`, n))
		a, err := review(s, "")
		if err != nil || a.Servers[0].Contract.Revision == "" {
			ht.Fatalf("review failed: %v", err)
		}
		accepted, err := review(s, a.Servers[0].Contract.Revision)
		if err != nil || accepted.ToolsAcceptance != mcp.ToolsAcceptanceAccepted {
			ht.Fatalf("accept failed: %v", err)
		}
		path := filepath.Join(s.StateDir, "mcp", "tool-pins.json")
		before, err := os.ReadFile(path)
		if err != nil {
			ht.Fatal(err)
		}
		old, err := review(s, "")
		if err != nil {
			ht.Fatal(err)
		}
		current.Store(fmt.Sprintf(`{"name":"read","description":"Read item %d","inputSchema":{"type":"object"}}`, n+1))
		stale, err := review(s, old.Servers[0].Contract.Revision)
		if err != nil || stale.ToolsAcceptance != mcp.ToolsAcceptanceStale {
			ht.Fatalf("stale approval accepted: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			ht.Fatal("refused approval changed stored bytes")
		}
	}, hegel.WithTestCases(40), hegel.WithDatabase(""))
}
