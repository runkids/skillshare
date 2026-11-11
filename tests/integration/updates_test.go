//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func TestUpdates_GlobalPreferenceAndDoctor(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	t.Setenv(config.AutoUpdateCheckEnv, "")
	if err := os.WriteFile(sb.ConfigPath, []byte("source: "+sb.SourcePath+"\nmode: copy\ntargets: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(sb.Root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "config.yaml"), []byte("targets: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Even from a project, updates changes the user's global preference.
	result := sb.RunCLIInDir(project, "updates", "off", "--json")
	result.AssertSuccess(t)
	var policy config.UpdatePolicy
	if err := json.Unmarshal([]byte(result.Stdout), &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Enabled || policy.Configured {
		t.Fatalf("policy = %+v", policy)
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "copy" || cfg.Updates.AutoCheck == nil || *cfg.Updates.AutoCheck {
		t.Fatalf("saved preference lost: %+v", cfg)
	}
	for _, mode := range []string{"-g", "-p"} {
		result = sb.RunCLIInDir(project, "doctor", mode, "--json")
		var out struct {
			Version struct {
				Latest          string
				UpdateAvailable bool `json:"update_available"`
			}
		}
		if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
			t.Fatalf("doctor JSON: %s / %v", result.Stdout, err)
		}
		if out.Version.UpdateAvailable || out.Version.Latest != "" {
			t.Fatalf("doctor leaked update while disabled: %s", result.Stdout)
		}
	}
	result = sb.RunCLIEnv(map[string]string{config.AutoUpdateCheckEnv: "true"}, "updates", "--json")
	result.AssertSuccess(t)
	if err := json.Unmarshal([]byte(result.Stdout), &policy); err != nil {
		t.Fatal(err)
	}
	if !policy.Enabled || !policy.EnvOverride || policy.Configured {
		t.Fatalf("override = %+v", policy)
	}
	result = sb.RunCLI("updates", "on", "--json")
	result.AssertSuccess(t)
	if err := json.Unmarshal([]byte(result.Stdout), &policy); err != nil {
		t.Fatal(err)
	}
	if !policy.Enabled || !policy.Configured {
		t.Fatalf("re-enabled = %+v", policy)
	}
}

func TestUpdates_RejectsUnexpectedArguments(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, args := range [][]string{{"updates", "off", "on"}, {"updates", "--project"}, {"updates", "unknown"}} {
		if result := sb.RunCLI(args...); result.ExitCode == 0 {
			t.Fatalf("%v should fail", args)
		}
	}
}
