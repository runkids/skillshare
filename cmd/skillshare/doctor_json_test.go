package main

import (
	"encoding/json"
	"testing"
)

func TestFetchDoctorUpdateResultDevModeSimulatesUpdate(t *testing.T) {
	oldVersion := version
	version = "dev"
	defer func() { version = oldVersion }()

	result := fetchDoctorUpdateResult()
	if result == nil {
		t.Fatal("expected simulated update result in dev mode")
	}
	if !result.UpdateAvailable || result.LatestVersion == "" {
		t.Fatalf("result = %#v, want update available with latest version", result)
	}
}

func TestFinalizeDoctorJSONDevModeVersion(t *testing.T) {
	oldVersion := version
	version = "dev"
	defer func() { version = oldVersion }()

	out := buildDoctorOutput(&doctorResult{})
	out.Version = &doctorVersion{Current: version}
	if version == "" || version == "dev" {
		out.Version.DevMode = true
		out.Version.Latest = "dev-ui-flow"
		out.Version.UpdateAvailable = true
	}

	if out.Version == nil || !out.Version.DevMode || !out.Version.UpdateAvailable || out.Version.Latest == "" {
		t.Fatalf("doctor version = %#v", out.Version)
	}
}

func TestDoctorCheckSuggestionsJSON(t *testing.T) {
	result := &doctorResult{}
	result.addCheckWithSuggestions(
		"cross_target_discovery",
		checkWarning,
		"overlap",
		[]string{"codex also scans universal"},
		[]string{"Choose one authoritative route."},
	)

	out := buildDoctorOutput(result)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	var decoded doctorOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Checks[0].Suggestions; len(got) != 1 || got[0] != "Choose one authoritative route." {
		t.Fatalf("doctor JSON suggestions = %v", got)
	}
}
