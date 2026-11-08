package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCaptureContractRejectsNumericCredentialLexemes(t *testing.T) {
	for _, tt := range []struct{ number, secret string }{
		{"9007199254740993", "9007199254740993"},
		{"-12345.67890", "-12345.67890"},
		{"1.23e+90", "1.23e+90"},
		{"17", "7"},
	} {
		for label, template := range map[string]string{
			"metadata": `{"name":"echo","inputSchema":{},"_meta":{"nested":[%s]}}`,
			"schema":   `{"name":"echo","inputSchema":{"default":{"nested":[%s]}}}`,
		} {
			t.Run(tt.number+"/"+label, func(t *testing.T) {
				// Resolve a synthetic fromEnv credential through the same probe setup as Check.
				p := newLiveProbe(Server{Command: "fixture", Env: map[string]Value{"TOKEN": {FromEnv: "FIXTURE_TOKEN"}}}, "", DefaultLiveTimeout, CheckOptions{
					LookupEnv: func(string) (string, bool) { return tt.secret, true },
				})
				raw := fmt.Sprintf(template, tt.number)
				if _, _, err := captureTool(json.RawMessage(raw), nil); err != nil {
					t.Fatalf("invalid regression fixture: %v", err)
				}
				name, tool, err := captureTool(json.RawMessage(raw), p.secrets)
				if err == nil || name != "" || tool.Hash != "" || len(tool.Definition) != 0 || strings.Contains(err.Error(), tt.secret) {
					t.Fatal("numeric credential accepted or exposed")
				}
			})
		}
	}
}

func TestToolBaselineRejectsNumericCredentialsInCurrentAndStoredSnapshots(t *testing.T) {
	const secret = "9007199254740993"
	for _, stored := range []bool{false, true} {
		for label, raw := range map[string]string{
			"metadata": `{"name":"echo","inputSchema":{},"_meta":{"nested":[` + secret + `]}}`,
			"schema":   `{"name":"echo","inputSchema":{"default":{"nested":[` + secret + `]}}}`,
		} {
			t.Run(fmt.Sprintf("%s/stored=%v", label, stored), func(t *testing.T) {
				s, source, server, now := baselineFixture(t)
				server.Headers = map[string]Value{"X-Token": {FromEnv: "FIXTURE_TOKEN"}}
				name, tool, err := captureTool(json.RawMessage(raw), nil)
				if err != nil {
					t.Fatal(err)
				}
				tainted := now
				tainted.Tools = map[string]ToolContract{name: tool}
				var before []byte
				if stored {
					// A rotated credential newly matches an already approved numeric value.
					baselineAccept(t, s, source, "", "echo", server, tainted)
					before = baselineBytes(t, s)
				} else {
					now = tainted
				}
				_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
				revision := preview.Servers[0].Contract.Revision
				if revision == "" {
					t.Fatal("fixture never reached a complete review")
				}
				now.Secrets = []string{secret}
				for _, opts := range []CheckOptions{{Live: true}, {Live: true, StrictTools: true}, {Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: revision}} {
					b, report := baselineObserve(s, source, "", "echo", server, now, opts)
					b.save(s, report)
					review := report.Servers[0].Contract
					if review.Status != "error" || review.Revision != "" || len(review.Changes) != 0 || (!stored && len(review.Tools) != 0) {
						t.Fatal("numeric credential entered a review or approval")
					}
					assertBaselineLevel(t, report, "error")
					if opts.AcceptTools && report.ToolsAcceptance != ToolsAcceptanceError {
						t.Fatal("numeric credential did not refuse explicit acceptance")
					}
					payload, err := json.Marshal(report)
					if err != nil || bytes.Contains(payload, []byte(secret)) {
						t.Fatal("numeric credential exposed in report")
					}
					if stored {
						if !bytes.Equal(before, baselineBytes(t, s)) {
							t.Fatal("refusal changed stored approval")
						}
					} else if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
						t.Fatal("refusal created stored approval")
					}
				}
			})
		}
	}
}

func TestNumericConnectionCredentialRejectedBeforeProbeAndFingerprint(t *testing.T) {
	const secret = "9007199254740993"
	var requests atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer endpoint.Close()
	for _, option := range []any{int64(9007199254740993), map[string]any{"nested": []any{int64(9007199254740993)}}} {
		s, source, server, now := baselineFixture(t)
		server.URL = endpoint.URL
		server.Options = map[string]map[string]any{"claude": {"timeout": option}}
		if err := server.Validate("echo"); err != nil {
			t.Fatalf("unsupported regression fixture: %v", err)
		}
		now.Secrets = []string{secret}
		if !serverHasKnownSecret(server, "", "echo", now.Secrets) {
			t.Fatal("numeric credential passed connection preflight")
		}
		p := newLiveProbe(server, "", DefaultLiveTimeout, CheckOptions{knownSecrets: now.Secrets})
		if live, err := p.probe(); err == nil || live != nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "connection identity") {
			t.Fatal("numeric credential reached live transport")
		}
		if identity, err := toolConnectionIdentity(server, now); err == nil || identity != "" || strings.Contains(err.Error(), secret) {
			t.Fatal("numeric credential produced a connection fingerprint")
		}
		_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
		if review := report.Servers[0].Contract; review.Status != "error" || review.Revision != "" {
			t.Fatal("numeric connection credential yielded review authority")
		}
	}
	if requests.Load() != 0 {
		t.Fatal("numeric connection credential reached the endpoint")
	}
}
