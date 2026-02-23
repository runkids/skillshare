//go:build !online

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstall_SubdirFromFileURL(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	repoPath := filepath.Join(sb.Root, "subdir-repo")
	sb.WriteFile(filepath.Join(repoPath, "skills", "alpha", "SKILL.md"), "# alpha")
	sb.WriteFile(filepath.Join(repoPath, "skills", "alpha", "README.md"), "alpha readme")
	sb.WriteFile(filepath.Join(repoPath, "other", "ignored.txt"), "ignore me")
	initGitRepo(t, repoPath)

	source := &install.Source{
		Type:     install.SourceTypeGitHTTPS,
		Raw:      "file://" + repoPath + "/skills/alpha",
		CloneURL: "file://" + repoPath,
		Subdir:   "skills/alpha",
		Name:     "alpha",
	}

	destPath := filepath.Join(sb.SourcePath, "alpha")
	result, err := install.Install(source, destPath, install.InstallOptions{})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.Action != "cloned and extracted" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned and extracted")
	}

	if !sb.FileExists(filepath.Join(destPath, "SKILL.md")) {
		t.Fatal("expected SKILL.md to be installed")
	}
	if sb.FileExists(filepath.Join(destPath, "ignored.txt")) {
		t.Fatal("unexpected non-subdir file copied into destination")
	}
}

func TestInstall_SubdirFallback_FuzzyPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	repoPath := filepath.Join(sb.Root, "fuzzy-subdir-repo")
	sb.WriteFile(filepath.Join(repoPath, "skills", "pdf", "SKILL.md"), "# PDF")
	initGitRepo(t, repoPath)

	source := &install.Source{
		Type:     install.SourceTypeGitHTTPS,
		Raw:      "file://" + repoPath + "/pdf",
		CloneURL: "file://" + repoPath,
		Subdir:   "pdf", // fuzzy match should resolve to skills/pdf on full-clone fallback
		Name:     "pdf",
	}

	destPath := filepath.Join(sb.SourcePath, "pdf")
	result, err := install.Install(source, destPath, install.InstallOptions{})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.Action != "cloned and extracted" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned and extracted")
	}
	if !sb.FileExists(filepath.Join(destPath, "SKILL.md")) {
		t.Fatal("expected fuzzy subdir install to succeed")
	}
}

func TestInstall_NonTTY_NoRawGitProgress(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	repoPath := filepath.Join(sb.Root, "progress-repo")
	sb.WriteFile(filepath.Join(repoPath, "skill-a", "SKILL.md"), "# skill-a")
	initGitRepo(t, repoPath)

	installResult := sb.RunCLI("install", "file://"+repoPath, "--track", "--name", "progress-track")
	installResult.AssertSuccess(t)

	output := installResult.Output()
	rawProgressHints := []string{
		"Receiving objects:",
		"Resolving deltas:",
		"remote:",
	}
	for _, hint := range rawProgressHints {
		if strings.Contains(output, hint) {
			t.Fatalf("expected non-TTY install output to stay quiet; found %q in output", hint)
		}
	}
}
