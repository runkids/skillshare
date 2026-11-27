package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/hooks"
)

func TestHooksImportFilePreservesAdoptWithoutAdoptSource(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(settings), 0644); err != nil {
		t.Fatal(err)
	}

	service := &hooks.Service{
		ConfigPath: configPath,
		Home:       home,
		StateDir:   filepath.Join(dir, "state"),
	}
	if err := runHooksImport(service, hooksOptions{
		from: "claude",
		file: settingsPath,
		name: "stop",
		json: true,
	}); err != nil {
		t.Fatalf("runHooksImport --file: %v", err)
	}

	plan, err := service.Preview()
	if err != nil {
		t.Fatalf("Preview after hooks import --file: %v", err)
	}
	if plan.Blocked || len(plan.Changes) != 1 || plan.Changes[0].Action != "adopt" {
		t.Fatalf("expected unblocked adopt after hooks import --file, got blocked=%t changes=%+v", plan.Blocked, plan.Changes)
	}

	// For a code file supplied via --file, AdoptSource is not set so an unrelated native file is not retired.
	codeSnippet := filepath.Join(dir, "snippet.ts")
	if err := os.WriteFile(codeSnippet, []byte("export default function () {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runHooksImport(service, hooksOptions{
		from: "pi",
		file: codeSnippet,
		name: "snippet",
		json: true,
	}); err != nil {
		t.Fatalf("runHooksImport pi --file: %v", err)
	}
}
