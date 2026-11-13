//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstallExtra_EarlyErrorsNeverEchoCredentials(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		credential := "https://user:SYNTHETIC@example.com/repo"
		cases := map[string][]string{
			"positional":    {"example-org/ai-hub", credential},
			"name":          {"example-org/ai-hub", "--name", credential},
			"option":        {"example-org/ai-hub", "--" + credential},
			"kind":          {"example-org/ai-hub", "--kind", credential},
			"threshold":     {"example-org/ai-hub", "--audit-threshold", credential},
			"source-parser": {"unrecognized-source-SYNTHETIC"},
		}
		for _, project := range []bool{false, true} {
			for _, output := range []string{"text", "json", "dry-run"} {
				for name, input := range cases {
					t.Run(fmt.Sprintf("project=%t/%s/%s", project, output, name), func(t *testing.T) {
						sb := testutil.NewSandbox(t)
						defer sb.Cleanup()
						sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
						cwd, mode := sb.Root, "-g"
						if project {
							cwd, mode = sb.SetupProjectDir("claude"), "-p"
						}
						before := projectTree(t, sb.Root)
						// Put --kind after malformed inputs for half the paths: the
						// boundary must recognize extra intent before formatting errors.
						args := append([]string{"install"}, input...)
						args = append(args, "--kind", "extra", mode)
						if name == "kind" {
							args = append([]string{"install", "--kind", "extra"}, input...)
							args = append(args, mode)
						}
						if output == "json" {
							args = append(args, "--json")
						} else if output == "dry-run" {
							args = append(args, "--dry-run")
						}
						result := sb.RunCLIInDir(cwd, args...)
						result.AssertFailure(t)
						if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
							t.Errorf("input disclosed: stdout=%s stderr=%s", result.Stdout, result.Stderr)
						}
						if output == "json" && !json.Valid([]byte(result.Stdout)) {
							t.Errorf("early error stdout is not JSON: %s", result.Stdout)
						}
						if after := projectTree(t, sb.Root); !reflect.DeepEqual(before, after) {
							t.Error("refusal changed fixture files, including logs/config/metadata/pins")
						}
					})
				}
			}
		}
	})
	t.Run("TestInstallExtra_CredentialNeverPersisted", func(t *testing.T) {
		sources := []string{
			"https://user:SYNTHETIC@example.com/repo", "https://example.com/repo?token=SYNTHETIC", "https://example.com/repo#SYNTHETIC",
			"https://example.com/%zz?token=SYNTHETIC", "https://example.com/%zz#SYNTHETIC",
			"ssh://git:SYNTHETIC@example.com/repo", "git://user:SYNTHETIC@example.com/repo",
			"https:/example.com/repo?token=SYNTHETIC", "https:example.com/repo?token=SYNTHETIC",
			`https:\\example.com\repo?token=SYNTHETIC`, "https//example.com/repo?token=SYNTHETIC",
			"example.com/repo?token=SYNTHETIC", "example.com/repo#SYNTHETIC", "owner/repo/sub?token=SYNTHETIC",
			"ado:org/project/repo?token=SYNTHETIC", "user:SYNTHETIC@example.com/org/repo", "user:SYNTHETIC@example.com:org/repo",
			"user@example.com/org/SYNTHETIC", " HTTPS://user:SYNTHETIC@example.com/repo",
			"file://user:SYNTHETIC@localhost/repo", "file://SYNTHETIC@localhost/repo",
			"file://user:SYNTHETIC@/tmp/repo", "file://user:SYNTHETIC@%zz/repo",
			"file://user:SYNTHETIC@localhost/%zz", " FILE://user:SYNTHETIC@localhost/repo ",
		}
		for _, projectMode := range []bool{false, true} {
			for _, source := range sources {
				for _, flags := range [][]string{nil, {"--json"}, {"--dry-run"}, {"--json", "--force"}, {"--json", "--dry-run"}} {
					sb := testutil.NewSandbox(t)
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					cwd, mode := sb.Root, "-g"
					if projectMode {
						cwd = sb.SetupProjectDir("claude")
						mode = "-p"
					}
					before := projectTree(t, sb.Root)
					args := append([]string{"install", source, "--kind", "extra", "--name", "rules", mode}, flags...)
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertFailure(t)
					for _, flag := range flags {
						if flag == "--json" && !json.Valid([]byte(result.Stdout)) {
							t.Errorf("credential refusal stdout is not JSON: %s", result.Stdout)
						}
					}
					if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
						t.Fatalf("credential disclosed: %s %s", result.Stdout, result.Stderr)
					}
					if after := projectTree(t, sb.Root); !reflect.DeepEqual(before, after) {
						t.Fatal("credential refusal changed files, including oplog/config/lock/metadata")
					}
					sb.Cleanup()
				}
			}
		}
	})
	t.Run("TestInstallExtra_ParseErrorNeverEchoesSource", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
		result := sb.RunCLI("install", "unrecognized-source-SYNTHETIC", "--kind", "extra", "--json", "-g")
		result.AssertFailure(t)
		if strings.Contains(result.Stdout+result.Stderr, "SYNTHETIC") {
			t.Fatalf("source disclosed: %s %s", result.Stdout, result.Stderr)
		}
	})
}
