package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

func TestLogMCPAcceptToolsListsUnsavedBaselinesAsFailed(t *testing.T) {
	for _, failed := range []mcp.CheckServer{
		{Name: "unsaved", Contract: &mcp.ContractReview{Status: "error"}},
		{Name: "broken", Findings: []mcp.CheckFinding{{Level: "error", Check: "command", Message: "command nope was not found on PATH"}}},
	} {
		t.Run(failed.Name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			report := &mcp.CheckReport{ToolsAcceptance: mcp.ToolsAcceptanceError, Servers: []mcp.CheckServer{{Name: "saved", Contract: &mcp.ContractReview{Status: "accepted"}}, failed}}
			logMCPAcceptTools(cfgPath, time.Now(), report, nil)
			entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
			if err != nil || len(entries) != 1 {
				t.Fatalf("entries = %+v, err = %v", entries, err)
			}
			e := entries[0]
			if e.Status != "error" || len(e.Args["accepted"].([]any)) != 1 || e.Args["accepted"].([]any)[0] != "saved" || e.Args["failed"].([]any)[0] != failed.Name {
				t.Fatalf("entry = %+v", e)
			}
		})
	}
}

func TestMCPCheckJSONControlsRemainLosslessAndInert(t *testing.T) {
	report := &mcp.CheckReport{Servers: []mcp.CheckServer{{Name: "fixture", Contract: &mcp.ContractReview{Tools: map[string]mcp.ToolContract{"read": {Definition: json.RawMessage(`{"name":"read","description":"` + "\u009b\u202e\U000e0001" + `","inputSchema":{"type":"object","maximum":9007199254740993}}`)}}}}}}
	raw, err := json.Marshal(mcpCheckJSON{report})
	if err != nil || strings.ContainsAny(string(raw), "\u009b\u202e\U000e0001") || !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("unsafe or lossy wire: %s %v", raw, err)
	}
	var roundtrip mcp.CheckReport
	if json.Unmarshal(raw, &roundtrip) != nil {
		t.Fatal("wire is not valid JSON")
	}
	var descriptor struct{ Description string }
	if json.Unmarshal(roundtrip.Servers[0].Contract.Tools["read"].Definition, &descriptor) != nil || descriptor.Description != "\u009b\u202e\U000e0001" {
		t.Fatalf("changed descriptor text: %q", descriptor.Description)
	}
}

func TestSafeMCPCheckTextEscapesAstralFormatRunes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"read\U000e0001_file", `read\udb40\udc01_file`},
		{"read\u202e\u009b_file", `read\u202e\u009b_file`},
	} {
		if got := safeMCPCheckText(tc.input); got != tc.want {
			t.Fatalf("safeMCPCheckText(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMCPCheckSourcePathDisplayDoesNotLeakKnownCredential(t *testing.T) {
	const secret = "cli-source-path-fixture-462"
	base := t.TempDir()
	directory := filepath.Join(base, secret)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  targets: []\n  servers:\n    echo:\n      targets: []\n      url: https://example.invalid/mcp\n      bearerToken: {fromEnv: FIXTURE_SOURCE_TOKEN}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIXTURE_SOURCE_TOKEN", secret)
	service := &mcp.Service{ConfigPath: configPath, Home: base, StateDir: filepath.Join(base, "state"), Platform: "linux"}
	var checkErr error
	output := captureStdout(t, func() { checkErr = runMCPCheck(service, []string{"echo", "--no-dns"}) })
	if checkErr != nil {
		t.Fatal(checkErr)
	}
	if strings.Contains(output, secret) || !strings.Contains(output, "[redacted]") {
		t.Fatalf("CLI source display leaked credential: %q", output)
	}
	if service.ConfigPath != configPath {
		t.Fatal("CLI redaction changed operational config path")
	}
}
