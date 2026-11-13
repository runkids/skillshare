//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func extraFixture(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	dir := filepath.Join(sb.Root, "remote")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("fixture rules"), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, dir)
	return "file://" + dir
}

func TestInstallExtra_GlobalAndJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	remote := extraFixture(t, sb)
	result := sb.RunCLI("install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", "--json", "-g")
	result.AssertSuccess(t)
	var got struct {
		Kind, Name, Remote, Commit, Action string
		AuditSkipped                       bool `json:"audit_skipped"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &got); err != nil {
		t.Fatalf("JSON: %v %q", err, result.Stdout)
	}
	if got.Kind != "extra" || got.Name != "rules" || got.Remote != remote || len(got.Commit) != 40 || got.Action != "installed" || !got.AuditSkipped {
		t.Fatalf("result: %+v", got)
	}
	root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
	if data, err := os.ReadFile(filepath.Join(root, "rules", "rules.md")); err != nil || string(data) != "fixture rules" {
		t.Fatalf("installed: %q %v", data, err)
	}
	store, _, err := install.ReadMetadata(root)
	if err != nil || store.Get("rules") == nil || store.Get("rules").Kind != "extra" || store.Get("rules").Commit != got.Commit {
		t.Fatalf("metadata: %+v %v", store, err)
	}
}

func TestInstallExtra_ProjectAndEditRefusal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	remote := extraFixture(t, sb)
	project := filepath.Join(sb.Root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "config.yaml"), []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", "-p"}
	result := sb.RunCLIInDir(project, args...)
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "sync extras")
	lock, err := install.LoadLock(filepath.Join(project, ".skillshare"))
	if err != nil || len(lock.ExtraCommitFor("rules", remote)) != 40 {
		t.Fatalf("pin: %+v %v", lock, err)
	}
	file := filepath.Join(project, ".skillshare", "extras", "rules", "rules.md")
	if err := os.WriteFile(file, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	result = sb.RunCLIInDir(project, append(args, "--json")...)
	result.AssertFailure(t)
	if !strings.Contains(result.Stdout, "--force") {
		t.Fatalf("missing refusal: %s", result.Stdout)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "local edit" {
		t.Fatalf("edit lost: %q %v", data, err)
	}
}

func TestInstallExtra_ProjectMissingAndLocalSourceRefused(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	project := filepath.Join(sb.Root, "empty")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	result := sb.RunCLIInDir(project, "install", "example/repo", "--kind", "extra", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "init -p")
	result = sb.RunCLI("install", sb.Root, "--kind", "extra", "--name", "rules", "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "extras init")
}

func TestInstallExtra_CredentialNeverPersisted(t *testing.T) {
	for _, source := range []string{"https:/example.com/repo?token=SYNTHETIC", "owner/repo#SYNTHETIC", "user:SYNTHETIC@example.com:org/repo"} {
		for _, flags := range [][]string{nil, {"--json"}, {"--dry-run"}} {
			sb := testutil.NewSandbox(t)
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			before, err := os.ReadFile(sb.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string{"install", source, "--kind", "extra", "--name", "rules", "-g"}, flags...)
			result := sb.RunCLI(args...)
			result.AssertFailure(t)
			if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
				t.Fatalf("credential disclosed: %s %s", result.Stdout, result.Stderr)
			}
			after, err := os.ReadFile(sb.ConfigPath)
			if err != nil || string(after) != string(before) {
				t.Fatalf("config changed: %v", err)
			}
			sb.Cleanup()
		}
	}
}

func TestInstallExtra_ParseErrorNeverEchoesSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	result := sb.RunCLI("install", "unrecognized-source-SYNTHETIC", "--kind", "extra", "--json", "-g")
	result.AssertFailure(t)
	if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
		t.Fatalf("source disclosed: %s %s", result.Stdout, result.Stderr)
	}
}
