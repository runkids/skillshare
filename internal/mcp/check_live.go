package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A live probe speaks MCP revision 2026-07-28 and falls back to the initialize handshake of
// 2025-11-25 for servers that predate server/discover, as the spec's backward-compatibility
// rules describe.
const (
	liveProtocolVersion   = "2026-07-28"
	legacyProtocolVersion = "2025-11-25"
	// DefaultLiveTimeout bounds each server's probe unless CheckOptions.Timeout says otherwise.
	DefaultLiveTimeout = 10 * time.Second
	liveConcurrency    = 4
	// liveGrace is how long a stopped server gets before the next, harder signal.
	liveGrace       = 500 * time.Millisecond
	liveMaxMessage  = 16 << 20
	liveMaxPages    = 50
	liveStderrLines = 5
)

// CheckLive is what a server said about itself when probed. Every field is self-reported.
type CheckLive struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ServerInfo      LiveServerInfo `json:"serverInfo"`
	Tools           int            `json:"tools"`
	// ToolNames are the names tools/list returned, for choosing a tool policy.
	ToolNames []string `json:"toolNames,omitempty"`
}

type LiveServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// checkLive probes, a few at a time, each selected server that passed the static checks.
func (s *Service) checkLive(source *Source, report *CheckReport, opts CheckOptions) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultLiveTimeout
	}
	slots := make(chan struct{}, liveConcurrency)
	var wg sync.WaitGroup
	for i := range report.Servers {
		result := &report.Servers[i]
		server := checkKey{result.Project, result.Name}.server(source)
		if reason := liveSkipReason(server, result.Findings); reason != "" {
			result.Findings = append(result.Findings, CheckFinding{Level: "info", Check: "live", Message: reason})
			continue
		}
		p := newLiveProbe(server, result.Project, timeout, opts)
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			live, finding := p.run()
			result.Live = live
			result.Findings = append(result.Findings, finding)
		}()
	}
	wg.Wait()
}

func liveSkipReason(server Server, findings []CheckFinding) string {
	if server.Disabled {
		return "not probed live: a disabled entry has no server to start"
	}
	for _, f := range findings {
		if f.Level == "error" {
			return "not probed live: fix the errors above first"
		}
	}
	return ""
}

type liveProbe struct {
	server  Server
	dir     string
	timeout time.Duration
	client  LiveServerInfo
	env     map[string]string
	headers http.Header
	// secrets are the resolved env, header and token values, longest first.
	secrets []string
}

func newLiveProbe(server Server, root string, timeout time.Duration, opts CheckOptions) *liveProbe {
	p := &liveProbe{server: server, timeout: timeout, env: map[string]string{}, headers: http.Header{}}
	if root != "" {
		p.dir, _ = expandHome(root)
	}
	p.client = LiveServerInfo{Name: "skillshare", Version: opts.ClientVersion}
	if p.client.Version == "" {
		p.client.Version = "dev"
	}
	resolve := func(v Value) string {
		if v.FromEnv == "" {
			return v.Literal
		}
		value, _ := opts.LookupEnv(v.FromEnv)
		return value
	}
	// A value shorter than four characters is no credential, and hiding every "1" or
	// "on" would garble the rest of the message.
	keep := func(value string) {
		if len(value) >= 4 && !slices.Contains(p.secrets, value) {
			p.secrets = append(p.secrets, value)
		}
	}
	for key, v := range server.Env {
		p.env[key] = resolve(v)
		keep(p.env[key])
	}
	for key, v := range server.Headers {
		value := resolve(v)
		p.headers.Set(key, value)
		keep(value)
	}
	if server.BearerToken != nil {
		token := resolve(*server.BearerToken)
		p.headers.Set("Authorization", "Bearer "+token)
		keep(token)
	}
	slices.SortFunc(p.secrets, func(a, b string) int { return len(b) - len(a) })
	return p
}

