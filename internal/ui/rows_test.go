package ui

import (
	"strings"
	"testing"
	"time"
)

func TestTook_OmitsWhatWouldReadZero(t *testing.T) {
	if got := Took(49 * time.Millisecond); got != "" {
		t.Fatalf("Took(49ms) = %q, want empty", got)
	}
}

func TestTook_ShowsTenthsOfASecond(t *testing.T) {
	if got := Took(1240 * time.Millisecond); !strings.Contains(got, "· 1.2s") {
		t.Fatalf("Took(1.24s) = %q, want it to contain %q", got, "· 1.2s")
	}
}

func TestVersionLabel_PrefixesReleases(t *testing.T) {
	if got := VersionLabel("0.23.5"); got != "v0.23.5" {
		t.Fatalf("VersionLabel(0.23.5) = %q, want v0.23.5", got)
	}
}

func TestVersionLabel_LeavesBuildNamesBare(t *testing.T) {
	if got := VersionLabel("dev"); got != "dev" {
		t.Fatalf("VersionLabel(dev) = %q, want dev", got)
	}
}
