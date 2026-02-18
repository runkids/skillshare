package audit

import "testing"

func TestResult_HasCritical(t *testing.T) {
	r := &Result{Findings: []Finding{
		{Severity: SeverityMedium},
		{Severity: SeverityHigh},
	}}
	if r.HasCritical() {
		t.Error("should not have critical")
	}

	r.Findings = append(r.Findings, Finding{Severity: SeverityCritical})
	if !r.HasCritical() {
		t.Error("should have critical")
	}
}

func TestResult_HasHigh(t *testing.T) {
	r := &Result{Findings: []Finding{
		{Severity: SeverityMedium},
	}}
	if r.HasHigh() {
		t.Error("should not have high")
	}

	r.Findings = append(r.Findings, Finding{Severity: SeverityHigh})
	if !r.HasHigh() {
		t.Error("should have high")
	}
}

func TestResult_MaxSeverity(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     string
	}{
		{"empty", nil, ""},
		{"medium only", []Finding{{Severity: SeverityMedium}}, SeverityMedium},
		{"high and medium", []Finding{{Severity: SeverityMedium}, {Severity: SeverityHigh}}, SeverityHigh},
		{"all levels", []Finding{{Severity: SeverityMedium}, {Severity: SeverityHigh}, {Severity: SeverityCritical}}, SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{Findings: tt.findings}
			if got := r.MaxSeverity(); got != tt.want {
				t.Errorf("MaxSeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResult_CountBySeverity(t *testing.T) {
	r := &Result{Findings: []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
		{Severity: SeverityMedium},
	}}

	c, h, m := r.CountBySeverity()
	if c != 2 || h != 1 || m != 3 {
		t.Errorf("CountBySeverity() = (%d, %d, %d), want (2, 1, 3)", c, h, m)
	}
}