// redact removes every configured value from text a server or the network produced.
func (p *liveProbe) redact(text string) string {
	for _, secret := range p.secrets {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	return text
}

func (p *liveProbe) run() (*CheckLive, CheckFinding) {
	live, err := p.probe()
	if err != nil {
		var auth *authRequiredError
		if errors.As(err, &auth) {
			f := CheckFinding{Level: "warning", Check: "live", Message: "sign-in required (HTTP 401); skillshare does not sign in"}
			if auth.metadata != "" {
				f.Subject = p.redact(auth.metadata)
				f.Message = "sign-in required (HTTP 401): resource metadata at " + f.Subject + "; skillshare does not sign in"
			}
			return nil, f
		}
		return nil, CheckFinding{Level: "error", Check: "live", Message: p.redact("live probe failed: " + err.Error())}
	}
	who := strings.TrimSpace(live.ServerInfo.Name + " " + live.ServerInfo.Version)
	if who == "" {
		who = "unnamed server"
	}
	return live, CheckFinding{Level: "info", Check: "live", Subject: live.ServerInfo.Name,
		Message: fmt.Sprintf("responds: %s, protocol %s, %d tool(s)", who, live.ProtocolVersion, live.Tools)}
}

// probe returns what the server reported about itself, with configured values redacted.
func (p *liveProbe) probe() (*CheckLive, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	var live *CheckLive
	var err error
	if p.server.Command != "" {
		live, err = p.stdio(ctx)
	} else {
		live, err = p.http(ctx)
	}
	if err != nil {
		return nil, err
	}
	live.ProtocolVersion = p.redact(live.ProtocolVersion)
	live.ServerInfo.Name = p.redact(live.ServerInfo.Name)
	live.ServerInfo.Version = p.redact(live.ServerInfo.Version)
	for i, name := range live.ToolNames {
		live.ToolNames[i] = p.redact(name)
	}
	return live, nil
}

// DraftProbe is a live probe of a server definition that is not saved.
type DraftProbe struct {
	Live *CheckLive `json:"live,omitempty"`
	// ErrorKind classifies a failure for a client to phrase: connect, timeout, auth,
	// protocol, command, or unknown. Error is its detail, with configured values redacted.
	ErrorKind string `json:"errorKind,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProbeDraft probes server as written rather than as saved, the way a live check probes a
// saved one; root is the mcp.projects root it belongs to. It saves and syncs nothing, and
// fails only for a definition that is not a valid server.
func ProbeDraft(server Server, root string, opts CheckOptions) (DraftProbe, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	// Only the connection matters; a half-written tool policy must not stop a probe.
	server = Server{Transport: server.Transport, Command: server.Command, Args: server.Args, URL: server.URL,
		Env: server.Env, Headers: server.Headers, BearerToken: server.BearerToken}
	if err := server.Validate("draft"); err != nil {
		return DraftProbe{}, err
	}
	if missing := checkEnv(server, opts.LookupEnv); len(missing) > 0 {
		return DraftProbe{ErrorKind: "auth", Error: missing[0].Message}, nil
	}
	for _, f := range checkLaunch(server, CheckOptions{SkipDNS: true, LookPath: opts.LookPath}) {
		if f.Level == "error" {
			return DraftProbe{ErrorKind: "command", Error: f.Message}, nil
		}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultLiveTimeout
	}
	p := newLiveProbe(server, root, timeout, opts)
	live, err := p.probe()
	if err != nil {
		return DraftProbe{ErrorKind: liveErrorKind(err), Error: p.redact(err.Error())}, nil
	}
	return DraftProbe{Live: live}, nil
}

// liveErrorKind classifies why a probe failed.
func liveErrorKind(err error) string {
	var auth *authRequiredError
	var status *httpStatusError
	var timeout *liveTimeoutError
	var dns *net.DNSError
	var op *net.OpError
	var rpc *rpcError
	var bad *protocolError
	switch {
	case errors.As(err, &auth), errors.As(err, &status) && (status.code == http.StatusUnauthorized || status.code == http.StatusForbidden):
		return "auth"
	case errors.As(err, &timeout):
		return "timeout"
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return "command"
	case errors.As(err, &dns), errors.As(err, &op):
		return "connect"
	case errors.As(err, &status), errors.As(err, &rpc), errors.As(err, &bad):
		return "protocol"
	}
	return "unknown"
}

// protocolError is a reply that is not the MCP a probe expects.
type protocolError struct{ message string }

func (e *protocolError) Error() string { return e.message }

func badReply(format string, args ...any) error {
	return &protocolError{fmt.Sprintf(format, args...)}
}

type liveTimeoutError struct{ timeout time.Duration }

func (e *liveTimeoutError) Error() string { return fmt.Sprintf("no answer within %s", e.timeout) }

// rpcConn sends JSON-RPC over one transport. legacy switches it to a 2025-11-25 session.
type rpcConn interface {
	call(ctx context.Context, method string, params any) (json.RawMessage, error)
	notify(ctx context.Context, method string, params any) error
	legacy(protocolVersion string)
}

type rpcMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message) }

// modern reports a code from the range the MCP specification reserves, which only a
// 2026-07-28 or later server sends; any other error to server/discover means a legacy server.
func (e *rpcError) modern() bool { return e.Code <= -32020 && e.Code >= -32099 }

func rpcRequest(id int, method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func (p *liveProbe) modernParams(cursor string) map[string]any {
	params := map[string]any{"_meta": map[string]any{
		"io.modelcontextprotocol/protocolVersion":    liveProtocolVersion,
		"io.modelcontextprotocol/clientInfo":         p.client,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}}
	if cursor != "" {
		params["cursor"] = cursor
	}
	return params
}

// callResult calls method and decodes a complete result into out.
func callResult(ctx context.Context, conn rpcConn, method string, params, out any) error {
	raw, err := conn.call(ctx, method, params)
	if err != nil {
		return err
	}
	var kind struct {
		ResultType string `json:"resultType"`
	}
	if err := json.Unmarshal(raw, &kind); err != nil {
		return badReply("%s returned an invalid result", method)
	}
	// Servers before 2026-07-28 leave resultType out; that means complete.
	if kind.ResultType != "" && kind.ResultType != "complete" {
		return badReply("%s returned resultType %q", method, kind.ResultType)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return badReply("%s returned an invalid result", method)
	}
	return nil
}

// handshake asks the server who it is and counts its tools. discoverWait, when set, is how
// long server/discover may go unanswered before the server counts as legacy (stdio).
func (p *liveProbe) handshake(ctx context.Context, conn rpcConn, discoverWait time.Duration, fallback func(error) bool) (*CheckLive, error) {
	live, err := p.discover(ctx, conn, discoverWait, fallback)
	if err == nil && live == nil {
		live, err = p.initialize(ctx, conn)
	}
	if err != nil && ctx.Err() != nil {
		return nil, &liveTimeoutError{p.timeout}
	}
	return live, err
}

// discover returns nil, nil when the server predates server/discover.
func (p *liveProbe) discover(ctx context.Context, conn rpcConn, wait time.Duration, fallback func(error) bool) (*CheckLive, error) {
	dctx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	var result struct {
		SupportedVersions []string                   `json:"supportedVersions"`
		Capabilities      map[string]json.RawMessage `json:"capabilities"`
		Meta              struct {
			ServerInfo LiveServerInfo `json:"io.modelcontextprotocol/serverInfo"`
		} `json:"_meta"`
	}
	err := callResult(dctx, conn, "server/discover", p.modernParams(""), &result)
	var rerr *rpcError
	switch {
	case err == nil:
	case errors.As(err, &rerr) && rerr.modern():
		var data struct {
			Supported []string `json:"supported"`
		}
		if rerr.Code == -32022 && json.Unmarshal(rerr.Data, &data) == nil && len(data.Supported) > 0 {
			return nil, unsupportedVersions(data.Supported)
		}
		return nil, err
	case errors.As(err, &rerr), fallback(err):
		return nil, nil
	case wait > 0 && dctx.Err() != nil && ctx.Err() == nil:
		return nil, nil
	default:
		return nil, err
	}
	if !slices.Contains(result.SupportedVersions, liveProtocolVersion) {
		return nil, unsupportedVersions(result.SupportedVersions)
	}
	live := &CheckLive{ProtocolVersion: liveProtocolVersion, ServerInfo: result.Meta.ServerInfo}
	if _, ok := result.Capabilities["tools"]; ok {
		live.ToolNames, err = listTools(ctx, conn, p.modernParams)
		live.Tools = len(live.ToolNames)
	}
	return live, err
}

func unsupportedVersions(versions []string) error {
	return badReply("server supports protocol version(s) %s; skillshare probes %s or the initialize handshake", strings.Join(versions, ", "), liveProtocolVersion)
}

func (p *liveProbe) initialize(ctx context.Context, conn rpcConn) (*CheckLive, error) {
	conn.legacy("")
	var result struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		ServerInfo      LiveServerInfo             `json:"serverInfo"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
	}
	params := map[string]any{"protocolVersion": legacyProtocolVersion, "capabilities": map[string]any{}, "clientInfo": p.client}
	if err := callResult(ctx, conn, "initialize", params, &result); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	conn.legacy(result.ProtocolVersion)
	if err := conn.notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return nil, fmt.Errorf("notifications/initialized: %w", err)
	}
	live := &CheckLive{ProtocolVersion: result.ProtocolVersion, ServerInfo: result.ServerInfo}
	if _, ok := result.Capabilities["tools"]; ok {
		var err error
		live.ToolNames, err = listTools(ctx, conn, func(cursor string) map[string]any {
			if cursor == "" {
				return map[string]any{}
			}
			return map[string]any{"cursor": cursor}
		})
		if err != nil {
			return nil, err
		}
		live.Tools = len(live.ToolNames)
	}
	return live, nil
}

