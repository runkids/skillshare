//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// check validates skill-level targets with the same snapshot it scans with,
// so a followed skill naming an unknown target is warned about.
func TestSkillfollowCheckWarnsUnknownTargetInFollowedSkill(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, projectRoot := sb.SourcePath, ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
			}
			external := filepath.Join(sb.Root, "external")
			sb.WriteFile(filepath.Join(external, "safe", "SKILL.md"), "---\nname: safe\ndescription: Safe skill\nmetadata:\n  targets:\n    - nonexistent-tool\n---\n# Safe\n")
			testutil.RunGit(t, external, "init")
			if err := os.Symlink(external, filepath.Join(source, "_repo")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "_repo\n")
			var result *testutil.Result
			if project {
				result = sb.RunCLIInDir(projectRoot, "check", "-p")
			} else {
				result = sb.RunCLI("check", "-g")
			}
			result.AssertSuccess(t)
			result.AssertAnyOutputContains(t, `_repo/safe: unknown target "nonexistent-tool"`)
		})
	}
}
