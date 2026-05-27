//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestExtrasExtension_TransformsMarkdownToToml(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Global extensions dir: <Home>/.config/skillshare/extensions/upper2toml/
	extDir := filepath.Join(sb.Home, ".config", "skillshare", "extensions", "upper2toml")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"),
		[]byte("run: [\"tr\", \"a-z\", \"A-Z\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Extra source file (default extras source dir).
	srcDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "commands")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.md"), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(sb.Home, ".gemini", "commands")
	sb.WriteConfig("extras:\n  - name: commands\n    targets:\n      - path: " + targetDir + "\n        extension: upper2toml\n")

	result := sb.RunCLI("sync", "extras")
	if result.ExitCode != 0 {
		t.Fatalf("sync extras failed (exit %d): %s", result.ExitCode, result.Stderr)
	}

	out, err := os.ReadFile(filepath.Join(targetDir, "hello.toml"))
	if err != nil {
		t.Fatalf("expected hello.toml: %v", err)
	}
	if string(out) != "BODY" {
		t.Errorf("output = %q, want BODY", string(out))
	}
}

func TestExtrasExtension_RejectsMergeMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	extDir := filepath.Join(sb.Home, ".config", "skillshare", "extensions", "id")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"),
		[]byte("run: [\"cat\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(sb.Home, ".config", "skillshare", "extras", "commands")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(sb.Home, ".gemini", "commands")
	sb.WriteConfig("extras:\n  - name: commands\n    targets:\n      - path: " + targetDir + "\n        mode: merge\n        extension: id\n")

	result := sb.RunCLI("sync", "extras")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit for extension + merge mode")
	}
}
