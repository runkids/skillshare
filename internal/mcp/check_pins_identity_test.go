package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Each mutation must require review even when advertised tool contracts are equal.
func TestToolBaselineIdentityDrift(t *testing.T) {
	for _, part := range []string{"command", "args", "cwd", "url", "env", "headers", "bearer", "tools", "native", "pi", "protocol", "serverInfo", "policy"} {
		t.Run(part, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			server = Server{Command: "fixture", Env: map[string]Value{"TOKEN": {FromEnv: "FIXTURE_TOKEN"}}}
			directory := t.TempDir()
			now.Command = filepath.Join(directory, "bin", "server")
			now.Dir = filepath.Join(directory, "work")
			if part == "headers" || part == "bearer" {
				server = Server{URL: "https://example.invalid/mcp"}
			}
			baselineAccept(t, s, source, "", "echo", server, now)
			switch part {
			case "command":
				now.Command = filepath.Join(directory, "bin", "other")
			case "args":
				server.Args = []string{"--mode", "other"}
			case "cwd":
				now.Dir = filepath.Join(directory, "other")
			case "url":
				server = Server{URL: "https://other.invalid/mcp"}
				now.Command = ""
				now.Dir = ""
			case "env":
				server.Env["TOKEN"] = Value{FromEnv: "OTHER_TOKEN"}
			case "headers":
				server.Headers = map[string]Value{"Authorization": {FromEnv: "HEADER_TOKEN"}}
			case "bearer":
				server.BearerToken = &Value{FromEnv: "BEARER_TOKEN"}
			case "tools":
				server.Tools = ToolPolicy{Allow: []string{"echo"}}
			case "native":
				server.Options = map[string]map[string]any{"codex": {"startup_timeout_sec": 25}}
			case "pi":
				server.PiOptions = PiOptions{"timeout": 25}
			case "protocol":
				live := *now.Live
				live.ProtocolVersion = "other-version"
				now.Live = &live
			case "serverInfo":
				live := *now.Live
				live.ServerInfo.Version = "2"
				now.Live = &live
			case "policy":
				pins, err := s.readToolPins()
				if err != nil {
					t.Fatal(err)
				}
				for key, record := range pins.Records {
					record.Policy = "skillshare-contract-v1"
					pins.Records[key] = record
				}
				data, err := json.Marshal(pins)
				if err != nil {
					t.Fatal(err)
				}
				writeBaseline(t, s, data)
			}
			_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			if review := report.Servers[0].Contract; review.Status != "drifted" || !review.IdentityChanged || len(review.Changes) != 0 {
				t.Fatalf("identity drift = %+v", review)
			}
		})
	}
}