func listTools(ctx context.Context, conn rpcConn, params func(cursor string) map[string]any) ([]string, error) {
	names, cursor := []string{}, ""
	for range liveMaxPages {
		var page struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := callResult(ctx, conn, "tools/list", params(cursor), &page); err != nil {
			return nil, fmt.Errorf("tools/list: %w", err)
		}
		for _, tool := range page.Tools {
			names = append(names, tool.Name)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return names, nil
}

// stdio starts the server, probes it over stdin/stdout, and always stops it again.
func (p *liveProbe) stdio(ctx context.Context) (*CheckLive, error) {
	command, err := expandHome(p.server.Command)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(command, p.server.Args...)
	cmd.Dir = p.dir
	cmd.Env = os.Environ()
	for _, key := range sortedKeys(p.env) {
		cmd.Env = append(cmd.Env, key+"="+p.env[key])
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// Plain pipes, so waiting for the server never waits for a descendant holding its output.
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer outR.Close()
	errR, errW, err := os.Pipe()
	if err != nil {
		outW.Close()
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = outW, errW
	startInGroup(cmd)
	err = cmd.Start()
	outW.Close()
	errW.Close()
	if err != nil {
		errR.Close()
		return nil, fmt.Errorf("could not start %s: %w", p.server.Command, err)
	}
	stderr := &tailBuffer{}
	stderrDone := make(chan struct{})
	go func() {
		io.Copy(stderr, errR)
		errR.Close()
		close(stderrDone)
	}()
	exited := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()
	conn := newStdioConn(stdin, outR, exited, &waitErr)
	live, err := p.handshake(ctx, conn, p.timeout/3, func(error) bool { return false })
	stopGroup(cmd, stdin, exited)
	close(conn.stop)
	if err != nil {
		select {
		case <-stderrDone:
		case <-time.After(liveGrace):
		}
		if lines := stderr.lastLines(liveStderrLines); lines != "" {
			err = fmt.Errorf("%w; stderr: %s", err, lines)
		}
	}
	return live, err
}

// stopGroup closes stdin, then terminates, then kills the server and everything it started.
func stopGroup(cmd *exec.Cmd, stdin io.Closer, exited <-chan struct{}) {
	stdin.Close()
	select {
	case <-exited:
	case <-time.After(liveGrace):
		terminateGroup(cmd)
		select {
		case <-exited:
		case <-time.After(liveGrace):
			killGroup(cmd)
			<-exited
		}
	}
	// Descendants may outlive the server itself.
	killGroup(cmd)
}

type stdioConn struct {
	stdin   io.Writer
	msgs    chan rpcMessage
	stop    chan struct{}
	exited  <-chan struct{}
	waitErr *error
	nextID  int
}

func newStdioConn(stdin io.Writer, stdout io.Reader, exited <-chan struct{}, waitErr *error) *stdioConn {
	c := &stdioConn{stdin: stdin, msgs: make(chan rpcMessage), stop: make(chan struct{}), exited: exited, waitErr: waitErr}
	go func() {
		defer close(c.msgs)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), liveMaxMessage)
		for scanner.Scan() {
			var msg rpcMessage
			// Only responses matter; lines that are not JSON-RPC are skipped.
			if json.Unmarshal(scanner.Bytes(), &msg) != nil || len(msg.ID) == 0 || msg.Method != "" {
				continue
			}
			select {
			case c.msgs <- msg:
			case <-c.stop:
				return
			}
		}
	}()
	return c
}

func (c *stdioConn) write(message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return c.exitError()
	}
	return nil
}

func (c *stdioConn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.nextID++
	id := strconv.Itoa(c.nextID)
	if err := c.write(rpcRequest(c.nextID, method, params)); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg, ok := <-c.msgs:
			if !ok {
				return nil, c.exitError()
			}
			if string(msg.ID) != id {
				continue
			}
			if msg.Error != nil {
				return nil, msg.Error
			}
			return msg.Result, nil
		}
	}
}

