package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLiveEmptySelectionCannotHideIntegrityErrors(t *testing.T) {
	for _, projectsOnly := range []bool{false, true} {
		for _, invalidStore := range []bool{false, true} {
			t.Run(fmt.Sprintf("projects=%v/store=%v", projectsOnly, invalidStore), func(t *testing.T) {
				body := "mcp:\n  targets: []\n  servers: {}\n"
				if projectsOnly {
					body += fmt.Sprintf("  projects:\n    %q:\n      servers:\n        fixture:\n          url: https://fixture.invalid/mcp\n", t.TempDir())
				}
				s := checkService(t, body)
				empty := ""
				opts := CheckOptions{Live: true, Root: &empty, SkipDNS: true, VerifyKeyPath: filepath.Join(t.TempDir(), "missing-verify.pem")}
				if invalidStore {
					opts.VerifyKeyPath = ""
					writeBaseline(t, s, []byte(`{"version":999,"servers":{}}`))
				}
				report, err := s.Check(opts)
				if err == nil || strings.Contains(err.Error(), "missing-verify.pem") {
					t.Fatalf("empty integrity check falsely succeeded or leaked path: report=%+v err=%v", report, err)
				}
			})
		}
	}
}

// Integrity errors must remain errors even when the endpoint would only return
// an authentication warning or could never provide an inventory.
func TestLiveBaselineIntegrityErrorPrecedesUnavailableProbe(t *testing.T) {
	var visits atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		visits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer endpoint.Close()
	s := checkService(t, "mcp:\n  targets: []\n  servers:\n    fixture:\n      url: "+endpoint.URL+"\n")
	writeBaseline(t, s, []byte(`{"version":999,"servers":{}}`))
	report, err := s.Check(CheckOptions{Live: true, SkipDNS: true})
	if err != nil || report.Summary.Errors == 0 || visits.Load() != 0 || report.Servers[0].Contract == nil || report.Servers[0].Contract.Revision != "" {
		t.Fatalf("integrity error masked: report=%+v err=%v visits=%d", report, err, visits.Load())
	}
}
