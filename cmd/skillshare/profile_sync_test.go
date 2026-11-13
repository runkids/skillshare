package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/profileapply"
)

func TestProfileSync_PreflightJSONWrittenOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("targets: {}\nprofiles: {}\n"), 0600); err != nil { t.Fatal(err) }
	var commandErr error
	output := captureStdout(t, func() {
		commandErr = runProfileSyncCLI(nil, profileapply.Scope{ConfigPath: path}, false, false, false, true, &profileSyncNative{}, func(_ *profileapply.View) error {
			return writeJSONError(errors.New("native settings conflict"))
		})
	})
	var out map[string]string
	if err := json.Unmarshal([]byte(output), &out); err != nil { t.Fatalf("preflight wrote multiple JSON documents: %s %v", output, err) }
	if commandErr == nil || out["error"] != "native settings conflict" { t.Fatal(output, commandErr) }
}
