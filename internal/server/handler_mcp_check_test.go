package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofrs/flock"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

func writeMCPSource(t *testing.T, s *Server, mcp string) {
	t.Helper()
	f, err := os.OpenFile(s.configPath(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(mcp); err != nil {
		t.Fatal(err)
	}
}

func TestMCPCheckReportsFindingsWithOK(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("SKILLSHARE_TEST_UNSET_TOKEN", "")
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://mcp.skillshare.invalid/mcp\n      bearerToken: {fromEnv: SKILLSHARE_TEST_UNSET_TOKEN}\n")
	w := httptest.NewRecorder()
	s.handleMCPCheck(w, httptest.NewRequest(http.MethodGet, "/api/mcp/check?dns=0", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var report struct {
		Servers []struct {
			Name     string `json:"name"`
			OK       bool   `json:"ok"`
			Findings []struct {
				Level   string `json:"level"`
				Check   string `json:"check"`
				Subject string `json:"subject"`
			} `json:"findings"`
		} `json:"servers"`
		Summary struct {
			Errors int `json:"errors"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Servers) != 1 || report.Servers[0].OK || report.Summary.Errors != 1 {
		t.Fatalf("the unset token must be one error: %s", w.Body.String())
	}
	for _, f := range report.Servers[0].Findings {
		if f.Check == "dns" {
			t.Fatalf("dns=0 must skip host lookups: %s", w.Body.String())
		}
		if f.Check == "env" && f.Subject != "SKILLSHARE_TEST_UNSET_TOKEN" {
			t.Fatalf("env subject = %q", f.Subject)
		}
	}
}

func TestMCPCheckFailsWhenCheckCannotRun(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	writeMCPSource(t, s, "mcp:\n  servers: [not, a, map]\n")
	w := httptest.NewRecorder()
	s.handleMCPCheck(w, httptest.NewRequest(http.MethodGet, "/api/mcp/check", nil))
	if w.Code < 400 || !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
}

func probeDraft(t *testing.T, s *Server, server string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleMCPProbe(w, httptest.NewRequest(http.MethodPost, "/api/mcp/probe", strings.NewReader(`{"mutation":{"name":"docs","server":`+server+`}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMCPProbeListsToolsOfAnUnsavedServer(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := map[string]any{"tools": []any{map[string]any{"name": "search", "inputSchema": map[string]any{"type": "object"}}, map[string]any{"name": "fetch", "inputSchema": map[string]any{"type": "object"}}}}
		if req.Method == "server/discover" {
			result = map[string]any{"supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer mcpServer.Close()
	before, err := os.ReadFile(s.configPath())
	if err != nil {
		t.Fatal(err)
	}
	result := probeDraft(t, s, `{"url":"`+mcpServer.URL+`/mcp","tools":{"allow":["half typed"]}}`)
	live, _ := result["live"].(map[string]any)
	if live == nil || live["tools"] != float64(2) || result["errorKind"] != nil {
		t.Fatalf("result = %v", result)
	}
	if after, _ := os.ReadFile(s.configPath()); string(after) != string(before) {
		t.Fatal("a probe must not save the server")
	}
}

func TestMCPProbeClassifiesARefusedConnection(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	result := probeDraft(t, s, `{"url":"`+closed.URL+`/mcp"}`)
	if result["errorKind"] != "connect" || !strings.Contains(result["error"].(string), "refused") {
		t.Fatalf("result = %v", result)
	}
}

// A live probe of a slow server must not hold the config lock, or every other API
// request waits for it.
func TestMCPCheckLiveDoesNotHoldTheConfigLock(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	command := filepath.Join(t.TempDir(), "slow-mcp")
	started, release := writeBlockingCommand(t, command, "")
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    slow:\n      command: "+command+"\n")
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleMCPCheck(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/mcp/check?dns=0&live=1", nil))
	}()
	defer func() { release(); <-done }()
	started()
	if !s.mu.TryLock() {
		t.Fatal("a live probe holds the config lock")
	}
	s.mu.Unlock()
}

type checkContractControl struct {
	mu           sync.Mutex
	tools        json.RawMessage
	onList       func()
	infoName     string
	errorMessage string
	endpoint     string
}

func (c *checkContractControl) setTools(raw string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = json.RawMessage(raw)
}
func (c *checkContractControl) setOnList(fn func()) { c.mu.Lock(); defer c.mu.Unlock(); c.onList = fn }
func (c *checkContractControl) setInfoName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.infoName = name
}
func (c *checkContractControl) setError(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorMessage = message
}
func checkContractFixture(t *testing.T, controls ...*checkContractControl) (*Server, *atomic.Int32) {
	t.Helper()
	s, _ := newTestServerWithExtras(t, nil, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	calls := &atomic.Int32{}
	control := &checkContractControl{}
	if len(controls) > 0 {
		control = controls[0]
	}
	if control.tools == nil {
		control.tools = json.RawMessage(`[{"name":"fixture","inputSchema":{"type":"object","default":900719925474099312345}}]`)
	}
	if control.infoName == "" {
		control.infoName = "fixture"
	}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(200)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			w.WriteHeader(400)
			return
		}
		if len(req.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		calls.Add(1)
		var result any = map[string]any{}
		control.mu.Lock()
		tools, infoName, onList, errorMessage := control.tools, control.infoName, control.onList, control.errorMessage
		control.mu.Unlock()
		switch req.Method {
		case "server/discover":
			result = map[string]any{"supportedVersions": []string{"2026-07-28"}, "capabilities": map[string]any{"tools": map[string]any{}}, "_meta": map[string]any{"io.modelcontextprotocol/serverInfo": map[string]string{"name": infoName, "version": "1"}}}
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": infoName, "version": "1"}}
		case "tools/list":
			if onList != nil {
				onList()
			}
			result = map[string]any{"tools": tools}
		}
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}
		if req.Method == "tools/list" && errorMessage != "" {
			delete(response, "result")
			response["error"] = map[string]any{"code": -32001, "message": errorMessage}
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(endpoint.Close)
	control.endpoint = endpoint.URL
	root := t.TempDir()
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: "+endpoint.URL+"\n  projects:\n    "+root+":\n      servers:\n        docs:\n          url: "+endpoint.URL+"\n")
	return s, calls
}

func checkGet(t *testing.T, s *Server, query string) *mcp.CheckReport {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleMCPCheck(w, httptest.NewRequest(http.MethodGet, "/api/mcp/check?dns=0&"+query, nil))
	if w.Code != 200 {
		t.Fatalf("GET status=%d: %s", w.Code, w.Body.String())
	}
	var report mcp.CheckReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return &report
}

func checkAccept(s *Server, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.handleMCPAcceptTools(w, httptest.NewRequest(http.MethodPost, "/api/mcp/check", strings.NewReader(body)))
	return w
}

func TestMCPCheckRootPresenceAndLiveReadOnly(t *testing.T) {
	s, _ := checkContractFixture(t)
	all := checkGet(t, s, "")
	if len(all.Servers) != 2 {
		t.Fatalf("omitted root: %+v", all)
	}
	own := checkGet(t, s, "root=")
	if len(own.Servers) != 1 || own.Servers[0].Project != "" {
		t.Fatalf("empty root: %+v", own)
	}
	root := ""
	for _, server := range all.Servers {
		if server.Project != "" {
			root = server.Project
		}
	}
	project := checkGet(t, s, "root="+url.QueryEscape(root))
	if len(project.Servers) != 1 || project.Servers[0].Project != root {
		t.Fatalf("project root: %+v", project)
	}
	before, _ := os.ReadFile(s.configPath())
	live := checkGet(t, s, "live=1&root=")
	if live.Servers[0].Contract == nil || live.Servers[0].Contract.Revision == "" {
		t.Fatalf("missing complete review: %+v", live)
	}
	if live.ToolsAcceptance != "" {
		t.Fatal("GET claimed acceptance")
	}
	if after, _ := os.ReadFile(s.configPath()); !bytes.Equal(before, after) {
		t.Fatal("GET changed source")
	}
	if _, err := os.Stat(filepath.Join(config.StateDir(), "mcp", "tool-pins.json")); !os.IsNotExist(err) {
		t.Fatalf("GET wrote baseline: %v", err)
	}
}

func TestMCPAcceptStrictRequest(t *testing.T) {
	s, calls := checkContractFixture(t)
	cases := []string{
		`{`, `null`, `[]`, `{}`, `{"name":"docs","revision":"r"}`,
		`{"name":"docs","root":null,"revision":"r"}`,
		`{"name":"docs","root":"","revision":"r","extra":true}`,
		`{"name":"docs","root":"","revision":"r"} {}`,
		`{"name":"docs","root":"","revision":"r","name":"other"}`,
		`{"name":"docs","root":"","revision":"r","\u006eame":"other"}`,
		`{"name":"docs","root":"","revision":"r","Name":"other"}`,
		`{"name":"docs","root":"","revision":"\ud800"}`,
		`{"name":42,"root":"","revision":"r"}`,
		`{"name":"docs","root":"","revision":"r"} true`,
	}
	for _, body := range cases {
		w := checkAccept(s, body)
		if w.Code != 400 {
			t.Errorf("body %s => %d %s", body, w.Code, w.Body.String())
		}
	}
	oversized := `{"name":"` + strings.Repeat("x", int(defaultJSONBodyLimit)) + `","root":"","revision":"r"}`
	for _, body := range []string{oversized, `{"name":"docs","root":"","revision":"r"}` + strings.Repeat(" ", int(defaultJSONBodyLimit))} {
		if w := checkAccept(s, body); w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized status=%d", w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request probed endpoint")
	}
}

func TestMCPAcceptExactScopeTypedPersistenceAndStale(t *testing.T) {
	s, _ := checkContractFixture(t)
	report := checkGet(t, s, "live=1&root=")
	rev := report.Servers[0].Contract.Revision
	body, _ := json.Marshal(map[string]string{"name": "docs", "root": "", "revision": rev})
	w := checkAccept(s, string(body))
	var result struct {
		Accepted bool            `json:"accepted"`
		Report   mcp.CheckReport `json:"report"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !result.Accepted || result.Report.ToolsAcceptance != mcp.ToolsAcceptanceAccepted || len(result.Report.Servers) != 1 || result.Report.Servers[0].Project != "" {
		t.Fatalf("accept status=%d: %s", w.Code, w.Body.String())
	}
	// The old token binds the pre-write store and cannot approve another observation.
	w = checkAccept(s, string(body))
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"accepted":false`) {
		t.Fatalf("stale status=%d: %s", w.Code, w.Body.String())
	}
	all := checkGet(t, s, "live=1")
	for _, server := range all.Servers {
		want := "unchanged"
		if server.Project != "" {
			want = "untrusted"
		}
		if server.Contract == nil || server.Contract.Status != want {
			t.Fatalf("cross-scope approval: %+v", server)
		}
	}
}

func TestMCPAcceptSkippedIsNotSuccess(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SKILLSHARE_MISSING_FIXTURE", "")
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://mcp.skillshare.invalid/mcp\n      bearerToken: {fromEnv: SKILLSHARE_MISSING_FIXTURE}\n")
	w := checkAccept(s, `{"name":"docs","root":"","revision":"`+strings.Repeat("a", 64)+`"}`)
	if w.Code != 422 || !strings.Contains(w.Body.String(), `"accepted":false`) {
		t.Fatalf("skipped status=%d: %s", w.Code, w.Body.String())
	}
}

func TestMCPAcceptDoesNotHoldConfigLock(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	command := filepath.Join(t.TempDir(), "slow-mcp")
	started, release := writeBlockingCommand(t, command, "")
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    slow:\n      command: "+command+"\n")
	done := make(chan struct{})
	go func() {
		defer close(done)
		checkAccept(s, `{"name":"slow","root":"","revision":"`+strings.Repeat("a", 64)+`"}`)
	}()
	defer func() { release(); <-done }()
	started()
	if !s.mu.TryLock() {
		t.Fatal("accept probe holds config lock")
	}
	s.mu.Unlock()
}

func TestMCPAcceptRequiresLocalBrowser(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/api/mcp/check", strings.NewReader(`{"name":"docs","root":"","revision":"r"}`))
	req.RemoteAddr = "192.0.2.1:1234"
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("remote acceptance status=%d", w.Code)
	}
}

func TestMCPCheckWireSnapshotPreservesExactNumbers(t *testing.T) {
	s, _ := checkContractFixture(t)
	w := httptest.NewRecorder()
	s.handleMCPCheck(w, httptest.NewRequest(http.MethodGet, "/api/mcp/check?dns=0&live=1&root=", nil))
	var wire struct {
		Servers []struct {
			Contract struct {
				Tools map[string]struct {
					DefinitionText string          `json:"definitionText"`
					Definition     json.RawMessage `json:"definition"`
				} `json:"tools"`
			} `json:"contract"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Servers) != 1 {
		t.Fatalf("missing wire server: %s", w.Body.String())
	}
	tool := wire.Servers[0].Contract.Tools["fixture"]
	if !strings.Contains(tool.DefinitionText, "900719925474099312345") || len(tool.Definition) != 0 {
		t.Fatalf("snapshot lost fidelity or duplicates raw JSON: %+v", tool)
	}
}

func TestMCPCheckWireRetainsCompleteEmptyInventory(t *testing.T) {
	report := &mcp.CheckReport{Servers: []mcp.CheckServer{{Name: "empty", Contract: &mcp.ContractReview{Status: "untrusted", Revision: strings.Repeat("a", 64), Tools: map[string]mcp.ToolContract{}}}}}
	raw, err := json.Marshal(mcpCheckWire(report))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"tools":{}`) {
		t.Fatalf("complete empty inventory must remain inspectable: %s", raw)
	}
}

func TestMCPCheckProjectModeOwnRoot(t *testing.T) {
	s, _ := newTestProjectServerWithExtras(t, nil)
	t.Setenv("HOME", t.TempDir())
	writeMCPSource(t, s, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: http://127.0.0.1:1/mcp\n")
	report := checkGet(t, s, "root=")
	if len(report.Servers) != 1 || report.Servers[0].Name != "docs" || report.Servers[0].Project != "" {
		t.Fatalf("project config own list: %+v", report)
	}
}

func TestMCPAcceptLogsOnlyTypedOutcomeAndNames(t *testing.T) {
	s, _ := checkContractFixture(t)
	report := checkGet(t, s, "live=1&root=")
	rev := report.Servers[0].Contract.Revision
	body, _ := json.Marshal(map[string]string{"name": "docs", "root": "", "revision": rev})
	if w := checkAccept(s, string(body)); w.Code != 200 {
		t.Fatalf("accept: %s", w.Body.String())
	}
	if w := checkAccept(s, string(body)); w.Code != 409 {
		t.Fatalf("stale: %s", w.Body.String())
	}
	result, err := oplog.Read(s.configPath(), oplog.OpsFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, entry := range result {
		if entry.Command != "mcp check --accept-tools" {
			continue
		}
		statuses = append(statuses, entry.Status)
		raw, _ := json.Marshal(entry.Args)
		if !bytes.Contains(raw, []byte(`"names":["docs"]`)) || bytes.Contains(raw, []byte(rev)) || bytes.Contains(raw, []byte("definition")) {
			t.Fatalf("unexpected log args: %s", raw)
		}
	}
	if len(statuses) != 2 || statuses[0] == statuses[1] {
		t.Fatalf("typed success/failure log = %v", statuses)
	}
}

func TestMCPAcceptCannotReuseReviewInAnotherRoot(t *testing.T) {
	s, calls := checkContractFixture(t)
	report := checkGet(t, s, "live=1")
	var global, project mcp.CheckServer
	for _, server := range report.Servers {
		if server.Project == "" {
			global = server
		} else {
			project = server
		}
	}
	body, _ := json.Marshal(map[string]string{"name": "docs", "root": project.Project, "revision": global.Contract.Revision})
	if w := checkAccept(s, string(body)); w.Code != 409 {
		t.Fatalf("wrong-scope review status=%d: %s", w.Code, w.Body.String())
	}
	before := calls.Load()
	body, _ = json.Marshal(map[string]string{"name": "docs", "root": "/not-configured", "revision": project.Contract.Revision})
	if w := checkAccept(s, string(body)); w.Code != 422 {
		t.Fatalf("unknown scope status=%d", w.Code)
	}
	if calls.Load() != before {
		t.Fatal("unknown root started a probe")
	}
}

func checkApproval(t *testing.T, server mcp.CheckServer) string {
	t.Helper()
	if server.Contract == nil || server.Contract.Revision == "" {
		t.Fatalf("no review revision: %+v", server)
	}
	raw, err := json.Marshal(map[string]string{"name": server.Name, "root": server.Project, "revision": server.Contract.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func checkAcceptanceOutcome(t *testing.T, w *httptest.ResponseRecorder, status int, outcome mcp.ToolsAcceptance) {
	t.Helper()
	var result struct {
		Accepted bool            `json:"accepted"`
		Report   mcp.CheckReport `json:"report"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != status || result.Accepted != (outcome == mcp.ToolsAcceptanceAccepted) || result.Report.ToolsAcceptance != outcome {
		t.Fatalf("status=%d outcome=%s: %s", w.Code, outcome, w.Body.String())
	}
	// Every failure response must also leave the persisted-state report ineligible
	// for another acceptance action.
	if !result.Accepted {
		for _, server := range result.Report.Servers {
			if server.Contract != nil && (server.Contract.Status == "accepted" || server.Contract.Revision != "") {
				t.Fatalf("refusal retained an acceptance action: %+v", server)
			}
		}
	}
}

func checkAcceptanceLog(t *testing.T, s *Server, accepted bool) {
	t.Helper()
	entries, err := oplog.Read(s.configPath(), oplog.OpsFile, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := "error"
	if accepted {
		want = "ok"
	}
	if len(entries) != 1 || entries[0].Command != "mcp check --accept-tools" || entries[0].Status != want {
		t.Fatalf("typed outcome/log mismatch: %+v", entries)
	}
	for key := range entries[0].Args {
		if key != "names" {
			t.Fatalf("unexpected logged content: %s", key)
		}
	}
}

func checkStoreBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.StateDir(), "mcp", "tool-pins.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMCPAcceptRouteGuardMatrix(t *testing.T) {
	s, calls := checkContractFixture(t)
	for _, bind := range []string{"127.0.0.1:19420", "0.0.0.0:19420"} {
		s.addr = bind
		for _, tt := range []struct{ host, remote, origin, site string }{
			{"localhost:19420", "127.0.0.1:1234", "", "cross-site"},
			{"localhost:19420", "127.0.0.1:1234", "https://foreign.invalid", "same-origin"},
			{"rebind.invalid:19420", "127.0.0.1:1234", "http://rebind.invalid:19420", "same-origin"},
			{"rebind.invalid:19420", "192.0.2.10:1234", "http://rebind.invalid:19420", "same-origin"},
			{"127.0.0.1:19420", "127.0.0.1:1234", "http://localhost:19420", "same-origin"},
		} {
			req := httptest.NewRequest(http.MethodPost, "/api/mcp/check", strings.NewReader(`{"name":"docs","root":"","revision":"`+strings.Repeat("a", 64)+`"}`))
			req.Host, req.RemoteAddr = tt.host, tt.remote
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.site != "" {
				req.Header.Set("Sec-Fetch-Site", tt.site)
			}
			w := httptest.NewRecorder()
			s.mux.ServeHTTP(w, req)
			if w.Code != 403 {
				t.Fatalf("bind=%s case=%+v status=%d: %s", bind, tt, w.Code, w.Body.String())
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("guard refusal started a probe")
	}
	if _, err := os.Stat(filepath.Join(config.StateDir(), "mcp", "tool-pins.json")); !os.IsNotExist(err) {
		t.Fatalf("guard refusal wrote store: %v", err)
	}
	// Reach the same registered POST route with a valid local request and exact token.
	for _, host := range []string{"localhost:19420", "127.0.0.1:19420"} {
		report := checkGet(t, s, "live=1&root=")
		req := httptest.NewRequest(http.MethodPost, "/api/mcp/check", strings.NewReader(checkApproval(t, report.Servers[0])))
		req.Host, req.RemoteAddr = host, "127.0.0.1:1234"
		req.Header.Set("Origin", "http://"+host)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		checkAcceptanceOutcome(t, w, 200, mcp.ToolsAcceptanceAccepted)
	}
}

func TestMCPAcceptHandlerRequiresSameOriginBeforeProbeAndPersistence(t *testing.T) {
	s, calls := checkContractFixture(t)
	s.addr = "127.0.0.1:19420"
	post := func(site, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/mcp/check", strings.NewReader(body))
		req.Host, req.RemoteAddr = "localhost:19420", "127.0.0.1:1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:19420")
		req.Header.Set("Sec-Fetch-Site", site)
		w := httptest.NewRecorder()
		s.handler.ServeHTTP(w, req)
		return w
	}
	review := checkGet(t, s, "live=1&root=")
	before := calls.Load()
	// The matching Origin passes the MCP route guard; only the outer handler
	// rejects same-site metadata before this otherwise valid approval can run.
	w := post("same-site", checkApproval(t, review.Servers[0]))
	if w.Code != http.StatusForbidden {
		t.Fatalf("same-site acceptance status=%d: %s", w.Code, w.Body.String())
	}
	if calls.Load() != before {
		t.Fatal("same-site refusal started an acceptance probe")
	}
	pinsPath := filepath.Join(config.StateDir(), "mcp", "tool-pins.json")
	if _, err := os.Stat(pinsPath); !os.IsNotExist(err) {
		t.Fatalf("same-site refusal wrote a baseline: %v", err)
	}
	fresh := checkGet(t, s, "live=1&root=")
	before = calls.Load()
	w = post("same-origin", checkApproval(t, fresh.Servers[0]))
	checkAcceptanceOutcome(t, w, http.StatusOK, mcp.ToolsAcceptanceAccepted)
	if calls.Load() <= before {
		t.Fatal("same-origin acceptance never reached its live reprobe")
	}
	if _, err := os.Stat(pinsPath); err != nil {
		t.Fatalf("same-origin acceptance did not persist a baseline: %v", err)
	}
}

func TestMCPAcceptFreshNoopIsByteStable(t *testing.T) {
	s, _ := checkContractFixture(t)
	first := checkGet(t, s, "live=1&root=")
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, first.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
	before := checkStoreBytes(t)
	fresh := checkGet(t, s, "live=1&root=")
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, fresh.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
	if !bytes.Equal(before, checkStoreBytes(t)) {
		t.Fatal("no-op acceptance rewrote the baseline")
	}
}

func TestMCPAcceptSecondProbeFailurePreservesBaseline(t *testing.T) {
	for _, tt := range []struct {
		name, tools string
		status      int
		outcome     mcp.ToolsAcceptance
	}{
		{"changed", `[{"name":"fixture","description":"Changed since review","inputSchema":{"type":"object"}}]`, 409, mcp.ToolsAcceptanceStale},
		{"malformed", `[{"name":"fixture"}]`, 422, mcp.ToolsAcceptanceError},
		{"missing", `null`, 422, mcp.ToolsAcceptanceError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			control := &checkContractControl{}
			s, _ := checkContractFixture(t, control)
			initial := checkGet(t, s, "live=1&root=")
			checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, initial.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
			review := checkGet(t, s, "live=1&root=")
			before := checkStoreBytes(t)
			control.setTools(tt.tools)
			checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, review.Servers[0])), tt.status, tt.outcome)
			checkAcceptanceLog(t, s, false)
			if !bytes.Equal(before, checkStoreBytes(t)) {
				t.Fatal("failed reprobe changed prior baseline")
			}
		})
	}
}

func TestMCPAcceptSourceAndStoreRacesPreserveBaseline(t *testing.T) {
	for _, race := range []string{"source", "store"} {
		t.Run(race, func(t *testing.T) {
			control := &checkContractControl{}
			s, _ := checkContractFixture(t, control)
			first := checkGet(t, s, "live=1&root=")
			checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, first.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
			otherApproval := ""
			if race == "store" {
				for _, server := range checkGet(t, s, "live=1").Servers {
					if server.Project != "" {
						otherApproval = checkApproval(t, server)
					}
				}
			}
			review := checkGet(t, s, "live=1&root=")
			before := checkStoreBytes(t)
			expected := before
			callbackResult := make(chan error, 1)
			control.setOnList(func() {
				if race == "source" {
					data, err := os.ReadFile(s.configPath())
					if err != nil {
						callbackResult <- err
						return
					}
					callbackResult <- os.WriteFile(s.configPath(), append(data, []byte("\n# concurrent fixture edit\n")...), 0600)
				} else {
					// A supported approval of another configured root is the competing
					// writer. Remove the fixture callback before its nested live probe.
					control.setOnList(nil)
					response := checkAccept(s, otherApproval)
					if response.Code != 200 {
						callbackResult <- fmt.Errorf("concurrent fixture approval: %d %s", response.Code, response.Body.String())
						return
					}
					var err error
					expected, err = os.ReadFile(filepath.Join(config.StateDir(), "mcp", "tool-pins.json"))
					callbackResult <- err
				}
			})
			w := checkAccept(s, checkApproval(t, review.Servers[0]))
			select {
			case err := <-callbackResult:
				if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatal("acceptance did not reach controlled reprobe")
			}
			checkAcceptanceOutcome(t, w, 409, mcp.ToolsAcceptanceStale)
			checkAcceptanceLog(t, s, false)
			if !bytes.Equal(expected, checkStoreBytes(t)) {
				t.Fatal("failed CAS overwrote another writer's baseline")
			}
			if race == "store" {
				if bytes.Equal(before, expected) {
					t.Fatal("competing approval did not write another record")
				}
				for _, server := range checkGet(t, s, "live=1").Servers {
					if server.Contract == nil || server.Contract.Status != "unchanged" {
						t.Fatalf("competing approval was lost: %+v", server)
					}
				}
			}
		})
	}
}

func TestMCPAcceptBusyPersistenceReturnsFailure(t *testing.T) {
	control := &checkContractControl{}
	s, _ := checkContractFixture(t, control)
	initial := checkGet(t, s, "live=1&root=")
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, initial.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
	control.setTools(`[{"name":"fixture","description":"New reviewed text","inputSchema":{"type":"object"}}]`)
	review := checkGet(t, s, "live=1&root=")
	before := checkStoreBytes(t)
	lock := flock.New(filepath.Join(config.StateDir(), "mcp", "operation.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		t.Fatalf("fixture lock: %v, %v", locked, err)
	}
	defer lock.Unlock()
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, review.Servers[0])), 422, mcp.ToolsAcceptanceError)
	checkAcceptanceLog(t, s, false)
	if !bytes.Equal(before, checkStoreBytes(t)) {
		t.Fatal("failed persistence changed prior baseline")
	}
}

func TestMCPAcceptAtomicWriteFailureReturnsFailure(t *testing.T) {
	control := &checkContractControl{}
	s, _ := checkContractFixture(t, control)
	first := checkGet(t, s, "live=1&root=")
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, first.Servers[0])), 200, mcp.ToolsAcceptanceAccepted)
	control.setTools(`[{"name":"fixture","description":"New reviewed text","inputSchema":{"type":"object"}}]`)
	review := checkGet(t, s, "live=1&root=")
	before := checkStoreBytes(t)
	directory := filepath.Join(config.StateDir(), "mcp")
	if err := os.Chmod(directory, 0500); err != nil {
		t.Skipf("cannot restrict fixture directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0700); err != nil {
			t.Error(err)
		}
	})
	probe, err := os.CreateTemp(directory, "write-permission-probe-*")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Skip("fixture filesystem permits writes despite directory mode")
	}
	checkAcceptanceOutcome(t, checkAccept(s, checkApproval(t, review.Servers[0])), 422, mcp.ToolsAcceptanceError)
	checkAcceptanceLog(t, s, false)
	if !bytes.Equal(before, checkStoreBytes(t)) {
		t.Fatal("failed atomic write changed prior baseline")
	}
}

func TestMCPAcceptProjectConfigSharesStateWithoutSharingTrust(t *testing.T) {
	control := &checkContractControl{}
	global, _ := checkContractFixture(t, control)
	globalPath, sharedStateHome := global.configPath(), os.Getenv("XDG_STATE_HOME")
	project, _ := newTestProjectServerWithExtras(t, nil)
	t.Setenv("SKILLSHARE_CONFIG", globalPath)
	t.Setenv("XDG_STATE_HOME", sharedStateHome)
	writeMCPSource(t, project, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: "+control.endpoint+"\n")
	observed := checkGet(t, global, "live=1&root=")
	oldGlobalBody := checkApproval(t, observed.Servers[0])
	checkAcceptanceOutcome(t, checkAccept(global, oldGlobalBody), 200, mcp.ToolsAcceptanceAccepted)
	projected := checkGet(t, project, "live=1&root=")
	if len(projected.Servers) != 1 || projected.Servers[0].Project != "" || projected.Servers[0].Contract.Status != "untrusted" {
		t.Fatalf("project inherited global trust: %+v", projected)
	}
	checkAcceptanceOutcome(t, checkAccept(project, oldGlobalBody), 409, mcp.ToolsAcceptanceStale)
	projectBody := checkApproval(t, projected.Servers[0])
	checkAcceptanceOutcome(t, checkAccept(project, projectBody), 200, mcp.ToolsAcceptanceAccepted)
	checkAcceptanceOutcome(t, checkAccept(project, projectBody), 409, mcp.ToolsAcceptanceStale)
	for _, server := range checkGet(t, global, "live=1").Servers {
		want := "unchanged"
		if server.Project != "" {
			want = "untrusted"
		}
		if server.Contract == nil || server.Contract.Status != want {
			t.Fatalf("project write changed global/root trust: %+v", server)
		}
	}
	if server := checkGet(t, project, "live=1&root=").Servers[0]; server.Contract.Status != "unchanged" {
		t.Fatalf("project trust not retained: %+v", server)
	}
}

func TestMCPCheckKnownShortSecretsNeverReachReviewWire(t *testing.T) {
	for _, part := range []string{"description", "key", "name", "serverInfo", "identity", "diagnostic"} {
		t.Run(part, func(t *testing.T) {
			control := &checkContractControl{}
			s, _ := checkContractFixture(t, control)
			t.Setenv("FIXTURE_SHORT_SECRET", "§")
			t.Setenv("FIXTURE_OVERLAP_SECRET", "§§")
			source, err := os.ReadFile(s.configPath())
			if err != nil {
				t.Fatal(err)
			}
			source = bytes.ReplaceAll(source, []byte("url: "+control.endpoint), []byte("headers:\n        X-One: {fromEnv: FIXTURE_SHORT_SECRET}\n        X-Two: {fromEnv: FIXTURE_OVERLAP_SECRET}\n      url: "+control.endpoint))
			// Scope this source to the own list; the shared project entry is not needed here.
			source = bytes.SplitN(source, []byte("  projects:"), 2)[0]
			switch part {
			case "description":
				control.setTools(`[{"name":"fixture","description":"inert §§ fixture","inputSchema":{"type":"object"}}]`)
			case "key":
				control.setTools(`[{"name":"fixture","inputSchema":{"type":"object"},"_meta":{"§§":"fixture"}}]`)
			case "name":
				control.setTools(`[{"name":"fixture§§","inputSchema":{"type":"object"}}]`)
			case "diagnostic":
				control.setError("fixture§§\u001b]52;c;inert\u0007\u202e")
			case "serverInfo":
				control.setInfoName("fixture§§")
			case "identity":
				source = bytes.ReplaceAll(source, []byte(control.endpoint), []byte(control.endpoint+"/§§"))
			}
			if err := os.WriteFile(s.configPath(), source, 0600); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			s.handleMCPCheck(w, httptest.NewRequest(http.MethodGet, "/api/mcp/check?dns=0&live=1&root=", nil))
			if w.Code != 200 {
				t.Fatalf("check failed to return report: %d %s", w.Code, w.Body.String())
			}
			for _, secret := range []string{"§", `\u00a7`} {
				if strings.Contains(strings.ToLower(w.Body.String()), secret) {
					t.Fatalf("known short credential reached wire: %s", w.Body.String())
				}
			}
			var report mcp.CheckReport
			if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if len(report.Servers) != 1 || report.Servers[0].OK || report.Servers[0].Live != nil || report.Servers[0].Contract != nil && (report.Servers[0].Contract.Revision != "" || len(report.Servers[0].Contract.Tools) > 0) {
				t.Fatalf("credential-bearing inventory received review: %+v", report)
			}
			if _, err := os.Stat(filepath.Join(config.StateDir(), "mcp", "tool-pins.json")); !os.IsNotExist(err) {
				t.Fatal("credential-bearing review wrote state")
			}
		})
	}
}

func TestMCPAcceptConfiguredCredentialInServerLabelNeverReachesLogs(t *testing.T) {
	s, calls := checkContractFixture(t)
	const secret = "docs" // Valid server label, now also a configured credential value.
	t.Setenv("FIXTURE_LABEL_CREDENTIAL", secret)
	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.SplitN(raw, []byte("  projects:"), 2)[0]
	raw = bytes.Replace(raw, []byte("    docs:\n"), []byte("    docs:\n      headers:\n        X-Token: {fromEnv: FIXTURE_LABEL_CREDENTIAL}\n"), 1)
	if err := os.WriteFile(s.configPath(), raw, 0600); err != nil {
		t.Fatal(err)
	}
	response := checkAccept(s, `{"name":"docs","root":"","revision":"`+strings.Repeat("a", 64)+`"}`)
	checkAcceptanceOutcome(t, response, 422, mcp.ToolsAcceptanceError)
	if calls.Load() != 0 {
		t.Fatal("credential-bearing label started a probe")
	}
	for _, file := range []string{oplog.OpsFile, oplog.AuditFile} {
		logged, err := os.ReadFile(filepath.Join(oplog.LogDir(s.configPath()), file))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(logged, []byte(secret)) {
			t.Fatalf("%s contains the fixture credential", file)
		}
		entries, err := oplog.Read(s.configPath(), file, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Status != "error" {
			t.Fatalf("%s outcome=%+v", file, entries)
		}
	}
	if bytes.Contains(response.Body.Bytes(), []byte(`"name":"docs"`)) {
		t.Fatal("refusal report retained the credential-bearing label")
	}
}
