//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSyncProject_VisibleDirectoryAfterRename(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	root := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(root, "demo", map[string]string{"SKILL.md": "# Demo"})
	sb.RunCLIInDir(root, "sync", "-p").AssertSuccess(t)

	link := filepath.Join(root, ".claude", "skills", "demo", "SKILL.md")
	if _, err := os.ReadFile(link); err != nil {
		t.Fatalf("initial target is unreadable: %v", err)
	}
	if err := os.Rename(filepath.Join(root, ".skillshare"), filepath.Join(root, "skillshare")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(link); !os.IsNotExist(err) {
		t.Fatalf("expected old target link to break after rename, got %v", err)
	}

	// Auto-detection must find the visible config and sync must repair old links.
	sb.RunCLIInDir(root, "sync").AssertSuccess(t)
	body, err := os.ReadFile(link)
	if err != nil || string(body) != "# Demo" {
		t.Fatalf("target was not repaired: body=%q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".skillshare")); !os.IsNotExist(err) {
		t.Fatalf("sync recreated the hidden project directory: %v", err)
	}
}
