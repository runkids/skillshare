//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstall_ContainerDirectoryName(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, sourceKind := range []string{"local-skill", "git-collection", "git-root"} {
			for _, customName := range []string{"", "custom-extractor"} {
				mode := "global"
				if project {
					mode = "project"
				}
				t.Run(mode+"/"+sourceKind+"/"+customName, func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					cwd, destRoot := sb.Root, sb.SourcePath
					args := []string{"install"}
					if project {
						cwd = sb.SetupProjectDir("claude")
						destRoot = filepath.Join(cwd, ".skillshare", "skills")
						args = append(args, "-p")
					}

					repo := filepath.Join(sb.Root, "MinerU-Ecosystem")
					skillDir := filepath.Join(repo, "skills")
					if sourceKind == "git-root" {
						repo = skillDir
					}
					sb.WriteFile(filepath.Join(skillDir, "SKILL.md"), "---\nname: MinerU Document Extractor\n---\n# Extractor")
					source := repo
					switch sourceKind {
					case "local-skill":
						source = skillDir
					case "git-collection", "git-root":
						initGitRepo(t, repo)
						source = "file://" + repo
					}
					args = append(args, source, "-y")
					want := "mineru-document-extractor"
					if customName != "" {
						args = append(args, "--name", customName)
						want = customName
					}
					sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
					dest := filepath.Join(destRoot, want)
					if !sb.FileExists(filepath.Join(dest, "SKILL.md")) {
						t.Fatalf("expected installed skill at %s", dest)
					}
					store, err := install.LoadMetadata(destRoot)
					if err != nil {
						t.Fatal(err)
					}
					meta := store.Get(want)
					if meta == nil {
						t.Fatal("missing metadata")
					}
					if sourceKind == "git-collection" && meta.Subdir != "skills" {
						t.Fatalf("metadata subdir = %q, want skills", meta.Subdir)
					}
					if customName == "" && sb.FileExists(filepath.Join(destRoot, "skills", "SKILL.md")) {
						t.Fatal("generic skill name should not be installed")
					}
				})
			}
		}
	}
}

func TestInstall_ContainerDirectoryNameCollision(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, rootSkill := range []bool{false, true} {
			mode := "global"
			if project {
				mode = "project"
			}
			layout := "nested"
			if rootSkill {
				layout = "root"
			}
			t.Run(mode+"/"+layout, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				cwd, destRoot := sb.Root, sb.SourcePath
				args := []string{"install", "--all", "--force", "--into", "extractors"}
				if project {
					cwd = sb.SetupProjectDir("claude")
					destRoot = filepath.Join(cwd, ".skillshare", "skills")
					args = append(args, "-p")
				}
				repo := filepath.Join(sb.Root, "collection")
				skillDir := filepath.Join(repo, "skills")
				if rootSkill {
					repo = skillDir
				}
				genericContent := "---\nname: PDF\n---\n# Generic skill"
				pdfContent := "---\nname: PDF\n---\n# PDF skill"
				sb.WriteFile(filepath.Join(skillDir, "SKILL.md"), genericContent)
				sb.WriteFile(filepath.Join(repo, "pdf", "SKILL.md"), pdfContent)
				initGitRepo(t, repo)
				sb.RunCLIInDir(cwd, append(args, "file://"+repo)...).AssertSuccess(t)
				for name, want := range map[string]string{"skills": genericContent, "pdf": pdfContent} {
					if got := sb.ReadFile(filepath.Join(destRoot, "extractors", name, "SKILL.md")); got != want {
						t.Fatalf("skill %s was overwritten: %q", name, got)
					}
				}
			})
		}
	}
}

func TestInstall_ContainerDirectoryName_TrackedRepoUnchanged(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, fileURL := range []bool{false, true} {
			mode := "global"
			if project {
				mode = "project"
			}
			sourceKind := "local"
			if fileURL {
				sourceKind = "file-url"
			}
			t.Run(mode+"/"+sourceKind, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				cwd, destRoot := sb.Root, sb.SourcePath
				args := []string{"install", "--track"}
				if project {
					cwd = sb.SetupProjectDir("claude")
					destRoot = filepath.Join(cwd, ".skillshare", "skills")
					args = append(args, "-p")
				}
				repo := filepath.Join(sb.Root, "skills")
				sb.WriteFile(filepath.Join(repo, "SKILL.md"), "---\nname: MinerU Document Extractor\n---\n# Extractor")
				initGitRepo(t, repo)
				source := repo
				if fileURL {
					source = "file://" + repo
				}
				sb.RunCLIInDir(cwd, append(args, source)...).AssertSuccess(t)
				if !sb.FileExists(filepath.Join(destRoot, "_skills", "SKILL.md")) {
					t.Fatal("tracked repository should retain its directory name")
				}
			})
		}
	}
}

func TestUpdate_ContainerDirectoryName_PreservesInstalledName(t *testing.T) {
	for _, name := range []string{"skills", "mineru-document-extractor"} {
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			repo := filepath.Join(sb.Root, "MinerU-Ecosystem")
			skillFile := filepath.Join(repo, "skills", "SKILL.md")
			sb.WriteFile(skillFile, "---\nname: MinerU Document Extractor\n---\n# V1")
			initGitRepo(t, repo)
			// Keep GitHub source parsing while cloning an isolated local fixture.
			sb.SetEnv("GIT_CONFIG_COUNT", "2")
			sb.SetEnv("GIT_CONFIG_KEY_0", "url.file://"+filepath.ToSlash(repo)+".insteadOf")
			sb.SetEnv("GIT_CONFIG_VALUE_0", "https://github.com/test/mineru.git")
			sb.SetEnv("GIT_CONFIG_KEY_1", "protocol.file.allow")
			sb.SetEnv("GIT_CONFIG_VALUE_1", "always")
			sb.RunCLI("install", "test/mineru", "--name", name, "-y").AssertSuccess(t)

			sb.WriteFile(skillFile, "---\nname: New Extractor Name\n---\n# V2")
			initGitRepo(t, repo)
			sb.RunCLI("update", "--all", "--force").AssertSuccess(t)
			if got := sb.ReadFile(filepath.Join(sb.SourcePath, name, "SKILL.md")); got != sb.ReadFile(skillFile) {
				t.Fatalf("update did not preserve the destination and refresh its content: %s", got)
			}
			if sb.FileExists(filepath.Join(sb.SourcePath, "new-extractor-name")) {
				t.Fatal("update should not rename an existing install")
			}
		})
	}
}
