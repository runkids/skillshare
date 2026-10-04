//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skillshare/internal/testutil"
)

func TestSkillfollowSyncUnreadableDeclarationPreservesTargets(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	for _, file := range []string{".skillfollow", ".skillfollow.local"} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(file+"/"+mode, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				target := sb.CreateTarget("claude")
				sb.WriteConfig("source: " + sb.SourcePath + "\nmode: " + mode + "\ntargets:\n  claude:\n    path: " + target + "\n")
				external := filepath.Join(sb.Root, "external")
				sb.WriteFile(filepath.Join(external, "a", "SKILL.md"), "# Preserved\n")
				if err := os.Symlink(external, filepath.Join(sb.SourcePath, "group")); err != nil {
					t.Fatal(err)
				}
				declaration := filepath.Join(sb.SourcePath, file)
				sb.WriteFile(declaration, "group\n")
				sb.RunCLI("sync", "-g").AssertSuccess(t)
				managed := filepath.Join(target, "group__a")
				if _, err := os.Lstat(managed); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(declaration, 0000); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(declaration, 0600)
				result := sb.RunCLI("sync", "--force", "-g")
				if _, err := os.Lstat(managed); err != nil {
					t.Fatalf("managed target removed after declaration read failure: %v", err)
				}
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, file)
				result.AssertAnyOutputContains(t, "permission denied")
				if data, err := os.ReadFile(filepath.Join(managed, "SKILL.md")); err != nil || string(data) != "# Preserved\n" {
					t.Fatalf("managed content changed: %q, %v", data, err)
				}
			})
		}
	}
}
