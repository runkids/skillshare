package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProbeDraftCredentialDiagnosticDistinguishesScopeAndIdentity(t *testing.T) {
	const secret = "draft-fixture-token-619"
	root := t.TempDir()
	for label, tc := range map[string]struct{ root, command, want string }{
		"scope":    {filepath.Join(root, secret), "fixture", "project scope contains a configured credential; no review is available"},
		"identity": {root, "fixture-" + secret, "connection identity contains a configured credential; use environment-backed references"},
	} {
		t.Run(label, func(t *testing.T) {
			lookups := 0
			got, err := ProbeDraft(Server{Command: tc.command, Env: map[string]Value{"TOKEN": {FromEnv: "FIXTURE_DRAFT_TOKEN"}}}, tc.root, CheckOptions{
				LookupEnv: func(key string) (string, bool) { return secret, key == "FIXTURE_DRAFT_TOKEN" },
				LookPath:  func(string) (string, error) { lookups++; return "", errors.New("unexpected command lookup") },
			})
			if err != nil || got.Live != nil || got.ErrorKind != "protocol" || got.Error != tc.want || strings.Contains(got.Error, secret) || lookups != 0 {
				t.Fatalf("wrong preflight diagnostic or lookup: %+v %v lookups=%d", got, err, lookups)
			}
		})
	}
}

func TestCheckRejectsCredentialInEffectiveProjectRoot(t *testing.T) {
	isolatedContractAudit(t)
	const secret = "effective-root-fixture-token-913"
	var requests atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req httpRequest
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var result any = map[string]any{"tools": []any{map[string]any{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}
		if req.Method == "server/discover" {
			result = map[string]any{"supportedVersions": []string{liveProtocolVersion}, "capabilities": map[string]any{"tools": map[string]any{}}, "_meta": map[string]any{"io.modelcontextprotocol/serverInfo": LiveServerInfo{Name: "fixture", Version: "1"}}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer endpoint.Close()
	base := t.TempDir()
	root := filepath.Join(base, secret)
	configPath := filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	neutral := filepath.Join(base, "shared-config.yaml")
	if err := os.WriteFile(neutral, []byte("mcp:\n  targets: []\n  servers:\n    echo:\n      targets: []\n      url: "+endpoint.URL+"\n      bearerToken: {fromEnv: FIXTURE_ROOT_TOKEN}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(neutral, configPath); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	s := &Service{ConfigPath: configPath, ProjectRoot: root, Home: base, StateDir: filepath.Join(base, "state"), Platform: "linux"}
	source, err := LoadSource(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if s.configProjectRoot() != root {
		t.Fatal("fixture did not retain the effective project root")
	}
	if _, err := toolConfigIdentity(source, []string{secret}); err != nil {
		t.Fatal("fixture must have credential-free canonical source paths")
	}
	// Establish a genuine approval before rotation makes the root confidential.
	opts := CheckOptions{Live: true, SkipDNS: true, Names: []string{"echo"}, LookupEnv: func(key string) (string, bool) { return "initial-fixture-token", key == "FIXTURE_ROOT_TOKEN" }}
	preview, err := s.Check(opts)
	if err != nil || preview.Servers[0].Contract == nil || preview.Servers[0].Contract.Revision == "" {
		t.Fatalf("initial complete review failed: %v", err)
	}
	revision := preview.Servers[0].Contract.Revision
	opts.AcceptTools, opts.Revision = true, revision
	accepted, err := s.Check(opts)
	if err != nil || accepted.ToolsAcceptance != ToolsAcceptanceAccepted {
		t.Fatalf("initial approval failed: %v", err)
	}
	opts.AcceptTools, opts.Revision = false, ""
	preview, err = s.Check(opts)
	if err != nil || preview.Servers[0].Contract == nil || preview.Servers[0].Contract.Revision == "" || preview.Servers[0].Contract.Status != "unchanged" {
		t.Fatalf("approved baseline could not be reviewed: %v", err)
	}
	revision = preview.Servers[0].Contract.Revision
	before := baselineBytes(t, s)
	if requests.Load() == 0 {
		t.Fatal("initial review never reached the live endpoint")
	}
	visits := requests.Load()
	opts.LookupEnv = func(key string) (string, bool) { return secret, key == "FIXTURE_ROOT_TOKEN" }
	for _, accept := range []bool{false, true} {
		opts.AcceptTools, opts.Revision = accept, ""
		if accept {
			opts.Revision = revision
		}
		report, err := s.Check(opts)
		if err != nil || report.Summary.Errors == 0 {
			t.Fatalf("credential-bearing effective root was not refused: %v", err)
		}
		scopeDiagnostic := false
		for _, finding := range report.Servers[0].Findings {
			if finding.Check == "live" && finding.Level == "error" {
				if finding.Message != "project scope contains a configured credential; no review is available" {
					t.Fatal("path-only refusal did not identify the project scope")
				}
				scopeDiagnostic = true
			}
		}
		if !scopeDiagnostic {
			t.Fatal("missing effective-root preflight diagnostic")
		}
		if requests.Load() != visits || report.Servers[0].Live != nil {
			t.Fatal("credential-bearing effective root reached the live endpoint")
		}
		if review := report.Servers[0].Contract; review != nil && review.Revision != "" {
			t.Fatal("credential-bearing effective root exposed a review revision")
		}
		payload, err := json.Marshal(report)
		if err != nil || strings.Contains(string(payload), secret) {
			t.Fatal("effective root credential leaked in diagnostics")
		}
		if accept && report.ToolsAcceptance != ToolsAcceptanceError {
			t.Fatal("credential-bearing effective root did not refuse acceptance")
		}
		if !bytes.Equal(before, baselineBytes(t, s)) {
			t.Fatal("refusal changed the existing baseline")
		}
	}

}

func TestCheckProjectScopePreservesControlsAndRedactsCredentials(t *testing.T) {
	const secret = "scope-fixture-secret"
	for _, tc := range []struct {
		name, suffix string
		redacted     bool
	}{
		{"controls", "project\u202e\x1b", false},
		{"credential", "project-" + secret + "\u202e", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), tc.suffix)
			config := fmt.Sprintf("mcp:\n  targets: []\n  servers:\n    credentials:\n      command: fixture\n      env:\n        TOKEN: {fromEnv: FIXTURE_TOKEN}\n  projects:\n    %q:\n      targets: []\n      servers:\n        entry:\n          disabled: true\n", root)
			s := checkService(t, config)
			report, err := s.Check(CheckOptions{Root: &root, SkipDNS: true, LookupEnv: func(key string) (string, bool) { return secret, key == "FIXTURE_TOKEN" }})
			if err != nil || len(report.Servers) != 1 {
				t.Fatalf("check: %v", err)
			}
			got := report.Servers[0].Project
			if !tc.redacted && got != root {
				t.Fatalf("scope changed: got %q want %q", got, root)
			}
			if tc.redacted && (strings.Contains(got, secret) || !strings.Contains(got, "[redacted]") || report.Summary.Errors == 0) {
				t.Fatalf("scope not refused and redacted: %q", got)
			}
		})
	}
}
