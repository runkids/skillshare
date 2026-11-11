package mcpchecklog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

func TestWriteAuditRecordsOutcomeWithoutDescriptorOrDiagnosticData(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yaml")
	report := &mcp.CheckReport{ToolsAcceptance: mcp.ToolsAcceptanceAccepted, Servers: []mcp.CheckServer{{Name: "fixture-secret", Contract: &mcp.ContractReview{Status: "accepted", Tools: map[string]mcp.ToolContract{"fixture-secret": {Definition: json.RawMessage(`{"secret":"fixture-secret"}`)}}, Audit: []mcp.ContractAudit{{Message: "fixture-secret"}}}}}}
	if err := WriteAudit(path, time.Now(), report, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteAudit(path, time.Now(), nil, errors.New("fixture-secret")); err != nil {
		t.Fatal(err)
	}
	refused := &mcp.CheckReport{ToolsAcceptance: mcp.ToolsAcceptanceError, Servers: []mcp.CheckServer{{Contract: &mcp.ContractReview{Status: "error", Tools: map[string]mcp.ToolContract{}}}}}
	if err := WriteAudit(path, time.Now(), refused, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := oplog.Read(path, oplog.AuditFile, 0)
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
	if entries[0].Status != "error" || entries[0].Args["accepted"] != false || entries[1].Status != "error" || entries[2].Status != "ok" || entries[2].Command != "mcp contract review" {
		t.Fatalf("outcomes=%v", entries)
	}
	if entries[2].Args["tools"] != float64(1) || entries[2].Args["findings"] != float64(1) || entries[2].Args["accepted"] != true {
		t.Fatalf("counts=%v", entries[2].Args)
	}
	raw, err := os.ReadFile(filepath.Join(oplog.LogDir(path), oplog.AuditFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "fixture-secret") {
		t.Fatal("audit log contains untrusted metadata")
	}

}
