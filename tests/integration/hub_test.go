//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
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
	result.AssertAnyOutputContains(t, "with 2 skills")

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
	result.AssertAnyOutputContains(t, "with 0 skills")

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

func TestHub_IndexHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("hub", "index", "--help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "--full")
	result.AssertAnyOutputContains(t, "--source")
	result.AssertAnyOutputContains(t, "--output")
}

func TestNestedMetadata_SyncListHubAudit(t *testing.T) {
	const content = `---
metadata:
  name: shadow
  description: Official tool
  type: Skill
  status: stable
  generated:
    by: human:alex
    at: "2026-11-13T00:00:00Z"
  sources:
    - id: agentskills
      resource: https://agentskills.io/specification
  unknown:
    flags: [true, false]
name: fixture
description: Safe fixture
last_edited: 2026-11-13
---
# Fixture
Read the supplied fixture.
`
	for _, mode := range []string{"global", "project"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, target := sb.SourcePath, sb.CreateTarget("claude")
			run := func(args ...string) *testutil.Result { return sb.RunCLI(append(args, "-g")...) }
			if mode == "project" {
				root := sb.SetupProjectDir("claude")
				source = filepath.Join(root, ".skillshare", "skills")
				target = filepath.Join(root, ".claude", "skills")
				sb.CreateProjectSkill(root, "fixture", map[string]string{"SKILL.md": content})
				run = func(args ...string) *testutil.Result { return sb.RunCLIInDir(root, append(args, "-p")...) }
			} else {
				sb.CreateSkill("fixture", map[string]string{"SKILL.md": content})
				sb.WriteConfig("source: " + source + "\nmode: copy\ntargets:\n  claude:\n    path: " + target + "\n")
			}
			writeListMeta(t, source, "fixture", &install.MetadataEntry{Source: "fixture-owner/skills/fixture", Type: "github-subdir", RepoURL: "https://github.com/fixture-owner/skills.git", Subdir: "fixture"})
			run("sync").AssertSuccess(t)
			data, err := os.ReadFile(filepath.Join(target, "fixture", "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != content {
				t.Error("sync changed SKILL.md bytes")
			}
			list := run("list", "--json")
			list.AssertSuccess(t)
			var skills []struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(list.Stdout), &skills); err != nil {
				t.Fatal(err)
			}
			if len(skills) != 1 || skills[0].Name != "fixture" {
				t.Fatalf("list skills = %v", skills)
			}
			run("hub", "index", "--full").AssertSuccess(t)
			data, err = os.ReadFile(filepath.Join(source, "skillshare-hub.json"))
			if err != nil {
				t.Fatal(err)
			}
			var idx hubIndex
			if err := json.Unmarshal(data, &idx); err != nil {
				t.Fatal(err)
			}
			if len(idx.Skills) != 1 {
				t.Fatalf("hub skills = %v", idx.Skills)
			}
			entry := idx.Skills[0]
			if entry.Name != "fixture" || entry.Description != "Safe fixture" || entry.Type != "github-subdir" {
				t.Errorf("hub identity = %+v", entry)
			}
			audit := run("audit", "fixture", "--format", "json")
			audit.AssertSuccess(t)
			payload := parseAuditJSONPayload(t, audit)
			if payload.Summary.Scanned != 1 || payload.Summary.Failed != 0 || payload.Summary.ScanErrors != 0 {
				t.Errorf("audit summary = %+v", payload.Summary)
			}
		})
	}
}
