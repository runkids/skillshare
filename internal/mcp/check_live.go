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
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"
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
	baseline := s.loadToolBaseline(source, len(report.Servers), opts)
	var wg sync.WaitGroup
	for i := range report.Servers {
		result := &report.Servers[i]
		server := checkKey{result.Project, result.Name}.server(source)
		if reason := liveSkipReason(server, result.Findings); reason != "" {
			result.Findings = append(result.Findings, CheckFinding{Level: "info", Check: "live", Message: reason})
			if opts.StrictTools && !server.Disabled {
				result.Findings = append(result.Findings, CheckFinding{Level: "error", Check: "tool-change", Message: "strict tool checking requires a complete live observation"})
			}
			continue
		}
		root := result.Project
		if root == "" {
			root = s.configProjectRoot()
		}
		p := newLiveProbe(server, root, timeout, opts)
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			live, finding := p.run()
			result.Live = live
			result.Findings = append(result.Findings, finding)
			if live == nil && opts.StrictTools {
				result.Findings = append(result.Findings, CheckFinding{Level: "error", Check: "tool-change", Message: "strict tool checking requires a complete live observation"})
			}
			if live != nil {
				review, findings := baseline.compare(i, result.Project, result.Name, server, contractObservation{Tools: p.toolContracts, Command: p.resolvedCommand, Dir: p.dir, Live: p.observedLive, Secrets: p.secrets})
				result.Contract = review
				result.Findings = append(result.Findings, findings...)
			}
		}()
	}
	wg.Wait()
	baseline.save(s, report)
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
	// Contracts exist only after a complete, unambiguous inventory.
	toolContracts   map[string]ToolContract
	resolvedCommand string
	observedLive    *CheckLive
}

func newLiveProbe(server Server, root string, timeout time.Duration, opts CheckOptions) *liveProbe {
	p := &liveProbe{server: server, timeout: timeout, env: map[string]string{}, headers: http.Header{}, toolContracts: map[string]ToolContract{}}
	p.dir, _ = os.Getwd()
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
	// Every nonempty environment-backed credential is confidential, including short values.
	keep := func(value string) {
		if value != "" && !slices.Contains(p.secrets, value) {
			p.secrets = append(p.secrets, value)
		}
	}
	for key, v := range server.Env {
		p.env[key] = resolve(v)
		if v.FromEnv != "" || sensitiveKey.MatchString(key) {
			keep(p.env[key])
		}
	}
	for key, v := range server.Headers {
		value := resolve(v)
		p.headers.Set(key, value)
		if v.FromEnv != "" || sensitiveKey.MatchString(key) {
			keep(value)
		}
	}
	if server.BearerToken != nil {
		token := resolve(*server.BearerToken)
		p.headers.Set("Authorization", "Bearer "+token)
		keep(token)
	}
	for _, value := range opts.knownSecrets {
		keep(value)
	}
	slices.SortFunc(p.secrets, func(a, b string) int { return len(b) - len(a) })
	return p
}

// redact removes every configured value from text a server or the network produced.
func (p *liveProbe) redact(text string) string {
	for _, secret := range p.secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return safeLiveText(text)
}

// safeLiveText makes terminal and bidi controls visible without changing stored contracts.
func safeLiveText(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			if r <= 0xffff {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				a, z := utf16.EncodeRune(r)
				fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
			}
		} else {
			b.WriteRune(r)
		}
		if b.Len() > 8192 {
			b.WriteString(" [diagnostic truncated]")
			break
		}
	}
	return b.String()
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
	identity := p.server
	identity.Env, identity.Headers, identity.BearerToken = nil, nil, nil
	material, _ := json.Marshal(identity)
	var declarations any
	if decodeLiveJSON(material, &declarations) != nil || containsLiveSecret(declarations, p.secrets) {
		return nil, badReply("connection identity contains a configured credential; use environment-backed references")
	}
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
	// Identity material cannot contain known credential bytes either. Avoid hashing
	// environment/header declarations here; baseline approval validates their references.
	material, _ = json.Marshal(struct {
		Server   Server
		Info     LiveServerInfo
		Protocol string
	}{identity, live.ServerInfo, live.ProtocolVersion})
	var decoded any
	if err := decodeLiveJSON(material, &decoded); err != nil {
		return nil, badReply("invalid connection identity")
	}
	if containsLiveSecret(decoded, p.secrets) {
		return nil, badReply("configured credential appeared in connection identity; use environment references and remove credential-bearing metadata")
	}
	copyLive := *live
	p.observedLive = &copyLive
	live.ProtocolVersion = p.redact(live.ProtocolVersion)
	live.ServerInfo.Name = p.redact(live.ServerInfo.Name)
	live.ServerInfo.Version = p.redact(live.ServerInfo.Version)
	return live, nil
}

