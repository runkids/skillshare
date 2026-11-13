package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestCheckExtras_OrphanPublicationBeforeDeclaration(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, artifact := range []string{"publishing-rules.json", "incoming-rules", "previous-rules"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, artifact), func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
				mode, cwd := modeGlobal, ""
				root := filepath.Join(base, "extras")
				if project {
					mode, cwd = modeProject, base
					cfg := &config.ProjectConfig{}
					if err := cfg.Save(base); err != nil {
						t.Fatal(err)
					}
					root = cfg.EffectiveExtrasSource(base)
				} else {
					cfg := &config.Config{Source: filepath.Join(base, "skills")}
					if err := cfg.Save(); err != nil {
						t.Fatal(err)
					}
					root = cfg.EffectiveExtrasSource()
				}
				stage := filepath.Join(root, ".skillshare-staging")
				if err := os.MkdirAll(stage, 0755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(stage, artifact)
				if strings.HasSuffix(artifact, ".json") {
					// Inspection must use the owned artifact's filename, never fields
					// (or even valid JSON) in a marker that it has not validated.
					if err := os.WriteFile(path, []byte(`{"name":"SYNTHETIC","remote":"https://user:SYNTHETIC@example.com"`), 0600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
				// This is the shared check/status JSON inspection owner.
				rows := inspectRemoteExtras(nil, root)
				if len(rows) != 1 || rows[0].Name != "rules" || rows[0].State != "interrupted" || rows[0].Remote != "" {
					t.Errorf("undeclared publication not reported safely: %+v", rows)
				}
				for _, names := range [][]string{nil, {"rules"}} {
					args := append([]string{"--json", "--exit-code"}, names...)
					var checkErr error
					out := captureStdout(t, func() { checkErr = cmdCheckExtras(mode, cwd, args) })
					if checkErr == nil || !strings.Contains(out, `"state": "interrupted"`) || !strings.Contains(out, `"name": "rules"`) || strings.Contains(out, "SYNTHETIC") || strings.Contains(out, "remote extra not found") {
						t.Errorf("orphan check output: %s; error: %v", out, checkErr)
					}
				}
			})
		}
	}
}
