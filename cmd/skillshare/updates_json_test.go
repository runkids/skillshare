package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatesJSONFailures(t *testing.T) {
	for _, tt := range []struct {
		name      string
		args      []string
		config    string
		network   bool
		rateLimit bool
	}{
		{name: "unknown action", args: []string{"--json", "unknown"}},
		{name: "unexpected argument before JSON flag", args: []string{"off", "on", "--json"}},
		{name: "unexpected argument after JSON flag", args: []string{"--json", "off", "on"}},
		{name: "preference read", args: []string{"--json"}, config: "updates: ["},
		{name: "preference save on", args: []string{"on", "--json"}, config: "updates: ["},
		{name: "preference save off", args: []string{"off", "--json"}, config: "updates: ["},
		{name: "network failure", args: []string{"check", "--json"}, network: true},
		{name: "rate limit", args: []string{"check", "--json"}, network: true, rateLimit: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			t.Setenv("SKILLSHARE_CONFIG", path)
			if err := os.WriteFile(path, []byte(tt.config), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			oldTransport := http.DefaultTransport
			if tt.network {
				http.DefaultTransport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					if !tt.rateLimit {
						return nil, errors.New("network unavailable")
					}
					return &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"message":"API rate limit exceeded"}`)), Request: r}, nil
				})
				t.Cleanup(func() { http.DefaultTransport = oldTransport })
			}
			var gotErr error
			out := captureStdout(t, func() { gotErr = cmdUpdates(tt.args) })
			var silent *jsonSilentError
			if !errors.As(gotErr, &silent) {
				t.Fatalf("error must suppress main's text output, got %v", gotErr)
			}
			var payload struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &payload); err != nil || payload.Error == "" {
				t.Fatalf("expected structured JSON error, got %q: %v", out, err)
			}
			if tt.network && calls != 1 {
				t.Fatalf("expected one explicit request, got %d", calls)
			}
		})
	}
}

func TestUpdatesPlainFailureRemainsPlain(t *testing.T) {
	var gotErr error
	out := captureStdout(t, func() { gotErr = cmdUpdates([]string{"unknown"}) })
	var silent *jsonSilentError
	if gotErr == nil || errors.As(gotErr, &silent) || out != "" {
		t.Fatalf("plain error changed: %v / %q", gotErr, out)
	}
}
