//go:build integration

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_ContainerDirectoryName_SourceUpdateCollision(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			cwd, destRoot := sb.Root, sb.SourcePath
			args := []string{"install", "--all"}
			if project {
				cwd = sb.SetupProjectDir("claude")
				destRoot = filepath.Join(cwd, ".skillshare", "skills")
				args = append(args, "-p")
			}
			repo := filepath.Join(sb.Root, "collection")
			original := "---\nname: pdf\n---\n# Original"
			sb.WriteFile(filepath.Join(repo, "skills", "SKILL.md"), original)
			initGitRepo(t, repo)
			args = append(args, "file://"+repo)
			sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
			sb.WriteFile(filepath.Join(repo, "pdf", "SKILL.md"), "# Unrelated skill")
			initGitRepo(t, repo)
			result := sb.RunCLIInDir(cwd, append(args, "--update")...)
			if result.ExitCode == 0 || !strings.Contains(result.Stdout+result.Stderr, "collision") {
				t.Fatalf("expected update collision error, got %d: %s%s", result.ExitCode, result.Stdout, result.Stderr)
			}
			if got := sb.ReadFile(filepath.Join(destRoot, "pdf", "SKILL.md")); got != original {
				t.Fatalf("collision overwrote the installed skill: %q", got)
			}
		})
	}
}

func TestInstall_ContainerDirectoryName_SourceUpdate(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, sourceKind := range []string{"local", "git-root", "git-subdir", "git-collection"} {
			for _, installedName := range []string{"skills", "original-extractor"} {
				mode := "global"
				if project {
					mode = "project"
				}
				t.Run(mode+"/"+sourceKind+"/"+installedName, func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					cwd, destRoot := sb.Root, sb.SourcePath
					args := []string{"install", "--into", "extractors"}
					if project {
						cwd = sb.SetupProjectDir("claude")
						destRoot = filepath.Join(cwd, ".skillshare", "skills")
						args = append(args, "-p")
					}
					repo := filepath.Join(sb.Root, "collection")
					skillDir := filepath.Join(repo, "skills")
					source := skillDir
					skillFile := filepath.Join(skillDir, "SKILL.md")
					sb.WriteFile(skillFile, "---\nname: Original Extractor\n---\n# First")
					if sourceKind != "local" {
						if sourceKind == "git-root" {
							repo = skillDir
						}
						if sourceKind == "git-collection" {
							sb.WriteFile(filepath.Join(repo, "other", "SKILL.md"), "---\nname: other\n---\n# Other")
							args = append(args, "--all")
						}
						initGitRepo(t, repo)
						source = "file://" + repo
						if sourceKind == "git-subdir" {
							source += "//skills"
						}
					}
					first := append(append([]string{}, args...), source)
					if installedName == "skills" {
						// Installing without frontmatter reproduces the legacy default.
						sb.WriteFile(skillFile, "# First")
						if sourceKind != "local" {
							initGitRepo(t, repo)
						}
					}
					sb.RunCLIInDir(cwd, first...).AssertSuccess(t)
					updated := "---\nname: New Extractor\n---\n# Second"
					sb.WriteFile(skillFile, updated)
					if sourceKind != "local" {
						initGitRepo(t, repo)
					}
					sb.RunCLIInDir(cwd, append(first, "--update")...).AssertSuccess(t)
					if got := sb.ReadFile(filepath.Join(destRoot, "extractors", installedName, "SKILL.md")); got != updated {
						t.Fatalf("source-based update did not preserve %s: %q", installedName, got)
					}
					if sb.FileExists(filepath.Join(destRoot, "extractors", "new-extractor")) {
						t.Fatal("source-based update created a duplicate installation")
					}
				})
			}
		}
	}
}
