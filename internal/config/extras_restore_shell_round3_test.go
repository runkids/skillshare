package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Execute only a fixture function named skillshare; no product or network call.
func TestExtraRestoreCommand_ActualShellArguments(t *testing.T) {
	// The optional seam remains compilable before production supports it;
	// baseline falls through to the current helper and still executes both shells.
	commandForShell := func(extra ExtraConfig, project, powershell bool) string {
		if command, ok := any(ExtraRestoreCommand).(func(ExtraConfig, bool, ...bool) string); ok {
			return command(extra, project, powershell)
		}
		return ExtraRestoreCommand(extra, project)
	}
	for _, shell := range []string{"posix", "powershell"} {
		for _, branch := range []string{"main", "release/o'brien", "release/a'$b`c\"d"} {
			for _, project := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/project=%t", shell, branch, project), func(t *testing.T) {
					extra := ExtraConfig{Name: "rules", Remote: "example-org/hub", Branch: branch}
					command := commandForShell(extra, project, shell == "powershell")
					mode := "-g"
					if project {
						mode = "-p"
					}
					want := []string{"install", extra.Remote, "--kind", "extra", "--name", extra.Name, "--branch", branch, mode}
					var cmd *exec.Cmd
					if shell == "posix" {
						if runtime.GOOS == "windows" {
							t.Skip("POSIX shell control")
						}
						cmd = exec.Command("sh", "-c", "skillshare() { printf '%s\\0' \"$@\"; }\n"+command)
					} else {
						pwsh := os.Getenv("SKILLSHARE_TEST_PWSH")
						if pwsh == "" {
							t.Skip("set SKILLSHARE_TEST_PWSH to exercise actual isolated PowerShell grammar")
						}
						cmd = exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", "function skillshare { [Console]::Out.Write((ConvertTo-Json -Compress -InputObject @($args))) }\n"+command)
					}
					home := t.TempDir()
					cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+home, "XDG_CACHE_HOME="+home, "POWERSHELL_TELEMETRY_OPTOUT=1")
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("generated command fails %s grammar: %v\n%s", shell, err, out)
					}
					var got []string
					if shell == "posix" {
						got = strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
					} else if err := json.Unmarshal(out, &got); err != nil {
						t.Fatalf("fixture argument JSON: %v %q", err, out)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s changed restore arguments: got=%q want=%q", shell, got, want)
					}
				})
			}
		}
	}
}