func (c *stdioConn) notify(_ context.Context, method string, params any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *stdioConn) legacy(string) {}

// exitError describes a server that stopped answering, with its exit status once known.
func (c *stdioConn) exitError() error {
	select {
	case <-c.exited:
		if *c.waitErr != nil {
			return fmt.Errorf("server exited early (%v)", *c.waitErr)
		}
		return errors.New("server exited early")
	case <-time.After(liveGrace):
		return errors.New("server closed its output")
	}
}

// tailBuffer keeps the end of a server's stderr.
type tailBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if extra := len(b.data) - 64<<10; extra > 0 {
		b.data = b.data[extra:]
	}
	return len(p), nil
}

func (b *tailBuffer) lastLines(n int) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var lines []string
	for _, line := range strings.Split(string(b.data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			if len(line) > 200 {
				line = line[:200] + "…"
			}
			lines = append(lines, line)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// http probes a Streamable HTTP endpoint. It never follows an authorization challenge.
func (p *liveProbe) http(ctx context.Context) (*CheckLive, error) {
	conn := &httpConn{client: &http.Client{}, url: p.server.URL, headers: p.headers, protocol: liveProtocolVersion}
	defer conn.close(ctx)
	return p.handshake(ctx, conn, 0, func(err error) bool {
		var status *httpStatusError
		return errors.As(err, &status) && status.fallsBack()
	})
}

type authRequiredError struct{ metadata string }

func (e *authRequiredError) Error() string { return "sign-in required" }

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d %s", e.code, http.StatusText(e.code))
}

// fallsBack reports the statuses a legacy server answers an unknown modern request with.
func (e *httpStatusError) fallsBack() bool {
	return e.code == http.StatusBadRequest || e.code == http.StatusNotFound || e.code == http.StatusMethodNotAllowed
}

type httpConn struct {
	client   *http.Client
	url      string
	headers  http.Header
	protocol string
	session  string
	nextID   int
}

func (c *httpConn) legacy(protocolVersion string) { c.protocol = protocolVersion }

func (c *httpConn) post(ctx context.Context, method string, message any) (*http.Response, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, values := range c.headers {
		req.Header[key] = values
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Method", method)
	if c.protocol != "" {
		req.Header.Set("MCP-Protocol-Version", c.protocol)
	}
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if session := resp.Header.Get("Mcp-Session-Id"); session != "" {
		c.session = session
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, &authRequiredError{metadata: resourceMetadata(resp.Header.Values("WWW-Authenticate"))}
	}
	return resp, nil
}

func (c *httpConn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.nextID++
	resp, err := c.post(ctx, method, rpcRequest(c.nextID, method, params))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := io.LimitReader(resp.Body, liveMaxMessage)
	status := &httpStatusError{code: resp.StatusCode}
	var msg *rpcMessage
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			msg, err = readSSE(body, strconv.Itoa(c.nextID))
		} else {
			msg = &rpcMessage{}
			err = json.NewDecoder(body).Decode(msg)
		}
		if err != nil || (msg.Error == nil && msg.Result == nil) {
			return nil, badReply("%s: the response is not a JSON-RPC response", method)
		}
	case status.fallsBack():
		// A modern server explains a rejection in a JSON-RPC error body.
		msg = &rpcMessage{}
		if json.NewDecoder(body).Decode(msg) != nil || msg.Error == nil {
			return nil, status
		}
	default:
		return nil, status
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	return msg.Result, nil
}

func (c *httpConn) notify(ctx context.Context, method string, params any) error {
	resp, err := c.post(ctx, method, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpStatusError{code: resp.StatusCode}
	}
	return nil
}

// close ends a 2025-11-25 session, which a stateless server never opens.
func (c *httpConn) close(ctx context.Context) {
	if c.session == "" || ctx.Err() != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.url, nil)
	if err != nil {
		return
	}
	for key, values := range c.headers {
		req.Header[key] = values
	}
	req.Header.Set("Mcp-Session-Id", c.session)
	if resp, err := c.client.Do(req); err == nil {
		resp.Body.Close()
	}
}

// readSSE returns the response with the given id from a request's event stream.
func readSSE(r io.Reader, id string) (*rpcMessage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), liveMaxMessage)
	var data []string
	dispatch := func() *rpcMessage {
		defer func() { data = nil }()
		var msg rpcMessage
		if len(data) == 0 || json.Unmarshal([]byte(strings.Join(data, "\n")), &msg) != nil {
			return nil
		}
		if string(msg.ID) == id && msg.Method == "" {
			return &msg
		}
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case line == "":
			if msg := dispatch(); msg != nil {
				return msg, nil
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if msg := dispatch(); msg != nil {
		return msg, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, badReply("the event stream ended without a response")
}

var resourceMetadataParam = regexp.MustCompile(`resource_metadata="([^"]*)"|resource_metadata=([^,\s]+)`)

// resourceMetadata is the Protected Resource Metadata URL a 401 challenge points to.
func resourceMetadata(challenges []string) string {
	for _, challenge := range challenges {
		if m := resourceMetadataParam.FindStringSubmatch(challenge); m != nil {
			return m[1] + m[2]
		}
	}
	return ""
}
