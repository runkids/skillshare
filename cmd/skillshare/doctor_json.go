package main

import (
	"fmt"

	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
	versioncheck "skillshare/internal/version"
)

// Check status constants.
const (
	checkPass    = "pass"
	checkWarning = "warning"
	checkError   = "error"
	checkInfo    = "info"
)

// doctorCheck represents a single health check result for JSON output.
type doctorCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // checkPass, checkWarning, checkError
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

type doctorOutput struct {
	Checks  []doctorCheck      `json:"checks"`
	Summary doctorSummary      `json:"summary"`
	Plugins []pluginpkg.Bundle `json:"plugins,omitempty"`
	Hooks   []hookpkg.Bundle   `json:"hooks,omitempty"`
	Version *doctorVersion     `json:"version,omitempty"`
}

type doctorSummary struct {
	Total    int `json:"total"`
	Pass     int `json:"pass"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
	Info     int `json:"info"`
}

type doctorVersion struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
}

// buildDoctorOutput assembles the final JSON from collected checks.
// Counts are derived from the checks slice (not from result.errors/warnings)
// because check-level counts may differ from the text-mode counters when a
// single check function calls addError/addWarning multiple times.
func buildDoctorOutput(result *doctorResult, plugins []pluginpkg.Bundle, hooks []hookpkg.Bundle) doctorOutput {
	var pass, warnings, errors, info int
	for _, c := range result.checks {
		switch c.Status {
		case checkPass:
			pass++
		case checkWarning:
			warnings++
		case checkError:
			errors++
		case checkInfo:
			info++
		}
	}
	return doctorOutput{
		Checks: result.checks,
		Summary: doctorSummary{
			Total:    len(result.checks),
			Pass:     pass,
			Warnings: warnings,
			Errors:   errors,
			Info:     info,
		},
		Plugins: plugins,
		Hooks:   hooks,
	}
}

// finalizeDoctorJSON writes the JSON output and returns an appropriate error.
// Shared by cmdDoctorGlobal and cmdDoctorProject.
func finalizeDoctorJSON(restoreUI func(), result *doctorResult, updateCh <-chan *versioncheck.CheckResult, plugins []pluginpkg.Bundle, hooks []hookpkg.Bundle) error {
	restoreUI()
	output := buildDoctorOutput(result, plugins, hooks)
	updateResult := <-updateCh
	output.Version = &doctorVersion{Current: version}
	if updateResult != nil && updateResult.UpdateAvailable {
		output.Version.Latest = updateResult.LatestVersion
		output.Version.UpdateAvailable = true
	}
	if result.errors > 0 {
		return writeJSONResult(output, fmt.Errorf("doctor found %d error(s)", result.errors))
	}
	return writeJSON(output)
}
