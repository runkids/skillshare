package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
)

func TestHandlePutConfig_WritesOpsLog(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	initialRaw := "source: " + sourceDir + "\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(initialRaw), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	s := New(&config.Config{
		Source:  sourceDir,
		Targets: map[string]config.TargetConfig{},
	}, "127.0.0.1:0")

	payload, _ := json.Marshal(map[string]string{"raw": initialRaw})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	entries, err := oplog.Read(cfgPath, oplog.OpsFile, 10)
	if err != nil {
		t.Fatalf("failed to read ops log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one operations log entry")
	}
	if entries[0].Command != "config" {
		t.Fatalf("expected latest command to be config, got %q", entries[0].Command)
	}
	if entries[0].Status != "ok" {
		t.Fatalf("expected latest status to be ok, got %q", entries[0].Status)
	}
}

func TestHandleAuditAll_WritesAuditLog(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	skillDir := filepath.Join(sourceDir, "safe-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Safe\n\nNo suspicious content."), 0644); err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	initialRaw := "source: " + sourceDir + "\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(initialRaw), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	s := New(&config.Config{
		Source:  sourceDir,
		Targets: map[string]config.TargetConfig{},
	}, "127.0.0.1:0")

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	entries, err := oplog.Read(cfgPath, oplog.AuditFile, 10)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one audit log entry")
	}
	if entries[0].Command != "audit" {
		t.Fatalf("expected latest command to be audit, got %q", entries[0].Command)
	}
	if entries[0].Status != "ok" {
		t.Fatalf("expected latest status to be ok, got %q", entries[0].Status)
	}
}
