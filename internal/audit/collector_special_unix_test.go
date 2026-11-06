//go:build unix

package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A blocked FIFO read must fail a regression without stranding a reader in
// the test process. Each child owns only fixtures under its parent's temp dir.
func TestCollectorSpecialFiles(t *testing.T) {
	const marker = "SKILLSHARE_TEST_COLLECTOR_SPECIAL"
	if fixture := os.Getenv(marker); fixture != "" {
		testCollectorSpecialFixture(t, fixture)
		return
	}
	for _, fixture := range []string{
		"direct", "direct-link", "walk", "walk-link", "walk-nontext",
		"pinned", "pinned-link", "metadata", "metadata-link",
		"metadata-pinned", "metadata-pinned-link",
	} {
		t.Run(fixture, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			if err := os.Mkdir(home, 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCollectorSpecialFiles$", "-test.count=1")
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				switch key {
				case "HOME", "TMPDIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
					"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "SKILLSHARE_CONFIG", marker:
					continue
				}
				cmd.Env = append(cmd.Env, entry)
			}
			cmd.Env = append(cmd.Env, marker+"="+fixture, "HOME="+home, "TMPDIR="+root,
				"XDG_CONFIG_HOME="+filepath.Join(home, "config"), "XDG_DATA_HOME="+filepath.Join(home, "data"),
				"XDG_STATE_HOME="+filepath.Join(home, "state"), "XDG_CACHE_HOME="+filepath.Join(home, "cache"))
			output, err := cmd.CombinedOutput() // CommandContext kills and reaps a blocked child.
			if ctx.Err() != nil {
				t.Fatalf("audit tried to read a stable special file; child deadline exceeded: %s", output)
			}
			if err != nil {
				t.Fatalf("fixture failed: %v\n%s", err, output)
			}
		})
	}
}

func testCollectorSpecialFixture(t *testing.T, fixture string) {
	t.Helper()
	root := t.TempDir()
	skillDir := filepath.Join(root, "skill")
	if err := os.Mkdir(skillDir, 0700); err != nil {
		t.Fatal(err)
	}
	const content = "# Ordinary instructions\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	name := "notes.md"
	metadata := strings.HasPrefix(fixture, "metadata")
	pinned := strings.Contains(fixture, "pinned")
	if metadata {
		name = "SKILL.md"
	} else if pinned || fixture == "walk-nontext" {
		// Exercise the skip count for a nontext FIFO.
		name = "payload.asset"
	}
	path := filepath.Join(skillDir, name)
	hashes := map[string]string{"SKILL.md": "sha256:" + sha256hex([]byte(content))}
	if pinned {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		hashes[name] = "sha256:" + sha256hex([]byte(content))
	}
	if !metadata || pinned {
		writeMetaWithHashes(t, skillDir, hashes)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	target := path
	if strings.HasSuffix(fixture, "-link") {
		target = filepath.Join(root, "target.fifo")
	}
	if err := syscall.Mkfifo(target, 0600); err != nil {
		t.Fatal(err)
	}
	if target != path {
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	if strings.HasPrefix(fixture, "direct") {
		if result, err := ScanFile(path); err == nil || result != nil {
			t.Fatalf("direct special file must be rejected: result=%+v err=%v", result, err)
		}
		return
	}
	result, err := ScanSkill(skillDir) // Include default integrity and metadata analyzers.
	if err != nil {
		t.Fatal(err)
	}
	if result.SkippedFiles != 1 {
		t.Errorf("special file counted %d skips, want 1", result.SkippedFiles)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := info.Size()
	if !metadata {
		wantBytes += int64(len(content))
	}
	if result.TotalBytes != wantBytes {
		t.Errorf("logical entry accounting changed: got %d bytes, want %d", result.TotalBytes, wantBytes)
	}
	if metadata && !pinned {
		return
	}
	wantPattern, wantSeverity := "content-unexpected", SeverityLow
	if pinned {
		wantPattern, wantSeverity = "content-tampered", SeverityMedium
	}
	for _, finding := range result.Findings {
		if finding.File == name && finding.Pattern == wantPattern && finding.Severity == wantSeverity {
			return
		}
	}
	t.Fatalf("missing %s evidence for %s: %+v", wantPattern, name, result.Findings)
}
