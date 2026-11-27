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
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "config.yaml"), []byte("targets: []\n"), 0644); err != nil {
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

func TestInstallExtra_RemovalBothModes(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, edited := range []bool{false, true} {
			sb := testutil.NewSandbox(t)
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			remote := extraFixture(t, sb)
			cwd, mode := sb.Root, "-g"
			root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
			if project {
				cwd = sb.SetupProjectDir("claude")
				mode = "-p"
				root = filepath.Join(cwd, ".skillshare", "extras")
			}
			sb.RunCLIInDir(cwd, "install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", mode).AssertSuccess(t)
			source := filepath.Join(root, "rules")
			if edited {
				if err := os.WriteFile(filepath.Join(source, "rules.md"), []byte("local edit"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			result := sb.RunCLIInDir(cwd, "extras", "remove", "rules", "--force", mode)
			result.AssertSuccess(t)
			// Files, metadata, acceptance and pin policy are asserted together in
			// extraops.TestRemove_RemoteUneditedAndEdited for both scope paths.
			if edited {
				result.AssertAnyOutputContains(t, "Kept edited")
			} else {
				result.AssertAnyOutputContains(t, "Deleted fetched copy")
			}
			sb.Cleanup()
		}
	}
}

func TestInstallExtra_LocalConsumers(t *testing.T) {
	for _, project := range []bool{false, true} {
		sb := testutil.NewSandbox(t)
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
		remote := extraFixture(t, sb)
		cwd, mode := sb.Root, "-g"
		root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
		if project {
			cwd = sb.SetupProjectDir("claude")
			mode = "-p"
			root = filepath.Join(cwd, ".skillshare", "extras")
		}
		sb.RunCLIInDir(cwd, "install", remote, "--kind", "extra", "--name", "rules", "--skip-audit", mode).AssertSuccess(t)
		for _, state := range []string{"intact", "modified", "missing", "interrupted", "error"} {
			switch state {
			case "modified":
				if err := os.WriteFile(filepath.Join(root, "rules", "rules.md"), []byte("edited"), 0644); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Rename(filepath.Join(root, "rules"), filepath.Join(root, "saved")); err != nil {
					t.Fatal(err)
				}
			case "interrupted":
				if err := os.MkdirAll(filepath.Join(root, ".skillshare-staging"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".skillshare-staging", "publishing-rules.json"), []byte(`{}`), 0644); err != nil {
					t.Fatal(err)
				}
			case "error":
				if err := os.Remove(filepath.Join(root, ".skillshare-staging", "publishing-rules.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(root, "saved"), filepath.Join(root, "rules")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, install.MetadataFileName), []byte(`{invalid`), 0644); err != nil {
					t.Fatal(err)
				}
			}
			result := sb.RunCLIInDir(cwd, "check", "extras", "--json", mode)
			var rows []struct{ Name, State, Remote, Message string }
			if err := json.Unmarshal([]byte(result.Stdout), &rows); err != nil || len(rows) != 1 || rows[0].State != state || rows[0].Remote != remote || !strings.Contains(rows[0].Message, "remote revision not checked") {
				t.Fatalf("check %s: %s %s %v", state, result.Stdout, result.Stderr, err)
			}
			if strings.Contains(result.Stdout, "up_to_date") || strings.Contains(result.Stdout, "update_available") {
				t.Fatal("local check claimed remote freshness")
			}
			listed := sb.RunCLIInDir(cwd, "extras", "list", "--json", mode)
			listed.AssertSuccess(t)
			var entries []struct {
				Remote, State string
				Targets       []any
			}
			if err := json.Unmarshal([]byte(listed.Stdout), &entries); err != nil || len(entries) != 1 || entries[0].Remote != remote || entries[0].State != state || entries[0].Targets == nil {
				t.Fatalf("list %s: %s %v", state, listed.Stdout, err)
			}
			if state == "interrupted" {
				sb.RunCLIInDir(cwd, "status", mode).AssertAnyOutputContains(t, "interrupted")
				sb.RunCLIInDir(cwd, "doctor", mode).AssertAnyOutputContains(t, "interrupted")
			}
			if state == "modified" {
				sb.RunCLIInDir(cwd, "check", "extras", "--exit-code", mode).AssertFailure(t)
			}
		}
		sb.Cleanup()
	}
}
