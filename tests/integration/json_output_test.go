package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// extractJSON finds the first complete JSON object in a string that may
// contain non-JSON text (e.g. dry-run messages from internal sync).
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

// --- sync --json ---

func TestSync_JSON_OutputsValidJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	result := sb.RunCLI("sync", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	// Verify expected fields
	for _, field := range []string{"targets", "linked", "local", "updated", "pruned", "dry_run", "duration", "details"} {
		if _, ok := output[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}

	// Verify details is an array (not null)
	details, ok := output["details"].([]any)
	if !ok {
		t.Fatalf("details should be an array, got %T", output["details"])
	}
	if len(details) != 1 {
		t.Errorf("expected 1 target detail, got %d", len(details))
	}
}

func TestSync_JSON_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	result := sb.RunCLI("sync", "--json", "--dry-run")
	result.AssertSuccess(t)

	// Dry-run may include "[dry-run] Would create link..." from internal sync
	// before the JSON object, so we extract the JSON portion
	jsonStr := extractJSON(result.Stdout)
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %q", err, result.Stdout)
	}
	if output["dry_run"] != true {
		t.Error("dry_run should be true")
	}
}

func TestSync_JSON_NilSlicesAreEmptyArrays(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// No skills, no targets → details should be [] not null
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("sync", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %q", err, result.Stdout)
	}

	details, ok := output["details"].([]any)
	if !ok {
		t.Fatalf("details should be an array, got %T (value: %v)", output["details"], output["details"])
	}
	if len(details) != 0 {
		t.Errorf("expected 0 details, got %d", len(details))
	}
}

// --- uninstall --json ---

func TestUninstall_JSON_OutputsValidJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("to-remove", map[string]string{"SKILL.md": "# Remove me"})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "to-remove", "--json")
	result.AssertSuccess(t)

	// uninstall --json still prints UI output before JSON; extract JSON
	jsonStr := extractJSON(result.Stdout)
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	// Verify fields
	for _, field := range []string{"removed", "failed", "skipped", "dry_run", "duration"} {
		if _, ok := output[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}

	removed, ok := output["removed"].([]any)
	if !ok {
		t.Fatalf("removed should be an array, got %T", output["removed"])
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(removed))
	}
	if removed[0] != "to-remove" {
		t.Errorf("expected removed[0] = 'to-remove', got %v", removed[0])
	}
}

func TestUninstall_JSON_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "keep-me", "--json", "--dry-run")
	result.AssertSuccess(t)

	jsonStr := extractJSON(result.Stdout)
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %q", err, result.Stdout)
	}
	if output["dry_run"] != true {
		t.Error("dry_run should be true")
	}
}

func TestUninstall_JSON_ImpliesForce(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("auto-force", map[string]string{"SKILL.md": "# Auto"})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// --json should skip confirmation (implies --force)
	result := sb.RunCLI("uninstall", "auto-force", "--json")
	result.AssertSuccess(t)

	jsonStr := extractJSON(result.Stdout)
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	removed, ok := output["removed"].([]any)
	if !ok || len(removed) != 1 {
		t.Errorf("expected 1 removed skill, got %v", output["removed"])
	}
}

// --- collect --json ---

// --- target list --json ---

func TestTarget_List_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	result := sb.RunCLI("target", "list", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	targets, ok := output["targets"].([]any)
	if !ok {
		t.Fatalf("targets should be an array, got %T", output["targets"])
	}
	if len(targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(targets))
	}
}

// --- status --json ---

func TestStatus_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	result := sb.RunCLI("status", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	for _, field := range []string{"source", "skill_count", "tracked_repos", "targets", "audit", "version"} {
		if _, ok := output[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}
}

// --- diff --json ---

func TestDiff_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	result := sb.RunCLI("diff", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	for _, field := range []string{"targets", "duration"} {
		if _, ok := output[field]; !ok {
			t.Errorf("missing field %q in JSON output", field)
		}
	}

	targets, ok := output["targets"].([]any)
	if !ok {
		t.Fatalf("targets should be an array, got %T", output["targets"])
	}
	if len(targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(targets))
	}
}

// --- collect --json ---

func TestCollect_JSON_NoLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("existing", map[string]string{"SKILL.md": "# Existing"})
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + claudePath + "\n")

	// Sync first so the skill is a symlink (not local)
	sb.RunCLI("sync")

	result := sb.RunCLI("collect", "--json", "--all")
	result.AssertSuccess(t)

	jsonStr := extractJSON(result.Stdout)
	var output map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	// Should have empty arrays for pulled and skipped
	if _, ok := output["pulled"]; !ok {
		t.Error("missing 'pulled' field")
	}
	if _, ok := output["dry_run"]; !ok {
		t.Error("missing 'dry_run' field")
	}
}