// sourceLiveSecrets protects diagnostics and every selected probe from all
// credential references declared by this source, including other server scopes.
func sourceLiveSecrets(source *Source, opts CheckOptions) []string {
	seen := map[string]bool{}
	var secrets []string
	add := func(servers map[string]Server) {
		for _, server := range servers {
			p := newLiveProbe(server, "", DefaultLiveTimeout, opts)
			for _, secret := range p.secrets {
				if secret != "" && !seen[secret] {
					seen[secret] = true
					secrets = append(secrets, secret)
				}
			}
		}
	}
	add(source.Servers)
	for _, project := range source.Projects {
		add(project.Servers)
	}
	slices.SortFunc(secrets, func(a, b string) int { return len(b) - len(a) })
	return secrets
}

func serverHasKnownSecret(server Server, root, name string, secrets []string) bool {
	server.Env, server.Headers, server.BearerToken = nil, nil, nil
	raw, _ := json.Marshal(struct {
		Server     Server
		Root, Name string
	}{server, root, name})
	var value any
	return decodeLiveJSON(raw, &value) != nil || containsLiveSecret(value, secrets)
}

func containsLiveSecret(value any, secrets []string) bool {
	contains := func(s string) bool {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(s, secret) {
				return true
			}
		}
		return false
	}
	switch v := value.(type) {
	case string:
		return contains(v)
	case json.Number:
		return contains(v.String())
	case []any:
		for _, item := range v {
			if containsLiveSecret(item, secrets) {
				return true
			}
		}
	case map[string]any:
		for key, item := range v {
			if contains(key) || containsLiveSecret(item, secrets) {
				return true
			}
		}
	}
	return false
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
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultLiveTimeout
	}
	p := newLiveProbe(server, root, timeout, opts)
	if err := server.Validate("draft"); err != nil {
		return DraftProbe{}, errors.New(p.redact(err.Error()))
	}
	if serverHasKnownSecret(server, root, "", p.secrets) {
		message := "connection identity contains a configured credential; use environment-backed references"
		if contractHasSecret(root, p.secrets) {
			message = "project scope contains a configured credential; no review is available"
		}
		return DraftProbe{ErrorKind: "protocol", Error: message}, nil
	}
	if missing := checkEnv(server, opts.LookupEnv); len(missing) > 0 {
		return DraftProbe{ErrorKind: "auth", Error: p.redact(missing[0].Message)}, nil
	}
	for _, f := range checkLaunch(server, CheckOptions{SkipDNS: true, LookPath: opts.LookPath}) {
		if f.Level == "error" {
			return DraftProbe{ErrorKind: "command", Error: p.redact(f.Message)}, nil
		}
	}
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
	JSONRPC string `json:"jsonrpc"`
	invalid error
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("JSON-RPC error %d", e.Code) }

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
	var fields map[string]json.RawMessage
	if decodeJSONBounded(raw, &fields, contractJSONDepth+8) != nil || fields == nil || !exactLiveFields(fields, "resultType", "tools", "nextCursor", "supportedVersions", "capabilities", "_meta", "protocolVersion", "serverInfo") {
		return badReply("%s returned ambiguous or invalid fields", method)
	}
	// Servers before 2026-07-28 leave resultType out; that means complete.
	if kind, exists := fields["resultType"]; exists {
		var resultType string
		if json.Unmarshal(kind, &resultType) != nil || resultType != "complete" {
			return badReply("%s returned a non-complete result type", method)
		}
	}
	if cursor, exists := fields["nextCursor"]; exists {
		var value string
		if string(cursor) == "null" || json.Unmarshal(cursor, &value) != nil {
			return badReply("%s returned an invalid pagination cursor", method)
		}
	}
	if method == "initialize" {
		if !validLiveServerInfo(fields["serverInfo"]) {
			return badReply("initialize returned invalid server information")
		}
	}
	if method == "server/discover" {
		if raw, exists := fields["_meta"]; exists {
			var meta map[string]json.RawMessage
			const infoKey = "io.modelcontextprotocol/serverInfo"
			if decodeLiveJSON(raw, &meta) != nil || meta == nil || !exactLiveFields(meta, infoKey) {
				return badReply("server/discover returned invalid server information")
			}
			if info, present := meta[infoKey]; present && !validLiveServerInfo(info) {
				return badReply("server/discover returned invalid server information")
			}
		}
	}
	if caps, exists := fields["capabilities"]; exists {
		var capabilities map[string]json.RawMessage
		if json.Unmarshal(caps, &capabilities) != nil || capabilities == nil {
			return badReply("%s returned invalid capabilities", method)
		}
		if tools, exists := capabilities["tools"]; exists {
			var capability map[string]json.RawMessage
			if json.Unmarshal(tools, &capability) != nil || capability == nil {
				return badReply("%s returned an invalid tools capability", method)
			}
		}
	}
	if err := decodeJSONBounded(raw, out, contractJSONDepth+8); err != nil {
		return badReply("%s returned an invalid result", method)
	}
	return nil
}

