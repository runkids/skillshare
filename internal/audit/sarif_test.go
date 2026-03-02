package audit

import (
	"encoding/json"
	"testing"
)

func TestToSARIF_Empty(t *testing.T) {
	t.Parallel()

	log := ToSARIF(nil, SARIFOptions{})

	if log.Schema != sarifSchema {
		t.Fatalf("schema = %q, want %q", log.Schema, sarifSchema)
	}
	if log.Version != sarifVersion {
		t.Fatalf("version = %q, want %q", log.Version, sarifVersion)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("runs count = %d, want 1", len(log.Runs))
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != toolName {
		t.Fatalf("tool name = %q, want %q", run.Tool.Driver.Name, toolName)
	}
	if run.Tool.Driver.InformationURI != toolInfoURI {
		t.Fatalf("informationUri = %q, want %q", run.Tool.Driver.InformationURI, toolInfoURI)
	}
	if len(run.Results) != 0 {
		t.Fatalf("results count = %d, want 0", len(run.Results))
	}
	if len(run.Tool.Driver.Rules) != 0 {
		t.Fatalf("rules count = %d, want 0", len(run.Tool.Driver.Rules))
	}
}

func TestToSARIF_Findings(t *testing.T) {
	t.Parallel()

	results := []*Result{
		{
			SkillName: "skill-a",
			Findings: []Finding{
				{Severity: SeverityHigh, Pattern: "prompt-injection", Message: "Prompt injection detected", File: "SKILL.md", Line: 10, Snippet: "ignore previous"},
				{Severity: SeverityMedium, Pattern: "external-link", Message: "External link found", File: "SKILL.md", Line: 20, Snippet: "[link](http://evil.com)"},
			},
		},
		{
			SkillName: "skill-b",
			Findings: []Finding{
				{Severity: SeverityHigh, Pattern: "prompt-injection", Message: "Prompt injection detected", File: "README.md", Line: 5, Snippet: "forget instructions"},
			},
		},
	}

	log := ToSARIF(results, SARIFOptions{ToolVersion: "1.2.3"})

	run := log.Runs[0]

	// Should have exactly 2 deduplicated rules
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("rules count = %d, want 2", len(run.Tool.Driver.Rules))
	}

	// First rule should be prompt-injection (first occurrence)
	if run.Tool.Driver.Rules[0].ID != "prompt-injection" {
		t.Fatalf("rules[0].id = %q, want %q", run.Tool.Driver.Rules[0].ID, "prompt-injection")
	}
	if run.Tool.Driver.Rules[1].ID != "external-link" {
		t.Fatalf("rules[1].id = %q, want %q", run.Tool.Driver.Rules[1].ID, "external-link")
	}

	// Should have 3 results total
	if len(run.Results) != 3 {
		t.Fatalf("results count = %d, want 3", len(run.Results))
	}

	// Check first result
	r0 := run.Results[0]
	if r0.RuleID != "prompt-injection" {
		t.Fatalf("results[0].ruleId = %q, want %q", r0.RuleID, "prompt-injection")
	}
	if r0.RuleIndex != 0 {
		t.Fatalf("results[0].ruleIndex = %d, want 0", r0.RuleIndex)
	}
	if r0.Level != "error" {
		t.Fatalf("results[0].level = %q, want %q", r0.Level, "error")
	}
	if r0.Message.Text != "Prompt injection detected" {
		t.Fatalf("results[0].message.text = %q, want %q", r0.Message.Text, "Prompt injection detected")
	}
	if len(r0.Locations) != 1 {
		t.Fatalf("results[0].locations count = %d, want 1", len(r0.Locations))
	}
	loc := r0.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "SKILL.md" {
		t.Fatalf("results[0].locations[0].uri = %q, want %q", loc.ArtifactLocation.URI, "SKILL.md")
	}
	if loc.Region.StartLine != 10 {
		t.Fatalf("results[0].locations[0].startLine = %d, want 10", loc.Region.StartLine)
	}

	// Check third result (from skill-b) references ruleIndex 0 (same pattern)
	r2 := run.Results[2]
	if r2.RuleIndex != 0 {
		t.Fatalf("results[2].ruleIndex = %d, want 0", r2.RuleIndex)
	}

	// Verify tool version is set
	if run.Tool.Driver.Version != "1.2.3" {
		t.Fatalf("tool version = %q, want %q", run.Tool.Driver.Version, "1.2.3")
	}
}

func TestToSARIF_SeverityMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity  string
		wantLevel string
		wantSecSv float64
	}{
		{SeverityCritical, "error", 9.0},
		{SeverityHigh, "error", 7.0},
		{SeverityMedium, "warning", 4.0},
		{SeverityLow, "note", 2.0},
		{SeverityInfo, "note", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			results := []*Result{{
				SkillName: "test-skill",
				Findings: []Finding{
					{Severity: tt.severity, Pattern: "test-" + tt.severity, Message: "test", File: "f.md", Line: 1},
				},
			}}

			log := ToSARIF(results, SARIFOptions{})
			run := log.Runs[0]

			if len(run.Tool.Driver.Rules) != 1 {
				t.Fatalf("rules count = %d, want 1", len(run.Tool.Driver.Rules))
			}
			rule := run.Tool.Driver.Rules[0]
			if rule.DefaultConfig.Level != tt.wantLevel {
				t.Errorf("rule level = %q, want %q", rule.DefaultConfig.Level, tt.wantLevel)
			}
			if rule.Properties.SecuritySeverity != tt.wantSecSv {
				t.Errorf("rule security-severity = %v, want %v", rule.Properties.SecuritySeverity, tt.wantSecSv)
			}

			if len(run.Results) != 1 {
				t.Fatalf("results count = %d, want 1", len(run.Results))
			}
			if run.Results[0].Level != tt.wantLevel {
				t.Errorf("result level = %q, want %q", run.Results[0].Level, tt.wantLevel)
			}
		})
	}
}

func TestToSARIF_ValidJSON(t *testing.T) {
	t.Parallel()

	results := []*Result{
		{
			SkillName: "skill-x",
			Findings: []Finding{
				{Severity: SeverityCritical, Pattern: "shell-execution", Message: "shell exec", File: "run.sh", Line: 3, Snippet: "exec bash"},
				{Severity: SeverityInfo, Pattern: "external-link", Message: "link", File: "", Line: 0},
			},
		},
	}

	log := ToSARIF(results, SARIFOptions{ToolVersion: "0.1.0"})

	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Round-trip: unmarshal back into a generic map to verify valid JSON
	var roundtrip map[string]interface{}
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal round-trip: %v", err)
	}

	// Verify top-level keys exist
	if _, ok := roundtrip["$schema"]; !ok {
		t.Fatal("missing $schema in JSON")
	}
	if _, ok := roundtrip["version"]; !ok {
		t.Fatal("missing version in JSON")
	}
	if _, ok := roundtrip["runs"]; !ok {
		t.Fatal("missing runs in JSON")
	}

	// Verify the finding with no File has no locations
	runs := roundtrip["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	resultsArr := run["results"].([]interface{})
	// Second result (Info finding with no File) should have empty locations
	r1 := resultsArr[1].(map[string]interface{})
	locs := r1["locations"].([]interface{})
	if len(locs) != 0 {
		t.Fatalf("expected 0 locations for finding without file, got %d", len(locs))
	}
}
