package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type inventoryConn struct {
	pages []string
	calls int
}

func TestLiveStdioBlockedWritesRespectContext(t *testing.T) {
	for _, notification := range []bool{false, true} {
		t.Run(fmt.Sprint(notification), func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			var waitErr error
			started := make(chan struct{})
			conn := newStdioConn(&signaledWriteCloser{writer, started}, strings.NewReader(""), make(chan struct{}), &waitErr)
			defer close(conn.stop)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				params := map[string]string{"cursor": strings.Repeat("x", 1<<20)}
				if notification {
					result <- conn.notify(ctx, "notifications/initialized", params)
					return
				}
				_, err := conn.call(ctx, "tools/list", params)
				result <- err
			}()
			select {
			case <-started:
				cancel()
			case <-time.After(time.Second):
				t.Fatal("stdio request never reached writer")
			}
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("write cancellation: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked stdio write outlived probe context")
			}
		})
	}
}

type bufferWriteCloser struct{ bytes.Buffer }

func (*bufferWriteCloser) Close() error { return nil }

type signaledWriteCloser struct {
	*io.PipeWriter
	started chan struct{}
}

func (w *signaledWriteCloser) Write(p []byte) (int, error) {
	close(w.started)
	return w.PipeWriter.Write(p)
}

func TestCheckPreflightCannotLeakOrResolveKnownCredentials(t *testing.T) {
	for _, entry := range []string{
		"url: https://fixturetoken.invalid/mcp\n      bearerToken: {fromEnv: FIXTURE_TOKEN}",
		"command: fixturetoken-command\n      env:\n        API_TOKEN: {fromEnv: FIXTURE_TOKEN}",
		"command: npx\n      args: [fixturetoken-package@latest]\n      env:\n        API_TOKEN: {fromEnv: FIXTURE_TOKEN}",
	} {
		t.Run(entry[:3], func(t *testing.T) {
			s := checkService(t, "mcp:\n  targets: []\n  servers:\n    provider:\n      "+entry+"\n")
			lookups := 0
			report, err := s.Check(CheckOptions{Live: true, LookupEnv: func(string) (string, bool) { return "fixturetoken", true }, LookPath: func(string) (string, error) { lookups++; return "", errors.New("missing") }, LookupHost: func(context.Context, string) ([]string, error) { lookups++; return nil, errors.New("missing") }})
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(report)
			if lookups != 0 || strings.Contains(string(raw), "fixturetoken") || report.Summary.Errors == 0 {
				t.Fatalf("unsafe preflight: lookups=%d %s", lookups, raw)
			}
		})
	}
}

func TestLiveLegacyServerInfoMustBePresentAndTyped(t *testing.T) {
	for _, info := range []string{"", `,"serverInfo":null`, `,"serverInfo":{"name":null,"version":"1"}`, `,"serverInfo":{"name":"fixture","version":null}`} {
		conn := &inventoryConn{pages: []string{`{"protocolVersion":"2025-11-25","capabilities":{}` + info + `}`}}
		p := newLiveProbe(Server{}, "", DefaultLiveTimeout, CheckOptions{})
		if _, err := p.initialize(context.Background(), conn); err == nil {
			t.Fatalf("malformed legacy serverInfo accepted: %s", info)
		}
	}
}

func TestCheckUnknownSelectionCannotEchoConfiguredCredentialLabels(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: []\n  servers:\n    fixturetoken:\n      command: fixture\n      env:\n        API_TOKEN: {fromEnv: FIXTURE_TOKEN}\n")
	_, err := s.Check(CheckOptions{Names: []string{"absent"}, LookupEnv: func(string) (string, bool) { return "fixturetoken", true }})
	if err == nil || strings.Contains(err.Error(), "fixturetoken") {
		t.Fatalf("unsafe selection diagnostic: %v", err)
	}
}

func TestProbeDraftPreflightRefusesCredentialBeforeCommandLookup(t *testing.T) {
	lookups := 0
	got, err := ProbeDraft(Server{Command: "/tmp/fixturetoken/mcp", Env: map[string]Value{"TOKEN": {FromEnv: "FIXTURE_TOKEN"}}}, "", CheckOptions{
		LookupEnv: func(string) (string, bool) { return "fixturetoken", true },
		LookPath:  func(string) (string, error) { lookups++; return "", errors.New("fixturetoken missing") },
	})
	if err != nil || got.Error == "" || strings.Contains(got.Error, "fixturetoken") || got.Live != nil || lookups != 0 {
		t.Fatalf("unsafe draft preflight: %+v %v lookups=%d", got, err, lookups)
	}
}