func validLiveServerInfo(raw json.RawMessage) bool {
	var info map[string]json.RawMessage
	if decodeLiveJSON(raw, &info) != nil || info == nil || !exactLiveFields(info, "name", "version") {
		return false
	}
	for _, key := range []string{"name", "version"} {
		var value string
		raw, ok := info[key]
		if !ok || string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
			return false
		}
	}
	return true
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
	case errors.As(err, &rerr) && rerr.Code == -32601, fallback(err):
		return nil, nil
	case wait > 0 && dctx.Err() != nil && ctx.Err() == nil:
		return nil, nil
	default:
		return nil, err
	}
	if !slices.Contains(result.SupportedVersions, liveProtocolVersion) {
		return nil, unsupportedVersions(result.SupportedVersions)
	}
	if result.Capabilities == nil {
		return nil, badReply("server/discover omitted capabilities")
	}
	live := &CheckLive{ProtocolVersion: liveProtocolVersion, ServerInfo: result.Meta.ServerInfo}
	if _, ok := result.Capabilities["tools"]; ok {
		live.ToolNames, p.toolContracts, err = listTools(ctx, conn, p.modernParams, p.secrets)
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
	if result.ProtocolVersion == "" || result.Capabilities == nil {
		return nil, badReply("initialize returned an incomplete result")
	}
	conn.legacy(result.ProtocolVersion)
	if err := conn.notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return nil, fmt.Errorf("notifications/initialized: %w", err)
	}
	live := &CheckLive{ProtocolVersion: result.ProtocolVersion, ServerInfo: result.ServerInfo}
	if _, ok := result.Capabilities["tools"]; ok {
		var err error
		live.ToolNames, p.toolContracts, err = listTools(ctx, conn, func(cursor string) map[string]any {
			if cursor == "" {
				return map[string]any{}
			}
			return map[string]any{"cursor": cursor}
		}, p.secrets)
		if err != nil {
			return nil, err
		}
		live.Tools = len(live.ToolNames)
	}
	return live, nil
}

const (
	liveMaxTools     = 4096
	liveMaxInventory = 4 << 20
)