func TestToolBaselineSimultaneousIdentityAndToolDrift(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	baselineAccept(t, s, source, "", "echo", server, now)
	server.URL = "https://other.invalid/mcp"
	_, tool, err := captureTool(json.RawMessage(`{"name":"echo","description":"Other instructions","inputSchema":{"type":"object"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{"echo": tool}
	_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	review := report.Servers[0].Contract
	if !review.IdentityChanged || len(review.Changes) == 0 || review.Status != "drifted" {
		t.Fatalf("drift = %+v", review)
	}
}

func TestToolBaselineLiteralDeclarationsNeverFingerprintCredentials(t *testing.T) {
	for _, part := range []string{"env", "header", "bearer"} {
		t.Run(part, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			secret := "fixture-only-inline-credential"
			switch part {
			case "env":
				server = Server{Command: "fixture", Env: map[string]Value{"TOKEN": {Literal: secret}}}
			case "header":
				server.Headers = map[string]Value{"X-Token": {Literal: secret}}
			case "bearer":
				server.BearerToken = &Value{Literal: secret}
			}
			_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			review := report.Servers[0].Contract
			if review.Status != "untrusted" || review.Revision != "" {
				t.Fatalf("literal declaration review = %+v", review)
			}

			for _, opts := range []CheckOptions{{Live: true, StrictTools: true}, {Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: strings.Repeat("a", 64)}} {
				b, refused := baselineObserve(s, source, "", "echo", server, now, opts)
				b.save(s, refused)
				if refused.Servers[0].Contract.Status != "error" || refused.Servers[0].Contract.Revision != "" {
					t.Fatal("literal declaration received trust token")
				}
				assertBaselineLevel(t, refused, "error")
			}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), secret) {
				t.Fatal("literal credential leaked")
			}
			if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
				t.Fatal("literal trust written")
			}
		})
	}
}

func TestToolBaselineCanonicalConfigAliasAndSourceIdentity(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	baselineAccept(t, s, source, "", "echo", server, now)
	alias := *s
	alias.ConfigPath = filepath.Join(filepath.Dir(s.ConfigPath), "alias.yaml")
	if err := os.Symlink(s.ConfigPath, alias.ConfigPath); err != nil {
		t.Skip(err)
	}
	aliasSource, err := LoadSource(alias.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	_, report := baselineObserve(&alias, aliasSource, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "unchanged" {
		t.Fatal("canonical config alias lost approval")
	}
	// Repointing an external source, even with identical declarations, changes its identity.
	external := filepath.Join(filepath.Dir(s.ConfigPath), "external.yaml")
	if err := os.WriteFile(external, []byte("servers:\n  echo:\n    url: https://example.invalid/mcp\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, []byte("sources:\n  mcp: external.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	externalSource, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	_, report = baselineObserve(s, externalSource, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "untrusted" {
		t.Fatal("external source inherited approval")
	}
}

func sourcePathPrivacyEndpoint(t *testing.T) string {
	t.Helper()
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch request.Method {
		case "server/discover":
			result = map[string]any{"supportedVersions": []string{liveProtocolVersion}, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": LiveServerInfo{Name: "path-fixture", Version: "1"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	t.Cleanup(endpoint.Close)
	return endpoint.URL
}

// The credential is declared by another server: selecting echo must still use
// the complete source's known-secret set for display and canonical identities.
func sourcePathPrivacyFixture(t *testing.T, url string, external bool) (*Service, *Source, string) {
	t.Helper()
	const secret = "source-path-fixture-token-831"
	base := t.TempDir()
	sensitiveDir := filepath.Join(base, secret)
	if err := os.MkdirAll(sensitiveDir, 0700); err != nil {
		t.Fatal(err)
	}
	servers := "  echo:\n    targets: []\n    url: " + url + "\n  auth:\n    targets: []\n    url: " + url + "\n    bearerToken: {fromEnv: FIXTURE_SOURCE_TOKEN}\n"
	configPath := filepath.Join(sensitiveDir, "config.yaml")
	// Construct indentation explicitly so source loading, not a hand-built Source,
	// exercises the supported inline and external configuration forms.
	config := "mcp:\n  targets: []\n  servers:\n    echo:\n      targets: []\n      url: " + url + "\n    auth:\n      targets: []\n      url: " + url + "\n      bearerToken: {fromEnv: FIXTURE_SOURCE_TOKEN}\n"
	if external {
		externalPath := filepath.Join(sensitiveDir, "servers.yaml")
		if err := os.WriteFile(externalPath, []byte("servers:\n"+servers), 0600); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(externalPath)
		configPath = filepath.Join(base, "config.yaml")
		config = "mcp:\n  targets: []\nsources:\n  mcp: " + string(encoded) + "\n"
	}
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: configPath, Home: base, StateDir: filepath.Join(base, "state"), Platform: "linux"}
	source, err := LoadSource(configPath)
	if err != nil {
		t.Fatal(err)
	}
	return s, source, secret
}

func sourcePathPrivacyOptions(secret string) CheckOptions {
	opts := offlineCheck("echo")
	opts.LookupEnv = func(key string) (string, bool) { return secret, key == "FIXTURE_SOURCE_TOKEN" }
	return opts
}

func TestCheckSourcePathDisplayRedactsKnownCredentials(t *testing.T) {
	endpoint := sourcePathPrivacyEndpoint(t)
	for _, external := range []bool{false, true} {
		for _, live := range []bool{false, true} {
			t.Run(fmt.Sprintf("external=%v/live=%v", external, live), func(t *testing.T) {
				s, source, secret := sourcePathPrivacyFixture(t, endpoint, external)
				configPath, sourcePath := source.ConfigPath, source.Path
				opts := sourcePathPrivacyOptions(secret)
				opts.Live = live
				report, err := s.Check(opts)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(report.SourcePath, secret) || !strings.Contains(report.SourcePath, "[redacted]") {
					t.Fatalf("unsafe display path: %q", report.SourcePath)
				}
				if source.ConfigPath != configPath || source.Path != sourcePath || s.ConfigPath != configPath {
					t.Fatal("display redaction mutated operational paths")
				}
				if err := source.CheckUnchanged(); err != nil {
					t.Fatal(err)
				}
				if live {
					review := report.Servers[0].Contract
					if review == nil || review.Status != "error" || review.Revision != "" {
						t.Fatalf("credential path yielded review authority: %+v", report)
					}
				}
				if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
					t.Fatal("path privacy refusal wrote trust")
				}
			})
		}
	}
}

func TestToolConfigIdentityRejectsCanonicalCredentialPaths(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%v", external), func(t *testing.T) {
			s, source, secret := sourcePathPrivacyFixture(t, "https://example.invalid/mcp", external)
			alias := filepath.Join(s.Home, "neutral-alias.yaml")
			if err := os.Symlink(source.Path, alias); err != nil {
				t.Skip(err)
			}
			if external {
				encoded, _ := json.Marshal(alias)
				if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: []\nsources:\n  mcp: "+string(encoded)+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				s.ConfigPath = alias
			}
			source, err := LoadSource(s.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(source.Path, secret) || strings.Contains(source.ConfigPath, secret) {
				t.Fatal("fixture lexical path must be neutral")
			}
			identity, err := toolConfigIdentity(source, []string{secret})
			if err == nil || identity != "" || strings.Contains(err.Error(), secret) {
				t.Fatalf("canonical credential path hashed or leaked: %q %v", identity, err)
			}
			opts := sourcePathPrivacyOptions(secret)
			opts.Live = true
			opts.knownSecrets = sourceLiveSecrets(source, opts)
			_, _, _, now := baselineFixture(t)
			for _, accept := range []bool{false, true} {
				opts.AcceptTools = accept
				if accept {
					opts.Revision = strings.Repeat("a", 64)
				}
				b, report := baselineObserve(s, source, "", "echo", source.Servers["echo"], now, opts)
				if b.err == nil || b.config != "" {
					t.Fatal("baseline load omitted canonical path guard")
				}
				b.save(s, report)
				if review := report.Servers[0].Contract; review.Status != "error" || review.Revision != "" {
					t.Fatal("canonical credential path produced review authority")
				}
				if accept && report.ToolsAcceptance != ToolsAcceptanceError {
					t.Fatal("canonical credential path accepted")
				}
				payload, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(payload), secret) {
					t.Fatal("source identity refusal exposed path credential")
				}
				if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
					t.Fatal("source identity refusal wrote trust")
				}
			}
			if err := source.CheckUnchanged(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestToolConfigIdentityDoesNotHashRedactedDisplayPaths(t *testing.T) {
	s, source, secret := sourcePathPrivacyFixture(t, "https://example.invalid/mcp", false)
	clean := filepath.Join(s.Home, "clean-config.yaml")
	if err := os.WriteFile(clean, source.configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	// The lexical alias contains the recognized value, but its real destination
	// is clean. Display is redacted while the actual canonical identity is kept.
	alias := s.ConfigPath
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(clean, alias); err != nil {
		t.Skip(err)
	}
	aliasSource, err := LoadSource(alias)
	if err != nil {
		t.Fatal(err)
	}
	cleanSource, err := LoadSource(clean)
	if err != nil {
		t.Fatal(err)
	}
	fromAlias, err := toolConfigIdentity(aliasSource, []string{secret})
	if err != nil {
		t.Fatal(err)
	}
	fromClean, err := toolConfigIdentity(cleanSource, []string{secret})
	if err != nil {
		t.Fatal(err)
	}
	if fromAlias == "" || fromAlias != fromClean {
		t.Fatal("display redaction changed canonical identity")
	}

}

func TestToolBaselineLockedSourceIdentityRetainsCredentialGuard(t *testing.T) {
	s, sensitiveSource, secret := sourcePathPrivacyFixture(t, "https://example.invalid/mcp", false)
	clean := filepath.Join(s.Home, "clean-config.yaml")
	if err := os.WriteFile(clean, sensitiveSource.configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(s.Home, "neutral-config-alias.yaml")
	if err := os.Symlink(clean, alias); err != nil {
		t.Skip(err)
	}
	s.ConfigPath = alias
	source, err := LoadSource(alias)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, now := baselineFixture(t)
	opts := sourcePathPrivacyOptions(secret)
	opts.Live = true
	opts.knownSecrets = sourceLiveSecrets(source, opts)
	_, preview := baselineObserve(s, source, "", "echo", source.Servers["echo"], now, opts)
	opts.AcceptTools = true
	opts.Revision = preview.Servers[0].Contract.Revision
	b, report := baselineObserve(s, source, "", "echo", source.Servers["echo"], now, opts)
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceAccepted {
		t.Fatalf("clean canonical identity not accepted: %+v", report)
	}
	before := baselineBytes(t, s)
	opts.AcceptTools = false
	opts.Revision = ""
	_, preview = baselineObserve(s, source, "", "echo", source.Servers["echo"], now, opts)
	opts.AcceptTools = true
	opts.Revision = preview.Servers[0].Contract.Revision
	b, report = baselineObserve(s, source, "", "echo", source.Servers["echo"], now, opts)
	if b.updates[0] == nil {
		t.Fatal("no valid no-op queued")
	}
	// Retarget only the fixture symlink to byte-identical configuration under a
	// recognized-credential directory. Byte CAS succeeds; canonical identity must refuse.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sensitiveSource.ConfigPath, alias); err != nil {
		t.Fatal(err)
	}
	if err := source.CheckUnchanged(); err != nil {
		t.Fatalf("fixture should satisfy byte CAS: %v", err)
	}
	if identity, err := toolConfigIdentity(source, opts.knownSecrets); err == nil || identity != "" {
		t.Fatal("canonical recheck yielded credential-derived identity")
	}
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceStale || report.Servers[0].Contract.Status != "error" || report.Servers[0].Contract.Revision != "" {
		t.Fatalf("locked no-op bypassed path guard: %+v", report)
	}
	if string(before) != string(baselineBytes(t, s)) {
		t.Fatal("locked path refusal changed prior store")
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatal("locked path refusal leaked credential")
	}
}

// Unix filesystems can distinguish bytes that encoding/json would replace with
// the same escape. Such real paths cannot participate in review authority.
func TestToolConfigIdentityRejectsInvalidUTF8FilesystemPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("raw invalid-UTF8 filenames require Unix filesystem semantics")
	}
	base := t.TempDir()
	for _, external := range []bool{false, true} {
		for _, invalid := range []byte{0xff, 0xfe} {
			t.Run(fmt.Sprintf("external=%v/byte=%x", external, invalid), func(t *testing.T) {
				directory := filepath.Join(base, fmt.Sprintf("external-%v-", external)+string([]byte{invalid}))
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Skipf("filesystem does not support fixture path bytes: %v", err)
				}
				configPath := filepath.Join(directory, "config.yaml")
				config := []byte("mcp:\n  targets: []\n  servers:\n    echo:\n      url: https://example.invalid/mcp\n")
				if external {
					sourcePath := filepath.Join(directory, "servers.yaml")
					if err := os.WriteFile(sourcePath, []byte("servers:\n  echo:\n    url: https://example.invalid/mcp\n"), 0600); err != nil {
						t.Fatal(err)
					}
					alias := filepath.Join(base, fmt.Sprintf("source-%x.yaml", invalid))
					if err := os.Symlink(sourcePath, alias); err != nil {
						t.Skip(err)
					}
					encoded, _ := json.Marshal(alias)
					configPath = filepath.Join(base, fmt.Sprintf("config-%x.yaml", invalid))
					config = []byte("mcp:\n  targets: []\nsources:\n  mcp: " + string(encoded) + "\n")
				}
				if err := os.WriteFile(configPath, config, 0600); err != nil {
					t.Fatal(err)
				}
				source, err := LoadSource(configPath)
				if err != nil {
					t.Fatal(err)
				}
				identity, err := toolConfigIdentity(source, nil)
				if err == nil || identity != "" {
					t.Fatal("invalid UTF-8 canonical path produced identity")
				}
				if err.Error() != "MCP source identity is unavailable" {
					t.Fatalf("path-bearing identity diagnostic: %v", err)
				}
				s := &Service{ConfigPath: configPath, Home: base, StateDir: filepath.Join(base, "state"), Platform: "linux"}
				_, _, _, now := baselineFixture(t)
				b, report := baselineObserve(s, source, "", "echo", source.Servers["echo"], now, CheckOptions{Live: true})
				b.save(s, report)
				if b.config != "" || report.Servers[0].Contract.Status != "error" || report.Servers[0].Contract.Revision != "" {
					t.Fatal("invalid filesystem path yielded review authority")
				}
				if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
					t.Fatal("invalid path wrote approval")
				}
				if err := source.CheckUnchanged(); err != nil {
					t.Fatal("identity refusal broke source byte checks")
				}
			})
		}
	}
}

func TestToolConfigIdentityPreservesValidReplacementRunePath(t *testing.T) {
	base := t.TempDir()
	directory := filepath.Join(base, "valid-\ufffd")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  targets: []\n  servers:\n    echo:\n      url: https://example.invalid/mcp\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(configPath)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := toolConfigIdentity(source, nil)
	if err != nil || !toolHashValid(identity) {
		t.Fatalf("valid replacement-rune path refused: %v", err)
	}
	s := &Service{ConfigPath: configPath, Home: base, StateDir: filepath.Join(base, "state"), Platform: "linux"}
	_, _, _, now := baselineFixture(t)
	baselineAccept(t, s, source, "", "echo", source.Servers["echo"], now)
}

func TestToolConnectionIdentityRejectsInvalidUTF8LaunchObservations(t *testing.T) {
	for _, field := range []string{"command", "cwd"} {
		for _, invalid := range []byte{0xff, 0xfe} {
			t.Run(fmt.Sprintf("%s/%x", field, invalid), func(t *testing.T) {
				s, source, _, now := baselineFixture(t)
				directory := t.TempDir()
				server := Server{Command: "fixture"}
				now.Command = filepath.Join(directory, "bin", "server")
				now.Dir = filepath.Join(directory, "work")
				invalidPath := filepath.Join(directory, "invalid-"+string([]byte{invalid}))
				if field == "command" {
					now.Command = invalidPath
				} else {
					now.Dir = invalidPath
				}
				identity, err := toolConnectionIdentity(server, now)
				if err == nil || identity != "" {
					t.Fatal("invalid UTF-8 launch observation yielded identity")
				}
				_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
				if review := report.Servers[0].Contract; review.Status != "error" || review.Revision != "" {
					t.Fatal("invalid launch observation yielded revision")
				}
				if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
					t.Fatal("invalid launch identity wrote store")
				}
			})
		}
	}
}

func TestToolConnectionIdentityPreservesValidUnicodeAndOmitsHTTPLaunch(t *testing.T) {
	s, source, httpServer, now := baselineFixture(t)
	directory := t.TempDir()
	now.Command = filepath.Join(directory, "bin-\ufffd", "server")
	now.Dir = filepath.Join(directory, "work-\ufffd")
	stdioServer := Server{Command: "fixture"}
	identity, err := toolConnectionIdentity(stdioServer, now)
	if err != nil || !toolHashValid(identity) {
		t.Fatalf("valid Unicode stdio path refused: %v", err)
	}
	baselineAccept(t, s, source, "", "echo", stdioServer, now)
	cleanHTTP, err := toolConnectionIdentity(httpServer, now)
	if err != nil {
		t.Fatal(err)
	}
	now.Command = filepath.Join(directory, string([]byte{0xff}))
	now.Dir = filepath.Join(directory, string([]byte{0xfe}))
	unusedHTTP, err := toolConnectionIdentity(httpServer, now)
	if err != nil || cleanHTTP != unusedHTTP {
		t.Fatalf("unused HTTP launch bytes affected identity: %v", err)
	}
}

func TestToolConnectionIdentityIgnoresClientSelection(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	server.Targets = TargetList{"claude"}
	first, err := toolConnectionIdentity(server, now)
	if err != nil {
		t.Fatal(err)
	}
	baselineAccept(t, s, source, "", "echo", server, now)
	for _, tc := range []struct {
		name     string
		targets  TargetList
		disabled bool
	}{
		{"other target", TargetList{"codex"}, false},
		{"explicitly empty", TargetList{}, false},
		{"disabled", TargetList{"claude"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := server
			changed.Targets, changed.Disabled = tc.targets, tc.disabled
			identity, err := toolConnectionIdentity(changed, now)
			if err != nil || identity != first {
				t.Fatalf("selection changed identity: %v", err)
			}
			if !tc.disabled {
				_, report := baselineObserve(s, source, "", "echo", changed, now, CheckOptions{Live: true})
				if review := report.Servers[0].Contract; review.Status != "unchanged" || review.IdentityChanged {
					t.Fatalf("selection changed approval: %+v", review)
				}
			}
		})
	}
}
