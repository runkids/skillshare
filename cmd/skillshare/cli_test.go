package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"skillshare/internal/theme"
)

func TestCLISearchOutput(t *testing.T) {
	for _, tt := range []struct {
		name           string
		args           []string
		query, command string
		count          int
	}{
		{"default", []string{"search", "sync", "--json"}, "sync", "skillshare sync", 1},
		{"words and limit", []string{"search", "link", "skills", "--limit=1", "-j"}, "link skills", "skillshare link", 1},
		{"delimiter", []string{"search", "--json", "--", "--json"}, "--json", "", 0},
		{"empty match", []string{"search", "qzxqzx", "--json"}, "qzxqzx", "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := cmdCLI(tt.args); err != nil {
					t.Fatal(err)
				}
			})
			var response cliSearchResponse
			if err := json.Unmarshal([]byte(out), &response); err != nil {
				t.Fatal(err, out)
			}
			if response.Query != tt.query || response.Results == nil || len(response.Results) != tt.count {
				t.Fatalf("unexpected response: %+v", response)
			}
			if tt.command == "" {
				if len(response.Results) != 0 {
					t.Fatal(response.Results)
				}
			} else if len(response.Results) == 0 || response.Results[0].Command != tt.command {
				t.Fatal(response.Results)
			}
			if strings.Contains(out, `"score"`) {
				t.Fatal("scores are not public API")
			}
		})
	}
	for _, args := range [][]string{{"search", "sync"}, {"search", "--", "--json"}} {
		out := captureStdout(t, func() {
			if err := cmdCLI(args); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "Run 'skillshare") || strings.HasPrefix(out, "{") {
			t.Fatal(out)
		}
	}
}

func TestCLIUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"search"}, {"search", "  "}, {"search", "sync", "--limit", "0"},
		{"search", "sync", "--limit=21"}, {"search", "sync", "--limit=bad"},
		{"search", "sync", "--limit"}, {"search", "sync", "--unknown"}, {"unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var got error
			jsonArgs := append(append([]string{}, args...), "--json")
			missingLimit := args[len(args)-1] == "--limit"
			if missingLimit {
				jsonArgs = append([]string{args[0], "--json"}, args[1:]...)
			}
			out := captureStdout(t, func() { got = cmdCLI(jsonArgs) })
			var silent *jsonSilentError
			var usage *cliUsageError
			if !errors.As(got, &silent) || !errors.As(got, &usage) {
				t.Fatalf("error = %T: %v", got, got)
			}
			var envelope struct {
				Error cliUsageError `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &envelope); err != nil {
				t.Fatal(err, out)
			}
			if envelope.Error.Code != "usage" || envelope.Error.Message == "" || !strings.Contains(envelope.Error.Hint, "--help") {
				t.Fatal(out)
			}
			if missingLimit && envelope.Error.Message != "--limit requires a number between 1 and 20" {
				t.Fatal(out)
			}
		})
	}
}

func TestCLICommandCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range cliCommandCatalog() {
		name := strings.TrimPrefix(entry.Command, "skillshare ")
		if commands[name] == nil || entry.Summary == "" || entry.Usage != entry.Command+" --help" || seen[name] {
			t.Fatalf("invalid entry: %+v", entry)
		}
		seen[name] = true
	}
	for name := range commands {
		switch name {
		case "cli", "__ui-restart", "completion", "ui", "tui":
			continue
		}
		if !seen[name] {
			t.Errorf("public command %q missing from discovery", name)
		}
	}
	for _, name := range []string{"cli", "__ui-restart", "completion", "ui", "tui", "version"} {
		if seen[name] {
			t.Errorf("excluded command %q is indexed", name)
		}
	}
}

// A fresh test process also catches package initializers that resolve the theme
// before main can take its offline-discovery branch.
func TestCLIDiscoveryDefersTheme(t *testing.T) {
	const helper = "SKILLSHARE_TEST_CLI_THEME"
	if os.Getenv(helper) == "1" {
		captureStdout(t, func() {
			if err := cmdCLI([]string{"search", "sync", "--json"}); err != nil {
				t.Fatal(err)
			}
		})
		t.Setenv("SKILLSHARE_THEME", "light")
		if got := theme.Get().Mode; got != theme.ModeLight {
			t.Fatalf("theme was resolved before discovery finished: %s", got)
		}
		return
	}
	t.Setenv(helper, "1")
	t.Setenv("NO_COLOR", "")
	t.Setenv("SKILLSHARE_THEME", "dark")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-test.run=^TestCLIDiscoveryDefersTheme$").CombinedOutput(); err != nil {
		t.Fatalf("theme helper: %v\n%s", err, output)
	}
}
