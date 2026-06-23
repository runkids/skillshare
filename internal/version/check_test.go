package version

import (
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want bool
	}{
		{name: "older major", v1: "1.0.0", v2: "2.0.0", want: true},
		{name: "older patch", v1: "1.0.0", v2: "1.0.1", want: true},
		{name: "equal", v1: "1.0.0", v2: "1.0.0", want: false},
		{name: "newer", v1: "1.0.1", v2: "1.0.0", want: false},
		{name: "missing patch treated as zero", v1: "1.0", v2: "1.0.1", want: true},
		{name: "current version v prefix", v1: "v1.0.0", v2: "1.0.1", want: true},
		{name: "latest version v prefix", v1: "1.0.0", v2: "v1.0.1", want: true},
		{name: "dev build skipped", v1: "dev", v2: "9.9.9", want: false},
		{name: "empty build skipped", v1: "", v2: "9.9.9", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareVersions(tt.v1, tt.v2)
			if err != nil {
				t.Fatalf("compareVersions(%q, %q) returned error: %v", tt.v1, tt.v2, err)
			}
			if got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %v, want %v", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestCompareVersionsRejectsNonNumericSegments(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want string
	}{
		{name: "current version segment", v1: "1.0.abc", v2: "1.0.0", want: `non-numeric version segment "abc" in "1.0.abc"`},
		{name: "latest version segment", v1: "1.0.0", v2: "1.0.beta", want: `non-numeric version segment "beta" in "1.0.beta"`},
		{name: "pre-release segment", v1: "1.2.3-beta", v2: "1.2.3", want: `non-numeric version segment "3-beta" in "1.2.3-beta"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compareVersions(tt.v1, tt.v2)
			if err == nil {
				t.Fatalf("compareVersions(%q, %q) returned nil error", tt.v1, tt.v2)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("compareVersions(%q, %q) error = %q, want to contain %q", tt.v1, tt.v2, err, tt.want)
			}
		})
	}
}

func TestCheckRejectsMalformedCurrentVersionBeforeCacheHandling(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if err := saveCache(&Cache{
		LastChecked:   time.Now().Add(checkInterval),
		LatestVersion: "9.9.9",
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	if result := Check("1.0.abc", InstallDirect); result != nil {
		t.Fatalf("Check returned %#v, want nil", result)
	}

	if got := GetCachedVersion(); got != "9.9.9" {
		t.Fatalf("cached version = %q, want %q", got, "9.9.9")
	}
}

func TestCheckAcceptsCurrentVersionWithVPrefix(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if err := saveCache(&Cache{
		LastChecked:   time.Now().Add(checkInterval),
		LatestVersion: "1.0.1",
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	result := Check("v1.0.0", InstallDirect)
	if result == nil {
		t.Fatal("Check returned nil, want update result")
	}
	if !result.UpdateAvailable {
		t.Fatalf("UpdateAvailable = false, want true")
	}
	if result.CurrentVersion != "v1.0.0" || result.LatestVersion != "1.0.1" {
		t.Fatalf("result = %#v, want current v1.0.0 latest 1.0.1", result)
	}
}