func TestLiveModernServerInfoRejectsMalformedPresentMetadata(t *testing.T) {
	for _, meta := range []string{`null`, `{"io.modelcontextprotocol/serverInfo":null}`, `{"io.modelcontextprotocol/serverInfo":{"name":null,"version":"1"}}`, `{"io.modelcontextprotocol/serverInfo":{"name":"fixture","version":null}}`, `{"io.modelcontextprotocol/ServerInfo":{"name":"fixture","version":"1"}}`} {
		conn := &inventoryConn{pages: []string{`{"supportedVersions":["2026-07-28"],"capabilities":{},"_meta":` + meta + `}`}}
		p := newLiveProbe(Server{}, "", DefaultLiveTimeout, CheckOptions{})
		if _, err := p.discover(context.Background(), conn, 0, func(error) bool { return false }); err == nil {
			t.Fatalf("malformed modern metadata accepted: %s", meta)
		}
	}
}

func TestLiveStdioIgnoresServerRequestsBeforeResponse(t *testing.T) {
	incoming := "{\"jsonrpc\":\"2.0\",\"id\":\"health-1\",\"method\":\"ping\"}\n{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n"
	var waitErr error
	conn := newStdioConn(&bufferWriteCloser{}, strings.NewReader(incoming), make(chan struct{}), &waitErr)
	defer close(conn.stop)
	result, err := conn.call(context.Background(), "tools/list", nil)
	if err != nil || !strings.Contains(string(result), "tools") {
		t.Fatalf("server request broke probe: %s %v", result, err)
	}
}

