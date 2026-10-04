//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestDoctor_UndeclaredSourceLinks(t *testing.T) {
	for _, mode := range []string{"global", "project"} {
		for _, linked := range []bool{false, true} {
			name := mode + "/no_links"
			if linked {
				name = mode + "/linked_directory"
			}
			t.Run(name, func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()

				cwd := sb.Home
				source := sb.SourcePath
				args := []string{"doctor", "-g"}
				if mode == "project" {
					cwd = sb.SetupProjectDir("claude")
					source = filepath.Join(cwd, ".skillshare", "skills")
					sb.CreateProjectSkill(cwd, "real-skill", map[string]string{"SKILL.md": "# Real skill"})
					args = []string{"doctor", "-p"}
				} else {
					sb.CreateSkill("real-skill", map[string]string{"SKILL.md": "# Real skill"})
					sb.WriteConfig("source: " + source + "\ntargets: {}\n")
				}

				if linked {
					external := filepath.Join(sb.Home, "external-skills")
					if err := os.MkdirAll(external, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(external, "SKILL.md"), []byte("# Invisible skill"), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(external, filepath.Join(source, "linked-skills")); err != nil {
						t.Fatal(err)
					}
				}

				result := sb.RunCLIInDir(cwd, args...)
				result.AssertSuccess(t)
				message := "linked-skills: not followed by discovery; its contents are invisible to skillshare"
				if linked {
					result.AssertOutputContains(t, message)
				} else {
					result.AssertOutputNotContains(t, "Source link")
					result.AssertOutputNotContains(t, "invisible to skillshare")
				}

				jsonResult := sb.RunCLIInDir(cwd, append(args, "--json")...)
				jsonResult.AssertSuccess(t)
				out := parseDoctorJSON(t, jsonResult.Stdout)
				count := 0
				for _, check := range out.Checks {
					if check.Name == "source" && !strings.Contains(check.Message, "(1 skills)") {
						t.Errorf("linked skill must remain undiscovered: %+v", check)
					}
					if check.Name != "undeclared_source_links" {
						continue
					}
					count++
					if check.Status != "info" || check.Message != message {
						t.Errorf("unexpected source link check: %+v", check)
					}
				}
				want := 0
				if linked {
					want = 1
				}
				if count != want {
					t.Errorf("got %d source link checks, want %d", count, want)
				}
			})
		}
	}
}
