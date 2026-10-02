package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdate_LegacyRepoRootOrchestratorMigratesToSkillOnly(t *testing.T) {
	repo := initTestRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: OfficeCLI\n---\n# OfficeCLI"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("repo readme"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "main.py"), []byte("print('officecli')"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "skills", "pptx"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skills", "pptx", "SKILL.md"), []byte("---\nname: pptx\n---\n# PPTX"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add root orchestrator")

	commit, err := getGitCommit(repo)
	if err != nil {
		t.Fatalf("getGitCommit failed: %v", err)
	}

	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "OfficeCLI")
	if err := cloneRepo("file://"+repo, dest, "", true, nil); err != nil {
		t.Fatalf("clone legacy install failed: %v", err)
	}
	if err := WriteMetaToStore(sourceDir, dest, &SkillMeta{
		Source:      "file://" + repo,
		Type:        SourceTypeGitHTTPS.String(),
		InstalledAt: time.Now(),
		RepoURL:     "file://" + repo,
		Version:     "old-commit",
		FileHashes:  map[string]string{"SKILL.md": "sha256:legacy"},
	}); err != nil {
		t.Fatalf("write metadata failed: %v", err)
	}

	source, err := ParseSource("file://" + repo)
	if err != nil {
		t.Fatalf("ParseSource failed: %v", err)
	}
	if _, err := Install(source, dest, InstallOptions{Update: true, SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	for _, rel := range []string{"SKILL.md"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Fatalf("expected %s after migration: %v", rel, err)
		}
	}
	for _, rel := range []string{".git", "README.md", filepath.Join("src", "main.py"), filepath.Join("skills", "pptx", "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(dest, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed during migration, err=%v", rel, err)
		}
	}

	store, err := LoadMetadata(sourceDir)
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}
	entry := store.Get("OfficeCLI")
	if entry == nil {
		t.Fatal("expected migrated metadata entry")
	}
	if entry.Version != commit {
		t.Fatalf("expected metadata version %q, got %q", commit, entry.Version)
	}
	if len(entry.FileHashes) != 1 {
		t.Fatalf("expected SKILL.md-only hashes, got %#v", entry.FileHashes)
	}
	if _, ok := entry.FileHashes["SKILL.md"]; !ok {
		t.Fatalf("expected SKILL.md hash, got %#v", entry.FileHashes)
	}
}

func TestUpdate_LocalCollectionRoot_KeepsSkillFileOnly(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	write := func(rel, body string) {
		p := filepath.Join(repo, rel)
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(body), 0644)
	}
	write("SKILL.md", "---\nname: root\n---\n# v1")
	write("README.md", "readme")
	write("skills/child/SKILL.md", "---\nname: child\n---\n# Child")

	// The dashboard installs the root of a collection as its SKILL.md alone.
	discovery, err := DiscoverLocal(&Source{Type: SourceTypeLocalPath, Raw: repo, Path: repo, Name: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(tmp, "dest")
	destPath := filepath.Join(sourceDir, "root")
	for _, skill := range discovery.Skills {
		if skill.Path == "." {
			if _, err := InstallFromDiscovery(discovery, skill, destPath, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
				t.Fatal(err)
			}
		}
	}

	write("SKILL.md", "---\nname: root\n---\n# v2")
	source, err := ParseSource(LoadMetadataOrNew(sourceDir).GetByPath("root").Source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, destPath, InstallOptions{SourceDir: sourceDir, Update: true, SkipAudit: true}); err != nil {
		t.Fatalf("update: %v", err)
	}

	var files []string
	filepath.Walk(destPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(destPath, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if len(files) != 1 || files[0] != "SKILL.md" {
		t.Errorf("updated files = %v, want only SKILL.md", files)
	}
	if got, _ := os.ReadFile(filepath.Join(destPath, "SKILL.md")); !strings.Contains(string(got), "# v2") {
		t.Errorf("SKILL.md was not updated: %s", got)
	}
}
