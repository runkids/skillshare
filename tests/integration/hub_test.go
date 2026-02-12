//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// hubIndex is a minimal struct for parsing hub index output.
type hubIndex struct {
	SchemaVersion int `json:"schemaVersion"`
	Skills        []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
		FlatName    string `json:"flatName,omitempty"`
		Type        string `json:"type,omitempty"`
	} `json:"skills"`
}

func TestHub_IndexGeneratesJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{
		"SKILL.md": "---\nname: alpha\ndescription: First skill\n---\n# Alpha",
	})
	sb.CreateSkill("beta", map[string]string{
		"SKILL.md": "---\nname: beta\ndescription: Second skill\n---\n# Beta",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "index")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Found 2 skill(s)")

	// Verify generated file
	data, err := os.ReadFile(filepath.Join(sb.SourcePath, "skillshare-hub.json"))
	if err != nil {
		t.Fatalf("read skillshare-hub.json: %v", err)
	}

	var idx hubIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse skillshare-hub.json: %v", err)
	}
	if idx.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", idx.SchemaVersion)
	}
	if len(idx.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(idx.Skills))
	}
	// Verify deterministic sort
	if idx.Skills[0].Name != "alpha" || idx.Skills[1].Name != "beta" {
		t.Errorf("skills not sorted: %s, %s", idx.Skills[0].Name, idx.Skills[1].Name)
	}
}

func TestHub_IndexCustomOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	outPath := filepath.Join(sb.Home, "custom-index.json")
	result := sb.RunCLI("hub", "index", "--output", outPath)
	result.AssertSuccess(t)

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output at %s: %v", outPath, err)
	}
}

func TestHub_IndexEmptySource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "index")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Found 0 skill(s)")

	data, err := os.ReadFile(filepath.Join(sb.SourcePath, "skillshare-hub.json"))
	if err != nil {
		t.Fatalf("read skillshare-hub.json: %v", err)
	}

	var idx hubIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(idx.Skills) != 0 {
		t.Errorf("got %d skills, want 0", len(idx.Skills))
	}
}

func TestHub_IndexFull(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("test-skill", map[string]string{
		"SKILL.md": "---\nname: test-skill\ndescription: A test\n---\n# Test",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "index", "--full")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "full (metadata included)")
}

func TestHub_IndexMinimalOmitsMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ndescription: A skill\n---\n# Content",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("hub", "index")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "minimal")

	data, err := os.ReadFile(filepath.Join(sb.SourcePath, "skillshare-hub.json"))
	if err != nil {
		t.Fatalf("read skillshare-hub.json: %v", err)
	}

	// Parse as raw JSON to check field presence
	var raw struct {
		Skills []map[string]any `json:"skills"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(raw.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(raw.Skills))
	}

	// Minimal output should not contain metadata fields
	for _, key := range []string{"flatName", "relPath", "type", "repoUrl", "version", "installedAt", "isInRepo"} {
		if _, ok := raw.Skills[0][key]; ok {
			t.Errorf("minimal mode should not contain %q", key)
		}
	}
}

func TestHub_Help(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("hub", "--help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "index")
}

func TestHub_IndexHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("hub", "index", "--help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "--full")
	result.AssertAnyOutputContains(t, "--source")
	result.AssertAnyOutputContains(t, "--output")
}
