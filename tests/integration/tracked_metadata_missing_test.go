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

const missingTrackedRepoName = "_team-skills"

func setupMissingTrackedRepoMetadata(t *testing.T, sb *testutil.Sandbox) {
	t.Helper()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	store.Set(missingTrackedRepoName, &install.MetadataEntry{
		Source:  "https://github.com/acme/team-skills.git",
		Tracked: true,
		Branch:  "main",
	})
	if err := store.Save(sb.SourcePath); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	gitignore := "# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\n" + missingTrackedRepoName + "/\n# END SKILLSHARE MANAGED\n"
	if err := os.WriteFile(filepath.Join(sb.SourcePath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
}

func TestTrackedMetadataMissingClone_StatusJSONReportsMissingRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupMissingTrackedRepoMetadata(t, sb)

	result := sb.RunCLI("status", "--json")
	result.AssertSuccess(t)

	var output struct {
		TrackedRepos []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"tracked_repos"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, result.Stdout)
	}

	if len(output.TrackedRepos) != 1 {
		t.Fatalf("expected one tracked repo from metadata, got %d: %+v", len(output.TrackedRepos), output.TrackedRepos)
	}
	if output.TrackedRepos[0].Name != missingTrackedRepoName {
		t.Fatalf("expected repo %q, got %q", missingTrackedRepoName, output.TrackedRepos[0].Name)
	}
	if output.TrackedRepos[0].Status != "missing" {
		t.Fatalf("expected missing tracked repo status, got %q", output.TrackedRepos[0].Status)
	}
	message := strings.ToLower(output.TrackedRepos[0].Message)
	if !strings.Contains(message, "missing") || !strings.Contains(output.TrackedRepos[0].Message, "skillshare install") {
		t.Fatalf("expected actionable missing-repo message with skillshare install suggestion, got %q", output.TrackedRepos[0].Message)
	}
}

func TestTrackedMetadataMissingClone_CheckJSONReportsMissingRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupMissingTrackedRepoMetadata(t, sb)

	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output struct {
		TrackedRepos []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"tracked_repos"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, result.Stdout)
	}

	if len(output.TrackedRepos) != 1 {
		t.Fatalf("expected one tracked repo from metadata, got %d: %+v", len(output.TrackedRepos), output.TrackedRepos)
	}
	if output.TrackedRepos[0].Name != missingTrackedRepoName {
		t.Fatalf("expected repo %q, got %q", missingTrackedRepoName, output.TrackedRepos[0].Name)
	}
	if output.TrackedRepos[0].Status != "missing" {
		t.Fatalf("expected missing tracked repo status, got %q", output.TrackedRepos[0].Status)
	}
	message := strings.ToLower(output.TrackedRepos[0].Message)
	if !strings.Contains(message, "missing") || !strings.Contains(output.TrackedRepos[0].Message, "skillshare install") {
		t.Fatalf("expected actionable missing-repo message with skillshare install suggestion, got %q", output.TrackedRepos[0].Message)
	}
}

func TestTrackedMetadataMissingClone_UpdateAllJSONSkipsMissingRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupMissingTrackedRepoMetadata(t, sb)

	result := sb.RunCLI("update", "--all", "--json", "--skip-audit")
	result.AssertSuccess(t)

	var output struct {
		Skipped int `json:"skipped"`
		Items   []struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"items"`
		MissingTrackedRepos *struct {
			Names []string `json:"names"`
			Hint  string   `json:"hint"`
		} `json:"missing_tracked_repos"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, result.Stdout)
	}

	if output.Skipped != 1 {
		t.Fatalf("expected missing tracked repo to count as skipped=1, got %d; output=%+v", output.Skipped, output)
	}
	if len(output.Items) != 1 {
		t.Fatalf("expected one update item for missing tracked repo, got %d: %+v", len(output.Items), output.Items)
	}
	item := output.Items[0]
	if item.Name != missingTrackedRepoName || item.Type != "repo" || item.Status != "skipped" {
		t.Fatalf("expected skipped repo item for %q, got %+v", missingTrackedRepoName, item)
	}
	// Per-item error is concise; the actionable hint is aggregated in the summary.
	if !strings.Contains(strings.ToLower(item.Error), "absent") {
		t.Fatalf("expected concise per-item error, got %q", item.Error)
	}
	if output.MissingTrackedRepos == nil {
		t.Fatalf("expected missing_tracked_repos summary in output: %s", result.Stdout)
	}
	if len(output.MissingTrackedRepos.Names) != 1 || output.MissingTrackedRepos.Names[0] != missingTrackedRepoName {
		t.Fatalf("expected summary names [%q], got %v", missingTrackedRepoName, output.MissingTrackedRepos.Names)
	}
	if !strings.Contains(output.MissingTrackedRepos.Hint, "skillshare install") {
		t.Fatalf("expected summary hint with skillshare install suggestion, got %q", output.MissingTrackedRepos.Hint)
	}
}

func TestTrackedMetadataMissingClone_DoctorJSONWarnsWithInstallSuggestion(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupMissingTrackedRepoMetadata(t, sb)

	result := sb.RunCLI("doctor", "--json")
	result.AssertSuccess(t)

	var output struct {
		Checks []struct {
			Name        string   `json:"name"`
			Status      string   `json:"status"`
			Message     string   `json:"message"`
			Details     []string `json:"details"`
			Suggestions []string `json:"suggestions"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, result.Stdout)
	}

	for _, check := range output.Checks {
		if check.Name != "tracked_repos" {
			continue
		}
		if check.Status != "warning" {
			t.Fatalf("expected tracked_repos warning, got %q", check.Status)
		}
		if !strings.Contains(check.Message, "tracked") || !strings.Contains(strings.ToLower(check.Message), "missing") {
			t.Fatalf("expected missing tracked repo message, got %q", check.Message)
		}
		if len(check.Details) == 0 || !strings.Contains(check.Details[0], missingTrackedRepoName) {
			t.Fatalf("expected missing repo detail for %q, got %v", missingTrackedRepoName, check.Details)
		}
		if len(check.Suggestions) == 0 || !strings.Contains(check.Suggestions[0], "skillshare install") {
			t.Fatalf("expected skillshare install suggestion, got %v", check.Suggestions)
		}
		return
	}

	t.Fatalf("expected tracked_repos doctor check in output: %+v", output.Checks)
}
