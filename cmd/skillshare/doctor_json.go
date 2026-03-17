package main

// doctorCheck represents a single health check result for JSON output.
type doctorCheck struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // "pass", "warning", "error"
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

type doctorOutput struct {
	Checks  []doctorCheck  `json:"checks"`
	Summary doctorSummary  `json:"summary"`
	Version *doctorVersion `json:"version,omitempty"`
}

type doctorSummary struct {
	Total    int `json:"total"`
	Pass     int `json:"pass"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type doctorVersion struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
}

func buildDoctorOutput(result *doctorResult) doctorOutput {
	var pass, warnings, errors int
	for _, c := range result.checks {
		switch c.Status {
		case "pass":
			pass++
		case "warning":
			warnings++
		case "error":
			errors++
		}
	}
	return doctorOutput{
		Checks: result.checks,
		Summary: doctorSummary{
			Total:    len(result.checks),
			Pass:     pass,
			Warnings: warnings,
			Errors:   errors,
		},
	}
}