func TestLiveStdioSkipsPlainLogsBeforeResponse(t *testing.T) {
	for name, log := range map[string]string{
		"plain JSON":    `{"level":"info","msg":"started"}`,
		"indented JSON": `  {"level":"info","msg":"started"}`,
		"nested fields": `{"level":"info","record":{"id":99,"method":"started"}}`,
		"plain text":    `server started`,
		"error log":     `{"level":"error","error":"cache miss","message":"warming"}`,
		"id field":      `{"id":99,"level":"info"}`,
		"method field":  `{"method":"started","level":"info"}`,
		"result field":  `{"result":{},"level":"info"}`,
		"id alias":      `{"ID":99,"level":"info"}`,
		"method alias":  `{"METHOD":"started","level":"info"}`,
		"result alias":  `{"RESULT":{},"level":"info"}`,
		"error alias":   `{"ERROR":"started","level":"info"}`,
		"Unicode log":   `{"re\u017fult":{},"level":"info"}`,
	} {
		t.Run(name, func(t *testing.T) {
			incoming := log + "\n" + `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"
			var waitErr error
			conn := newStdioConn(&bufferWriteCloser{}, strings.NewReader(incoming), make(chan struct{}), &waitErr)
			defer close(conn.stop)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result, err := conn.call(ctx, "tools/list", nil)
			if err != nil || string(result) != `{"tools":[]}` {
				t.Fatalf("plain log blocked the valid response: %s %v", result, err)
			}
		})
	}
}

func TestLiveStdioRejectsMalformedOrProtocolShapedLogsBeforeResponse(t *testing.T) {
	for name, line := range map[string]string{
		"malformed JSON":         `{"level":"info"`,
		"malformed envelope":     `{"jsonrpc":"2.0","id":1,"result":`,
		"duplicate log key":      `{"level":"info","level":"error"}`,
		"duplicate reserved":     `{"jsonrpc":"2.0","id":99,"id":1,"result":{}}`,
		"invalid version":        `{"jsonrpc":"1.0","id":1,"result":{}}`,
		"trailing value":         `{"level":"info"} {}`,
		"jsonrpc field":          `{"jsonrpc":null,"level":"info"}`,
		"jsonrpc alias":          `{"JSONRPC":"2.0","level":"info"}`,
		"Unicode jsonrpc":        `{"j\u017fonrpc":"2.0","level":"info"}`,
		"missing version result": `{"id":1,"result":{}}`,
		"missing version error":  `{"id":1,"error":{"code":-32601,"message":"bad"}}`,
		"aliased response":       `{"ID":1,"RESULT":{}}`,
		"Unicode response":       `{"id":1,"re\u017fult":{}}`,
		"null response fields":   `{"id":null,"error":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			incoming := line + "\n" + `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}` + "\n"
			var waitErr error
			conn := newStdioConn(&bufferWriteCloser{}, strings.NewReader(incoming), make(chan struct{}), &waitErr)
			defer close(conn.stop)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result, err := conn.call(ctx, "tools/list", nil)
			var protocol *protocolError
			if !errors.As(err, &protocol) || len(result) != 0 {
				t.Fatalf("invalid line was skipped before the valid response: %s %v", result, err)
			}
		})
	}
}

func (c *inventoryConn) call(context.Context, string, any) (json.RawMessage, error) {
	i := min(c.calls, len(c.pages)-1)
	c.calls++
	return json.RawMessage(c.pages[i]), nil
}
func (*inventoryConn) notify(context.Context, string, any) error { return nil }
func (*inventoryConn) legacy(string)                             {}

func TestLiveInventoryRejectsAmbiguousOrPartialLists(t *testing.T) {
	tool := `{"name":"read","inputSchema":{"type":"object"}}`
	for name, pages := range map[string][]string{
		"missing list":         {`{}`},
		"null list":            {`{"tools":null}`},
		"missing schema":       {`{"tools":[{"name":"read"}]}`},
		"duplicate tool":       {`{"tools":[` + tool + `,` + tool + `]}`},
		"cross-page duplicate": {`{"tools":[` + tool + `],"nextCursor":"again"}`, `{"tools":[` + tool + `]}`},
		"cursor cycle":         {`{"tools":[],"nextCursor":"again"}`},
		"partial result":       {`{"resultType":"partial","tools":[]}`},
		"null result type":     {`{"resultType":null,"tools":[]}`},
		"empty result type":    {`{"resultType":"","tools":[]}`},
		"null cursor":          {`{"tools":[],"nextCursor":null}`},
		"duplicate JSON key":   {`{"tools":[],"tools":[` + tool + `]}`},
	} {
		t.Run(name, func(t *testing.T) {
			conn := &inventoryConn{pages: pages}
			names, tools, err := listTools(context.Background(), conn, func(string) map[string]any { return nil }, nil)
			if err == nil || names != nil || tools != nil {
				t.Fatalf("partial inventory returned: names=%v, tools=%v, err=%v", names, tools, err)
			}
		})
	}
}

func TestLiveInventoryRejectsPageLimitWithRemainingCursor(t *testing.T) {
	pages := make([]string, liveMaxPages)
	for i := range pages {
		pages[i] = fmt.Sprintf(`{"tools":[],"nextCursor":"page-%d"}`, i+1)
	}
	conn := &inventoryConn{pages: pages}
	if _, _, err := listTools(context.Background(), conn, func(string) map[string]any { return nil }, nil); err == nil {
		t.Fatal("unfinished inventory at page cap was accepted")
	}
	if conn.calls != liveMaxPages {
		t.Fatalf("calls = %d", conn.calls)
	}
}

func TestLiveInventoryRejectsCredentialBeforeReturningHashes(t *testing.T) {
	conn := &inventoryConn{pages: []string{`{"tools":[{"name":"read","description":"private q","inputSchema":{"type":"object"}}]}`}}
	names, tools, err := listTools(context.Background(), conn, func(string) map[string]any { return nil }, []string{"q"})
	if err == nil || names != nil || tools != nil || strings.Contains(err.Error(), "private q") {
		t.Fatalf("credential-bearing inventory escaped: names=%v tools=%v err=%v", names, tools, err)
	}
}

func TestLiveHTTPRejectsInvalidEnvelopes(t *testing.T) {
	for name, body := range map[string]string{
		"wrong id":              `{"jsonrpc":"2.0","id":99,"result":{}}`,
		"wrong version":         `{"jsonrpc":"1.0","id":1,"result":{}}`,
		"both result and error": `{"jsonrpc":"2.0","id":1,"result":{},"error":{"code":-32601,"message":"bad"}}`,
		"trailing value":        `{"jsonrpc":"2.0","id":1,"result":{}} {}`,
		"duplicate id":          `{"jsonrpc":"2.0","id":99,"id":1,"result":{}}`,
		"response method":       `{"jsonrpc":"2.0","id":1,"method":"tools/list","result":{}}`,
		"case alias version":    `{"jsonrpc":"1.0","JSONRPC":"2.0","id":1,"result":{}}`,
		"case alias id":         `{"jsonrpc":"2.0","ID":1,"result":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			conn := &httpConn{client: &http.Client{}, url: server.URL}
			if _, err := conn.call(context.Background(), "tools/list", nil); err == nil {
				t.Fatal("invalid response was accepted")
			}
		})
	}
}

func TestLiveMalformedDiscoveryCannotFallBackToTrust(t *testing.T) {
	initialized := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage
			Method string
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "server/discover" {
			w.WriteHeader(400)
			fmt.Fprint(w, "null")
			return
		}
		initialized++
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}}})
	}))
	defer server.Close()
	p := newLiveProbe(Server{URL: server.URL}, "", DefaultLiveTimeout, CheckOptions{})
	if _, err := p.http(context.Background()); err == nil || initialized != 0 {
		t.Fatalf("malformed discovery fell back: initialized=%d err=%v", initialized, err)
	}
}

func TestLiveSSERequiresEventDelimiter(t *testing.T) {
	body := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n"
	if _, err := readSSE(strings.NewReader(body), "1"); err == nil {
		t.Fatal("unterminated event accepted")
	}
	if _, err := readSSE(strings.NewReader(body+"\n"), "1"); err != nil {
		t.Fatalf("complete event rejected: %v", err)
	}
}

func TestLiveStrictToolsRequiresCompleteObservation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	s := checkService(t, "mcp:\n  targets: []\n  servers:\n    remote:\n      targets: []\n      url: "+server.URL+"\n")
	report, err := s.Check(CheckOptions{Live: true, StrictTools: true, SkipDNS: true})
	if err != nil || report.Summary.Errors == 0 || report.Servers[0].OK {
		t.Fatalf("strict unavailable observation: report=%+v err=%v", report, err)
	}
	if requests.Load() == 0 {
		t.Fatal("strict observation never reached the HTTP 401 endpoint")
	}
	strictFailure := false
	for _, finding := range report.Servers[0].Findings {
		if finding.Check == "tool-change" && finding.Level == "error" {
			strictFailure = true
		}
	}
	if !strictFailure {
		t.Fatal("enabled server's incomplete observation did not fail strict tool checking")
	}
}

func TestLiveStrictToolsSkipsDisabledProjectEntry(t *testing.T) {
	s, tmp := projectsService(t, "mcp:\n  servers:\n    remote:\n      url: http://127.0.0.1:1/mcp\n      targets: [opencode]\n  projects:\n    $TMP/project:\n      targets: [opencode]\n      servers:\n        remote:\n          disabled: true\n")
	root := filepath.Join(tmp, "project")
	report, err := s.Check(CheckOptions{Live: true, StrictTools: true, SkipDNS: true, Root: &root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Servers) != 1 || report.Summary.Errors != 0 || !report.Servers[0].OK {
		t.Fatalf("disabled entry failed strict checking: %+v", report)
	}
	result := report.Servers[0]
	if result.Project != root || result.Name != "remote" {
		t.Fatalf("check did not select the disabled project override: %+v", result)
	}
	if result.Live != nil || result.Contract != nil {
		t.Fatal("disabled entry acquired a live observation or review")
	}
	skipped := false
	for _, finding := range result.Findings {
		if finding.Check == "live" && finding.Level == "info" && strings.Contains(finding.Message, "disabled") {
			skipped = true
		}
		if finding.Check == "tool-change" {
			t.Fatal("disabled entry required a tool observation")
		}
	}
	if !skipped {
		t.Fatal("disabled entry was not explicitly skipped")
	}
}

func TestLiveStrictToolsRejectsEnabledPreflightSkip(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	s := checkService(t, "mcp:\n  targets: []\n  servers:\n    remote:\n      targets: []\n      url: "+server.URL+"\n      bearerToken: {fromEnv: FIXTURE_MISSING_TOKEN}\n")
	report, err := s.Check(CheckOptions{Live: true, StrictTools: true, SkipDNS: true, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("entry with a preflight error reached the live endpoint")
	}
	if report.Servers[0].Live != nil || report.Servers[0].Contract != nil {
		t.Fatal("preflight-skipped entry acquired a live observation or review")
	}
	skipped, strictFailure := false, false
	for _, finding := range report.Servers[0].Findings {
		if finding.Check == "live" && finding.Level == "info" && finding.Message == "not probed live: fix the errors above first" {
			skipped = true
		}
		if finding.Check == "tool-change" && finding.Level == "error" {
			strictFailure = true
		}
	}
	if !skipped || !strictFailure {
		t.Fatal("enabled entry did not report both preflight skip and strict tool failure")
	}
}

func TestLiveProbeDoesNotFollowRedirects(t *testing.T) {
	visits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { visits++; w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	p := newLiveProbe(Server{URL: origin.URL}, "", DefaultLiveTimeout, CheckOptions{})
	if _, err := p.http(context.Background()); err == nil || visits != 0 {
		t.Fatalf("redirect followed: visits=%d err=%v", visits, err)
	}
}

func TestLiveDiagnosticsRedactShortCredentialsAndControls(t *testing.T) {
	p := &liveProbe{secrets: []string{"q"}}
	got := p.redact("failure q\x1b]52;c;payload\a\r\u202e\U000e0001")
	if want := `failure [redacted]\u001b]52;c;payload\u0007\u000d\u202e\udb40\udc01`; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}
