package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProjectInitArgs_Visible(t *testing.T) {
	opts, showHelp, err := parseProjectInitArgs([]string{"--visible"})
	if err != nil {
		t.Fatalf("parseProjectInitArgs failed: %v", err)
	}
	if showHelp {
		t.Fatal("--visible should not request help")
	}
	if !opts.visible {
		t.Error("opts.visible = false, want true")
	}

	opts, _, err = parseProjectInitArgs([]string{"--targets", "claude"})
	if err != nil {
		t.Fatalf("parseProjectInitArgs failed: %v", err)
	}
	if opts.visible {
		t.Error("opts.visible = true without --visible")
	}
}

func TestPerformProjectInit_Visible(t *testing.T) {
	root := t.TempDir()

	if err := performProjectInit(root, projectInitOptions{visible: true, targets: []string{"claude"}}); err != nil {
		t.Fatalf("performProjectInit failed: %v", err)
	}

	dir := filepath.Join(root, "skillshare")
	for _, sub := range []string{"config.yaml", "skills", "agents", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("missing %s in visible project directory: %v", sub, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".skillshare")); !os.IsNotExist(err) {
		t.Error("--visible also created a hidden .skillshare directory")
	}
	if !projectConfigExists(root) {
		t.Error("visible project not detected by projectConfigExists")
	}
}

func TestPerformProjectInit_DefaultStaysHidden(t *testing.T) {
	root := t.TempDir()

	if err := performProjectInit(root, projectInitOptions{targets: []string{"claude"}}); err != nil {
		t.Fatalf("performProjectInit failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".skillshare", "config.yaml")); err != nil {
		t.Errorf("default init did not create .skillshare/config.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skillshare")); !os.IsNotExist(err) {
		t.Error("default init created a visible skillshare directory")
	}
}

// A second init must report the existing project rather than creating the other
// layout alongside it.
func TestPerformProjectInit_VisibleProjectIsAlreadyInitialized(t *testing.T) {
	root := t.TempDir()

	if err := performProjectInit(root, projectInitOptions{visible: true, targets: []string{"claude"}}); err != nil {
		t.Fatalf("performProjectInit failed: %v", err)
	}

	err := performProjectInit(root, projectInitOptions{targets: []string{"claude"}})
	if err == nil {
		t.Fatal("re-init of a visible project succeeded, want already-initialized error")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".skillshare")); !os.IsNotExist(statErr) {
		t.Error("re-init created a hidden directory next to the visible project")
	}
}