// listTools succeeds only after the entire bounded inventory has been received.
func listTools(ctx context.Context, conn rpcConn, params func(cursor string) map[string]any, secrets []string) ([]string, map[string]ToolContract, error) {
	names, tools, cursor := []string{}, map[string]ToolContract{}, ""
	seen := map[string]bool{}
	total, canonicalTotal := 0, 0
	for range liveMaxPages {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var page struct {
			Tools      []json.RawMessage `json:"tools"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := callResult(ctx, conn, "tools/list", params(cursor), &page); err != nil {
			return nil, nil, fmt.Errorf("tools/list: %w", err)
		}
		if page.Tools == nil {
			return nil, nil, badReply("tools/list must return an array of tools")
		}
		for _, raw := range page.Tools {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			total += len(raw)
			if total > liveMaxInventory || len(tools) >= liveMaxTools {
				return nil, nil, badReply("tool inventory exceeds its byte or tool limit")
			}
			name, contract, err := captureTool(raw, secrets)
			if err != nil {
				return nil, nil, badReply("tool inventory rejected: %v", err)
			}
			canonicalTotal += len(contract.Definition)
			if canonicalTotal > liveMaxInventory {
				return nil, nil, badReply("canonical tool inventory exceeds its byte limit")
			}
			if _, exists := tools[name]; exists {
				return nil, nil, badReply("tool inventory contains duplicate names")
			}
			names = append(names, name)
			tools[name] = contract
		}
		if page.NextCursor == "" {
			return names, tools, nil
		}
		if seen[page.NextCursor] {
			return nil, nil, badReply("tool inventory repeated a pagination cursor")
		}
		seen[page.NextCursor] = true
		cursor = page.NextCursor
	}
	return nil, nil, badReply("tool inventory is incomplete at the page limit")
}

// stdio starts the server, probes it over stdin/stdout, and always stops it again.
func (p *liveProbe) stdio(ctx context.Context) (*CheckLive, error) {
	command, err := expandHome(p.server.Command)
	if err != nil {
		return nil, err
	}
	if p.dir == "" {
		return nil, badReply("cannot determine the server working directory")
	}
	if !filepath.IsAbs(command) {
		if strings.ContainsAny(command, "/\\") {
			command = filepath.Join(p.dir, command)
		} else {
			command, err = exec.LookPath(command)
			if err != nil {
				return nil, err
			}
			command, err = filepath.Abs(command)
			if err != nil {
				return nil, err
			}
		}
	}
	if containsLiveSecret([]any{command, p.dir}, p.secrets) {
		return nil, badReply("connection identity contains a configured credential; use environment-backed references")
	}
	p.resolvedCommand = command
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
			// A tail can split an encoded or long credential. Do not expose the
			// server's arbitrary diagnostic payload as part of approval output.
			err = fmt.Errorf("%w; server diagnostic output omitted", err)
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
	stdin   io.WriteCloser
	msgs    chan rpcMessage
	stop    chan struct{}
	exited  <-chan struct{}
	waitErr *error
	nextID  int
}

func newStdioConn(stdin io.WriteCloser, stdout io.Reader, exited <-chan struct{}, waitErr *error) *stdioConn {
	c := &stdioConn{stdin: stdin, msgs: make(chan rpcMessage), stop: make(chan struct{}), exited: exited, waitErr: waitErr}
	go func() {
		defer close(c.msgs)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), liveMaxMessage)
		for scanner.Scan() {
			raw := scanner.Bytes()
			// Plain logging lines are allowed, but JSON-RPC-looking envelopes fail closed.
			if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
				continue
			}
			msg, err := decodeRPCMessage(raw)
			if err != nil {
				var fields map[string]json.RawMessage
				if decodeJSONBounded(raw, &fields, contractJSONDepth+8) == nil {
					var envelope, id, outcome bool
					for key := range fields {
						switch {
						case strings.EqualFold(key, "jsonrpc"):
							envelope = true
						case strings.EqualFold(key, "id"):
							id = true
						case strings.EqualFold(key, "result"), strings.EqualFold(key, "error"):
							outcome = true
						}
					}
					if !envelope && !(id && outcome) {
						continue
					}
				}
				msg = &rpcMessage{invalid: err}
			}
			if err == nil && msg.Method != "" {
				continue
			}
			select {
			case c.msgs <- *msg:
			case <-c.stop:
				return
			}
		}
	}()
	return c
}

func (c *stdioConn) write(ctx context.Context, message any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	// Closing the process pipe interrupts a blocked write. Wait for the watcher
	// before returning so it cannot close stdin during a later request. A timed
	// out blocked write makes this transport unusable; stdio then stops the group.
	finished, watched := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watched)
		select {
		case <-ctx.Done():
			_ = c.stdin.Close()
		case <-finished:
		}
	}()
	n, err := c.stdin.Write(append(data, '\n'))
	close(finished)
	<-watched
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || n != len(data)+1 {
		return c.exitError()
	}
	return nil
}

func (c *stdioConn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.nextID++
	id := strconv.Itoa(c.nextID)
	if err := c.write(ctx, rpcRequest(c.nextID, method, params)); err != nil {
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
			if msg.invalid != nil {
				return nil, msg.invalid
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

func (c *stdioConn) notify(ctx context.Context, method string, params any) error {
	return c.write(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
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
	conn := &httpConn{client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, url: p.server.URL, headers: p.headers, protocol: liveProtocolVersion}
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
	body := &io.LimitedReader{R: resp.Body, N: liveMaxMessage + 1}
	status := &httpStatusError{code: resp.StatusCode}
	var msg *rpcMessage
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			msg, err = readSSE(body, strconv.Itoa(c.nextID))
		} else {
			var raw []byte
			raw, err = io.ReadAll(body)
			if err == nil {
				msg, err = decodeRPCMessage(raw)
			}
		}
		if body.N <= 0 || err != nil || msg == nil || string(msg.ID) != strconv.Itoa(c.nextID) || msg.Method != "" {
			return nil, badReply("%s: the response is not a JSON-RPC response", method)
		}
	case status.fallsBack():
		// A modern server explains a rejection in a JSON-RPC error body.
		raw, readErr := io.ReadAll(body)
		if body.N <= 0 {
			return nil, badReply("MCP response exceeds its byte limit")
		}
		msg, err = decodeRPCMessage(raw)
		if readErr != nil || err != nil {
			if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") || bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) || bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
				return nil, badReply("invalid discovery response")
			}
			return nil, status
		}
		if msg.Error == nil || string(msg.ID) != strconv.Itoa(c.nextID) {
			return nil, badReply("invalid discovery response")
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
	var invalid error
	dispatch := func() *rpcMessage {
		defer func() { data = nil }()
		if len(data) == 0 {
			return nil
		}
		msg, err := decodeRPCMessage([]byte(strings.Join(data, "\n")))
		if err != nil {
			invalid = err
			return nil
		}
		if string(msg.ID) == id && msg.Method == "" {
			return msg
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
			if invalid != nil {
				return nil, invalid
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if len(data) != 0 {
		return nil, badReply("the event stream ended before the response event was complete")
	}
	if invalid != nil {
		return nil, invalid
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
