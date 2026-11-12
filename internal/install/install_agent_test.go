package install

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
)

func agentDiscoveryFixture(t *testing.T, content string) (*DiscoveryResult, AgentInfo) {
	t.Helper()
	repo := t.TempDir()
	agent := AgentInfo{Name: "reviewer", Path: "agents/team/reviewer.md", FileName: "reviewer.md"}
	path := filepath.Join(repo, filepath.FromSlash(agent.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return &DiscoveryResult{
		RepoPath: repo,
		Source:   &Source{Type: SourceTypeLocalPath, Raw: repo, Path: repo},
	}, agent
}

func TestInstallAgentFromDiscovery_TrackedDestinationEscape(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := initTestRepo(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repo, "escape")); err != nil {
		t.Fatal(err)
	}
	agentPath := "agents/_trap/escape/CLAUDE.md"
	path := filepath.Join(repo, filepath.FromSlash(agentPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Benign agent\nReview changes carefully.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add agent and destination link")
	source, err := ParseSource(fileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "agents-source")
	if _, err := InstallTrackedRepo(source, root, InstallOptions{
		Name: "trap", Kind: "agent", Track: true, SourceDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "_trap", "escape")
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("tracked fixture did not plant destination link: %v, %v", info, err)
	}
	discovery, err := DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)
	var target *AgentInfo
	for i := range discovery.Agents {
		if discovery.Agents[i].Path == agentPath {
			target = &discovery.Agents[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("fixture agent not discovered: %+v", discovery.Agents)
	}
	_, err = InstallAgentFromDiscovery(discovery, *target, root, InstallOptions{Kind: "agent", SourceDir: root})
	if !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("expected destination link refusal, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("agent escaped the destination root: %v", err)
	}
}

func TestDiscoveredAgent_RefusesDestinationLinks(t *testing.T) {
	for _, operation := range []string{"install", "force", "update"} {
		for _, component := range []string{"intermediate", "final", "dangling final"} {
			t.Run(operation+"/"+component, func(t *testing.T) {
				discovery, agent := agentDiscoveryFixture(t, "# Reviewer\nReview changes carefully.\n")
				root, outside := t.TempDir(), t.TempDir()
				external := filepath.Join(outside, "reviewer.md")
				if component != "dangling final" {
					if err := os.WriteFile(external, []byte("external original"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				link := filepath.Join(root, "team")
				target := outside
				if component != "intermediate" {
					if err := os.MkdirAll(link, 0o755); err != nil {
						t.Fatal(err)
					}
					link = filepath.Join(link, agent.FileName)
					target = external
				}
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				opts := InstallOptions{Kind: "agent", SourceDir: root, Force: operation == "force"}
				var err error
				if operation == "update" {
					_, err = UpdateAgentFromDiscovery(discovery, agent, root, opts)
				} else {
					_, err = InstallAgentFromDiscovery(discovery, agent, root, opts)
				}
				if !errors.Is(err, sourcefs.ErrLink) {
					t.Fatalf("expected destination link refusal, got %v", err)
				}
				if component == "dangling final" {
					if _, err := os.Lstat(external); !os.IsNotExist(err) {
						t.Fatalf("external file created: %v", err)
					}
				} else if data, err := os.ReadFile(external); err != nil || string(data) != "external original" {
					t.Fatalf("external file changed: %q, %v", data, err)
				}
				if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("destination link replaced: %v, %v", info, err)
				}
				if entries, err := os.ReadDir(outside); err != nil || len(entries) > 1 {
					t.Fatalf("external directory changed: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestDiscoveredAgent_BlockedAuditPreservesInstalledContent(t *testing.T) {
	for _, update := range []bool{false, true} {
		name := "force install"
		if update {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			discovery, agent := agentDiscoveryFixture(t, "# Reviewer\nReview changes carefully.\n")
			root := t.TempDir()
			opts := InstallOptions{SourceDir: root, AuditThreshold: audit.SeverityHigh}
			if _, err := InstallAgentFromDiscovery(discovery, agent, root, opts); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(root, "team", agent.FileName)
			before, err := os.ReadFile(filepath.Join(root, MetadataFileName))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(resolveDiscoveredAgentSourcePath(discovery, agent), []byte("# Reviewer\nsudo apt-get install -y jq\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			opts.Force = true
			if update {
				_, err = UpdateAgentFromDiscovery(discovery, agent, root, opts)
			} else {
				_, err = InstallAgentFromDiscovery(discovery, agent, root, opts)
			}
			if !errors.Is(err, audit.ErrBlocked) {
				t.Fatalf("expected audit rejection, got %v", err)
			}
			if data, err := os.ReadFile(dest); err != nil || string(data) != "# Reviewer\nReview changes carefully.\n" {
				t.Fatalf("installed agent changed on audit block: %q, %v", data, err)
			}
			if data, err := os.ReadFile(filepath.Join(root, MetadataFileName)); err != nil || string(data) != string(before) {
				t.Fatalf("metadata changed on audit block: %q, %v", data, err)
			}
		})
	}
}

func TestDiscoveredAgent_InstallUpdateMetadataAndDryRun(t *testing.T) {
	discovery, agent := agentDiscoveryFixture(t, "# Reviewer\nReview changes carefully.\n")
	root := filepath.Join(t.TempDir(), "agents")
	destDir := filepath.Join(root, "group", "nested")
	// Legacy callers omit SourceDir; Into belongs to the source root, while
	// the discovered agent's own nested path must remain part of its key.
	opts := InstallOptions{Into: "group/nested"}
	result, err := InstallAgentFromDiscovery(discovery, agent, destDir, InstallOptions{Into: opts.Into, DryRun: true})
	if err != nil || result.Action != "would install" {
		t.Fatalf("dry run install: %+v, %v", result, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("dry run created source directory: %v", err)
	}
	result, err = InstallAgentFromDiscovery(discovery, agent, destDir, opts)
	if err != nil || result.Action != "installed" || result.AuditSkipped || result.AuditThreshold == "" || len(result.Warnings) != 0 {
		t.Fatalf("install: %+v, %v", result, err)
	}
	dest := filepath.Join(destDir, "team", agent.FileName)
	if result.SkillPath != dest {
		t.Fatalf("destination = %q, want %q", result.SkillPath, dest)
	}
	key := "group/nested/team/reviewer"
	store, err := LoadMetadata(root)
	if err != nil || store.Get(key) == nil || store.Get(key).EffectiveKind() != "agent" {
		t.Fatalf("agent metadata missing at %q: %+v, %v", key, store, err)
	}
	if err := os.WriteFile(resolveDiscoveredAgentSourcePath(discovery, agent), []byte("# Updated reviewer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = InstallAgentFromDiscovery(discovery, agent, destDir, opts)
	if err != nil || result.Action != "skipped" {
		t.Fatalf("existing install must skip: %+v, %v", result, err)
	}
	result, err = UpdateAgentFromDiscovery(discovery, agent, destDir, InstallOptions{Into: opts.Into, DryRun: true})
	if err != nil || result.Action != "would reinstall from source" {
		t.Fatalf("dry run update: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(dest); err != nil || string(data) != "# Reviewer\nReview changes carefully.\n" {
		t.Fatalf("skip/dry run changed installed agent: %q, %v", data, err)
	}
	result, err = UpdateAgentFromDiscovery(discovery, agent, destDir, opts)
	if err != nil || result.Action != "updated" || len(result.Warnings) != 0 {
		t.Fatalf("update: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(dest); err != nil || string(data) != "# Updated reviewer\n" {
		t.Fatalf("updated content: %q, %v", data, err)
	}
	opts.Force, opts.SkipAudit = true, true
	result, err = InstallAgentFromDiscovery(discovery, agent, destDir, opts)
	if err != nil || result.Action != "installed" || !result.AuditSkipped {
		t.Fatalf("forced install with skipped audit: %+v, %v", result, err)
	}
	store, err = LoadMetadata(root)
	hash, hashErr := computeSingleFileHash(dest)
	if err != nil || hashErr != nil || store.Get(key) == nil || store.Get(key).FileHashes[agent.FileName] != hash {
		t.Fatalf("updated metadata not rooted at %q: %+v, %v, %v", key, store, err, hashErr)
	}
}

func TestDiscoveredAgent_FollowPolicyAndLinkedSourceRoot(t *testing.T) {
	for _, linkedRoot := range []bool{false, true} {
		name := "followed first-level link"
		if linkedRoot {
			name = "linked source root"
		}
		t.Run(name, func(t *testing.T) {
			discovery, agent := agentDiscoveryFixture(t, "# Reviewer\n")
			root, external := t.TempDir(), t.TempDir()
			opts := InstallOptions{SourceDir: root}
			if linkedRoot {
				root = filepath.Join(root, "agents")
				if err := os.Symlink(external, root); err != nil {
					t.Fatal(err)
				}
				opts.SourceDir = root
			} else {
				if err := os.Symlink(external, filepath.Join(root, "team")); err != nil {
					t.Fatal(err)
				}
				opts.SourceFollow = sourcewalk.NewFollow(root, nil)
			}
			if _, err := InstallAgentFromDiscovery(discovery, agent, root, opts); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(resolveDiscoveredAgentSourcePath(discovery, agent), []byte("# Updated reviewer\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := UpdateAgentFromDiscovery(discovery, agent, root, opts); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(external, agent.FileName)
			if linkedRoot {
				dest = filepath.Join(external, "team", agent.FileName)
			}
			if data, err := os.ReadFile(dest); err != nil || string(data) != "# Updated reviewer\n" {
				t.Fatalf("supported link target not updated: %q, %v", data, err)
			}
		})
	}
}
