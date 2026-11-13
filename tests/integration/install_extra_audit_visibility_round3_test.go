//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstallExtra_SuccessfulAuditFindingVisibility(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, force := range []bool{false, true} {
			for _, structured := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/force=%t/json=%t", project, force, structured), func(t *testing.T) {
					sb := testutil.NewSandbox(t)
					defer sb.Cleanup()
					sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
					remote := extraFixture(t, sb)
					repo := strings.TrimPrefix(remote, "file://")
					if err := os.WriteFile(filepath.Join(repo, "rules.md"), []byte("SYNTHETIC_VISIBLE_FINDING\n"), 0644); err != nil {
						t.Fatal(err)
					}
					testutil.RunGit(t, repo, "commit", "-am", "visible finding fixture")
					cwd, mode, configPath := sb.Root, "-g", sb.ConfigPath
					root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
					if project {
						cwd, mode = sb.SetupProjectDir("claude"), "-p"
						configPath = filepath.Join(cwd, ".skillshare", "config.yaml")
						root = filepath.Join(cwd, ".skillshare", "extras")
					}
					severity := "LOW"
					if force {
						severity = "CRITICAL"
					}
					rule := "rules:\n  - id: synthetic-visible\n    severity: " + severity + "\n    pattern: synthetic\n    message: fixture audit visibility\n    regex: 'SYNTHETIC_VISIBLE_FINDING'\n"
					if err := os.WriteFile(filepath.Join(filepath.Dir(configPath), "audit-rules.yaml"), []byte(rule), 0644); err != nil {
						t.Fatal(err)
					}
					args := []string{"install", remote, "--kind", "extra", "--name", "rules", "--threshold", "HIGH", mode}
					if force {
						args = append(args, "--force")
					}
					if structured {
						args = append(args, "--json")
					}
					result := sb.RunCLIInDir(cwd, args...)
					result.AssertSuccess(t)
					text := result.Stdout + result.Stderr
					if structured {
						var out struct {
							AuditOverridden bool     `json:"audit_overridden"`
							Warnings        []string `json:"warnings"`
						}
						if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.AuditOverridden != force {
							t.Fatalf("audit result: %s %v", result.Stdout, err)
						}
						text = strings.Join(out.Warnings, "\n")
					}
					if !strings.Contains(text, "synthetic-visible") || !strings.Contains(text, severity) {
						t.Errorf("successful audit hides finding identity/severity: %s", text)
					}
					if !structured && force && !strings.Contains(strings.ToLower(text), "overrid") {
						t.Errorf("plain success hides explicit audit override: %s", text)
					}
					store, _, err := install.ReadMetadata(root)
					if err != nil || (len(store.AuditAccepted["rules"]) > 0) != force {
						t.Errorf("acceptance policy changed: %+v %v", store, err)
					}
				})
			}
		}
	}
}
