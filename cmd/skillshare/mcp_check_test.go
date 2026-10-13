package main

import (
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

func TestLogMCPAcceptToolsListsUnsavedBaselinesAsFailed(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	accepted := mcp.CheckFinding{Level: "info", Check: "tool-change", Message: "accepted the current tool definitions as the baseline (changed: read)"}
	report := &mcp.CheckReport{Servers: []mcp.CheckServer{
		{Name: "saved", Findings: []mcp.CheckFinding{accepted}},
		{Name: "unsaved", Findings: []mcp.CheckFinding{accepted, {Level: "warning", Check: "tool-change", Message: "could not save the tool baseline: another MCP operation is running"}}},
	}}
	logMCPAcceptTools(cfgPath, time.Now(), report, nil)

	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
	e := entries[0]
	if e.Status != "error" || len(e.Args["accepted"].([]any)) != 1 || e.Args["accepted"].([]any)[0] != "saved" || e.Args["failed"].([]any)[0] != "unsaved" {
		t.Fatalf("entry = %+v", e)
	}
}

func TestLogMCPAcceptToolsCountsFirstPinsAndReportErrors(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	report := &mcp.CheckReport{Servers: []mcp.CheckServer{
		{Name: "new", Findings: []mcp.CheckFinding{{Level: "info", Check: "tool-change", Message: "pinned 2 tool definition(s) as the baseline"}}},
		{Name: "broken", Findings: []mcp.CheckFinding{{Level: "error", Check: "command", Message: "command nope was not found on PATH"}}},
	}}
	logMCPAcceptTools(cfgPath, time.Now(), report, nil)

	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
	e := entries[0]
	if e.Status != "error" || e.Args["accepted"].([]any)[0] != "new" || e.Args["failed"].([]any)[0] != "broken" {
		t.Fatalf("entry = %+v", e)
	}
}
