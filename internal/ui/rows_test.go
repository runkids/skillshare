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
