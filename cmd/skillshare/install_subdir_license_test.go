package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestHandleGitSubdirInstall_SingleSkill_ShowsLicense(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot := t.TempDir()
	subdir := filepath.Join(repoRoot, "packs", "licensed")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create test repo: %v", err)
	}
	content := "---\nname: licensed\nlicense: MIT\n---\n# Licensed\n"
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	initGitRepoForTest(t, repoRoot)

	cfg := &config.Config{Source: t.TempDir()}
	source := &install.Source{
		Type:     install.SourceTypeGitHTTPS,
		Raw:      "file://" + repoRoot + "/packs/licensed",
		CloneURL: "file://" + repoRoot,
		Subdir:   "packs/licensed",
		Name:     "licensed",
	}

	output := captureStdoutStderr(t, func() {
		_, err := handleGitSubdirInstall(source, cfg, install.InstallOptions{DryRun: true})
		if err != nil {
			t.Fatalf("handleGitSubdirInstall() error = %v", err)
		}
	})

	if !strings.Contains(output, "License: MIT") {
		t.Fatalf("expected output to include license, got:\n%s", output)
	}
}

func initGitRepoForTest(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, string(out))
	}

	commands := [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	}
	for _, args := range commands {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
		}
	}
}

func captureStdoutStderr(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	var outBuf, errBuf bytes.Buffer
	doneOut := make(chan struct{})
	doneErr := make(chan struct{})

	go func() {
		_, _ = outBuf.ReadFrom(stdoutR)
		close(doneOut)
	}()
	go func() {
		_, _ = errBuf.ReadFrom(stderrR)
		close(doneErr)
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-doneOut
	<-doneErr

	return outBuf.String() + errBuf.String()
}
