//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/testutil"
)

// collect never writes a target skill named like a declared entry into the
// source: with the link offline that would turn the boundary into a real
// directory and mask the external tree when it returns. Other skills still land.
func TestSkillfollowCollectRefusesDeclaredEntry(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, live := range []bool{false, true} {
			name := map[bool]string{false: "global", true: "project"}[project] + map[bool]string{false: "/missing", true: "/live"}[live]
			t.Run(name, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				source, projectRoot, targetPath := sb.SourcePath, "", ""
				if project {
					projectRoot = sb.SetupProjectDir("claude")
					source = filepath.Join(projectRoot, ".skillshare", "skills")
					targetPath = filepath.Join(projectRoot, ".claude", "skills")
				} else {
					targetPath = sb.CreateTarget("claude")
					sb.WriteConfig("source: " + source + "\ntargets:\n  claude:\n    path: " + targetPath + "\n")
				}
				for _, skill := range []string{"team", "keep"} {
					sb.WriteFile(filepath.Join(targetPath, skill, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill+"\n")
				}
				sb.WriteFile(filepath.Join(source, ".skillfollow"), "team\n")
				external := filepath.Join(sb.Root, "external")
				if live {
					sb.WriteFile(filepath.Join(external, "a", "SKILL.md"), "---\nname: a\n---\n# a\n")
					if err := os.Symlink(external, filepath.Join(source, "team")); err != nil {
						t.Fatal(err)
					}
				}

				var result *testutil.Result
				if project {
					result = sb.RunCLIInDir(projectRoot, "collect", "-p", "--force")
				} else {
					result = sb.RunCLI("collect", "-g", "--force")
				}
				result.AssertSuccess(t)
				result.AssertOutputContains(t, filepath.Join(source, "team")+" is a link; edit its target directly")
				if _, err := os.Stat(filepath.Join(source, "keep", "SKILL.md")); err != nil {
					t.Errorf("skill outside the declaration was not collected: %v\n%s", err, result.Output())
				}
				info, err := os.Lstat(filepath.Join(source, "team"))
				if live {
					if err != nil || info.Mode()&os.ModeSymlink == 0 {
						t.Errorf("the declared link was replaced: %v %v", info, err)
					}
					if _, err := os.Stat(filepath.Join(external, "SKILL.md")); !os.IsNotExist(err) {
						t.Errorf("collect wrote into the external tree: %v", err)
					}
				} else if !os.IsNotExist(err) {
					t.Errorf("collect created the declared entry: %v\n%s", err, result.Output())
				}
			})
		}
	}
}

// collect --dry-run previews what the real run does: a target skill named like
// a declared entry, live or offline, is listed as failed, not as collected.
func TestSkillfollowCollectDryRunRefusesDeclaredEntry(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(map[bool]string{false: "missing", true: "live"}[live]+map[bool]string{false: "/text", true: "/json"}[jsonOutput], func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				targetPath := sb.CreateTarget("claude")
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + targetPath + "\n")
				for _, skill := range []string{"team", "keep"} {
					sb.WriteFile(filepath.Join(targetPath, skill, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill+"\n")
				}
				sb.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), "team\n")
				if live {
					external := filepath.Join(sb.Root, "external")
					sb.WriteFile(filepath.Join(external, "a", "SKILL.md"), "---\nname: a\n---\n# a\n")
					if err := os.Symlink(external, filepath.Join(sb.SourcePath, "team")); err != nil {
						t.Fatal(err)
					}
				}
				refusal := filepath.Join(sb.SourcePath, "team") + " is a link; edit its target directly"

				if jsonOutput {
					result := sb.RunCLI("collect", "-g", "--dry-run", "--json")
					result.AssertSuccess(t)
					var out struct {
						Pulled []string          `json:"pulled"`
						Failed map[string]string `json:"failed"`
						DryRun bool              `json:"dry_run"`
					}
					if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
						t.Fatalf("invalid JSON: %v\n%s", err, result.Stdout)
					}
					if !slices.Equal(out.Pulled, []string{"keep"}) || out.Failed["team"] != refusal || !out.DryRun {
						t.Errorf("pulled = %v, failed = %v, dry_run = %v; want [keep], team refused, true", out.Pulled, out.Failed, out.DryRun)
					}
				} else {
					result := sb.RunCLI("collect", "-g", "--dry-run")
					result.AssertSuccess(t)
					result.AssertOutputContains(t, refusal)
					result.AssertOutputContains(t, "Would collect 1 skill")
				}
				if _, err := os.Stat(filepath.Join(sb.SourcePath, "keep")); !os.IsNotExist(err) {
					t.Errorf("dry run collected keep: %v", err)
				}
			})
		}
	}
}
