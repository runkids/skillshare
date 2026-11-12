package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRelayDoctorChecksSendsChecksAndKeepsResult(t *testing.T) {
	out := `{"check":{"name":"source","status":"pass","message":"Source: ok"}}
{"check":{"name":"targets","status":"warning","message":"Targets: 1 issue"}}
{
  "checks": [],
  "summary": {"total": 2}
}
`
	var events []string
	final, err := relayDoctorChecks(strings.NewReader(out), func(event string, data any) {
		b, _ := json.Marshal(data)
		events = append(events, event+" "+string(b))
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		`check {"name":"source","status":"pass","message":"Source: ok"}`,
		`check {"name":"targets","status":"warning","message":"Targets: 1 issue"}`,
	}
	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %q, want %q", events, want)
	}
	var result struct {
		Summary struct{ Total int } `json:"summary"`
	}
	if err := json.Unmarshal(final, &result); err != nil || result.Summary.Total != 2 {
		t.Fatalf("final result = %q (err %v)", final, err)
	}
}
