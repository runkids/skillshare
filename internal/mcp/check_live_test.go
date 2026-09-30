package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

const liveSecret = "s3cr3t-live-value"

// TestMCPLiveHelperServer is not a test: the live probe tests start this test binary as an
// MCP server, selected by MCP_LIVE_HELPER, speaking the mode given after --.
func TestMCPLiveHelperServer(t *testing.T) {
	if os.Getenv("MCP_LIVE_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if marker := os.Getenv("MCP_LIVE_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started"), 0600)
	}
	if os.Getenv("API_TOKEN") != liveSecret {
		fmt.Fprintln(os.Stderr, "API_TOKEN was not passed")
		os.Exit(4)
	}
	switch mode {
	case "crash":
		fmt.Fprintf(os.Stderr, "starting\nfatal: token %s rejected\n", os.Getenv("API_TOKEN"))
		os.Exit(3)
	case "hang":
		signal.Ignore(syscall.SIGTERM)
		if os.Getenv("MCP_LIVE_CHILD") == "" {
			child := exec.Command(os.Args[0], os.Args[1:]...)
			child.Env = append(os.Environ(), "MCP_LIVE_CHILD=1")
			if err := child.Start(); err != nil {
				os.Exit(5)
			}
			_ = os.WriteFile(os.Getenv("MCP_LIVE_PIDS"), fmt.Appendf(nil, "%d %d", os.Getpid(), child.Process.Pid), 0600)
		}
		select {}
	}
	out := json.NewEncoder(os.Stdout)
	reply := func(id json.RawMessage, result any) {
		_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	initialized := false
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Cursor string `json:"cursor"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			os.Exit(6)
		}
		switch {
		case mode == "modern" && msg.Method == "server/discover":
			reply(msg.ID, map[string]any{"resultType": "complete", "supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{"tools": map[string]any{}},
				"_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]string{"name": "helper", "version": "1.2.3"}}})
		case mode == "modern" && msg.Method == "tools/list" && msg.Params.Cursor == "":
			reply(msg.ID, map[string]any{"resultType": "complete", "tools": []map[string]string{{"name": "a"}}, "nextCursor": "page2"})
		case mode == "modern" && msg.Method == "tools/list":
			reply(msg.ID, map[string]any{"resultType": "complete", "tools": []map[string]string{{"name": "b"}}})
		case msg.Method == "initialize":
			reply(msg.ID, map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "legacy", "version": "0.9"}})
		case msg.Method == "notifications/initialized":
			initialized = true
		case msg.Method == "tools/list" && initialized:
			reply(msg.ID, map[string]any{"tools": []map[string]string{{"name": "a"}, {"name": "b"}, {"name": "c"}}})
		case mode == "silent":
			// Some legacy servers never answer a request they do not know.
		default:
			_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{"code": -32601, "message": "Method not found"}})
		}
	}
	os.Exit(0)
}

// liveConfig declares one stdio server that runs the helper above in mode.
func liveConfig(mode string, env map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "mcp:\n  targets: [claude]\n  servers:\n    fake:\n      command: %q\n      args: [%q, \"--\", %q]\n      env:\n        MCP_LIVE_HELPER: \"1\"\n        API_TOKEN: {fromEnv: LIVE_TOKEN}\n", os.Args[0], "-test.run=^TestMCPLiveHelperServer$", mode)
	for _, key := range sortedKeys(env) {
		fmt.Fprintf(&b, "        %s: %q\n", key, env[key])
	}
	return b.String()
}

func liveCheck(timeout time.Duration, env map[string]string) CheckOptions {
	opts := offlineCheck()
	opts.Live, opts.Timeout = true, timeout
	opts.LookPath = exec.LookPath
	opts.LookupEnv = func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
	return opts
}

func liveOf(t *testing.T, report *CheckReport) (*CheckLive, CheckFinding) {
	t.Helper()
	for _, f := range report.Servers[0].Findings {
		if f.Check == "live" {
			return report.Servers[0].Live, f
		}
	}
	t.Fatalf("no live finding: %+v", report.Servers[0].Findings)
	return nil, CheckFinding{}
}

func assertNoSecret(t *testing.T, report *CheckReport) {
	t.Helper()
	data, _ := json.Marshal(report)
	if strings.Contains(string(data), liveSecret) {
		t.Fatalf("report leaks a configured value: %s", data)
	}
}

var tokenEnv = map[string]string{"LIVE_TOKEN": liveSecret}

func TestLiveStdioDiscover(t *testing.T) {
	report, err := checkService(t, liveConfig("modern", nil)).Check(liveCheck(10*time.Second, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	live, f := liveOf(t, report)
	want := CheckLive{ProtocolVersion: "2026-07-28", ServerInfo: LiveServerInfo{Name: "helper", Version: "1.2.3"}, Tools: 2, ToolNames: []string{"a", "b"}}
	if f.Level != "info" || live == nil || !reflect.DeepEqual(*live, want) || !report.Servers[0].OK {
		t.Fatalf("live = %+v, finding = %+v", live, f)
	}
}

func TestLiveStdioFallsBackToInitialize(t *testing.T) {
	report, err := checkService(t, liveConfig("legacy", nil)).Check(liveCheck(10*time.Second, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	live, f := liveOf(t, report)
	want := CheckLive{ProtocolVersion: "2025-11-25", ServerInfo: LiveServerInfo{Name: "legacy", Version: "0.9"}, Tools: 3, ToolNames: []string{"a", "b", "c"}}
	if live == nil || !reflect.DeepEqual(*live, want) {
		t.Fatalf("live = %+v, finding = %+v", live, f)
	}
}

func TestLiveStdioFallsBackWhenDiscoverIsUnanswered(t *testing.T) {
	report, err := checkService(t, liveConfig("silent", nil)).Check(liveCheck(3*time.Second, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	if live, f := liveOf(t, report); live == nil || live.ProtocolVersion != "2025-11-25" {
		t.Fatalf("live = %+v, finding = %+v", live, f)
	}
}

func TestLiveStdioExitReportsRedactedStderr(t *testing.T) {
	report, err := checkService(t, liveConfig("crash", nil)).Check(liveCheck(10*time.Second, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	_, f := liveOf(t, report)
	if f.Level != "error" || !strings.Contains(f.Message, "exit status 3") || !strings.Contains(f.Message, "token [redacted] rejected") || report.Summary.Errors != 1 {
		t.Fatalf("finding = %+v", f)
	}
	assertNoSecret(t, report)
}

func TestLiveStdioSpawnFailure(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    gone:\n      command: /no/such/mcp-server\n")
	opts := liveCheck(time.Second, nil)
	// The static check passes; starting the command is what fails.
	opts.LookPath = func(name string) (string, error) { return name, nil }
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, f := liveOf(t, report); f.Level != "error" || !strings.Contains(f.Message, "could not start /no/such/mcp-server") {
		t.Fatalf("finding = %+v", f)
	}
}

func TestLiveAbsentStartsNothing(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	opts := liveCheck(time.Second, tokenEnv)
	opts.Live = false
	report, err := checkService(t, liveConfig("modern", map[string]string{"MCP_LIVE_MARKER": marker})).Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a check without Live started the server")
	}
	if report.Servers[0].Live != nil || subjectOf(report.Servers[0].Findings, "live") != "" || hasFinding(report.Servers[0].Findings, "info", "live", "") {
		t.Fatalf("unexpected live result: %+v", report.Servers[0])
	}
}

func TestLiveSkipsServerWithStaticError(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	report, err := checkService(t, liveConfig("modern", map[string]string{"MCP_LIVE_MARKER": marker})).Check(liveCheck(time.Second, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, f := liveOf(t, report); f.Level != "info" || !strings.Contains(f.Message, "not probed live") {
		t.Fatalf("finding = %+v", f)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a server with a missing variable was started")
	}
}

// liveHTTP serves a check config for one remote server backed by handler.
func liveHTTP(t *testing.T, handler http.HandlerFunc) (*CheckReport, CheckFinding) {
	t.Helper()
	ts := httptest.NewServer(handler)
	defer ts.Close()
	s := checkService(t, fmt.Sprintf("mcp:\n  targets: [claude]\n  servers:\n    remote:\n      url: %s/mcp\n      bearerToken: {fromEnv: LIVE_TOKEN}\n      headers:\n        X-Team: {fromEnv: LIVE_TEAM}\n", ts.URL))
	report, err := s.Check(liveCheck(5*time.Second, map[string]string{"LIVE_TOKEN": liveSecret, "LIVE_TEAM": "team-" + liveSecret}))
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, report)
	_, f := liveOf(t, report)
	return report, f
}

type httpRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

func decodeRequest(t *testing.T, r *http.Request) httpRequest {
	var req httpRequest
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		t.Errorf("invalid request body %s", body)
	}
	if r.Header.Get("Authorization") != "Bearer "+liveSecret || r.Header.Get("X-Team") != "team-"+liveSecret || r.Header.Get("Mcp-Method") != req.Method {
		t.Errorf("%s: missing or wrong headers: %v", req.Method, r.Header)
	}
	return req
}

func TestLiveHTTPDiscoverJSON(t *testing.T) {
	report, f := liveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		req := decodeRequest(t, r)
		if r.Header.Get("MCP-Protocol-Version") != "2026-07-28" {
			t.Errorf("missing protocol version header")
		}
		w.Header().Set("Content-Type", "application/json")
		result := map[string]any{"resultType": "complete", "tools": []any{map[string]string{"name": "x"}}}
		if req.Method == "server/discover" {
			result = map[string]any{"resultType": "complete", "supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{"tools": map[string]any{}},
				"_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]string{"name": "remote", "version": "2"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	})
	want := CheckLive{ProtocolVersion: "2026-07-28", ServerInfo: LiveServerInfo{Name: "remote", Version: "2"}, Tools: 1, ToolNames: []string{"x"}}
	if f.Level != "info" || report.Servers[0].Live == nil || !reflect.DeepEqual(*report.Servers[0].Live, want) {
		t.Fatalf("live = %+v, finding = %+v", report.Servers[0].Live, f)
	}
}

func TestLiveHTTPSSEResponse(t *testing.T) {
	report, f := liveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		req := decodeRequest(t, r)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": keep-alive\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")
		result, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{},
			"_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]string{"name": "sse", "version": "1"}}}})
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", result)
	})
	if f.Level != "info" || report.Servers[0].Live == nil || report.Servers[0].Live.ServerInfo.Name != "sse" || report.Servers[0].Live.Tools != 0 {
		t.Fatalf("live = %+v, finding = %+v", report.Servers[0].Live, f)
	}
}

func TestLiveHTTPFallsBackToInitialize(t *testing.T) {
	report, f := liveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			return
		}
		req := decodeRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == "initialize":
			w.Header().Set("Mcp-Session-Id", "session-1")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "old", "version": "1"}}})
		case r.Header.Get("Mcp-Session-Id") != "session-1":
			http.Error(w, "Bad Request: No valid session ID provided", http.StatusBadRequest)
		case req.Method == "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case r.Header.Get("MCP-Protocol-Version") != "2025-06-18":
			http.Error(w, "wrong protocol version", http.StatusBadRequest)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"tools": []any{map[string]string{"name": "x"}, map[string]string{"name": "y"}}}})
		}
	})
	want := CheckLive{ProtocolVersion: "2025-06-18", ServerInfo: LiveServerInfo{Name: "old", Version: "1"}, Tools: 2, ToolNames: []string{"x", "y"}}
	if report.Servers[0].Live == nil || !reflect.DeepEqual(*report.Servers[0].Live, want) {
		t.Fatalf("live = %+v, finding = %+v", report.Servers[0].Live, f)
	}
}

func TestLiveHTTPUnauthorizedIsWarning(t *testing.T) {
	calls := 0
	report, f := liveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://auth.example.com/.well-known/oauth-protected-resource", scope="files:read"`)
		w.WriteHeader(http.StatusUnauthorized)
	})
	if f.Level != "warning" || f.Subject != "https://auth.example.com/.well-known/oauth-protected-resource" || !strings.Contains(f.Message, "sign-in required") || !report.Servers[0].OK || calls != 1 {
		t.Fatalf("finding = %+v, calls = %d", f, calls)
	}
}

func TestLiveHTTPServerErrorIsError(t *testing.T) {
	report, f := liveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom "+r.Header.Get("Authorization"), http.StatusInternalServerError)
	})
	if f.Level != "error" || !strings.Contains(f.Message, "HTTP 500") || report.Servers[0].OK {
		t.Fatalf("finding = %+v", f)
	}
}
