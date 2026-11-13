package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_RejectsInvalidThresholdBeforeFetch(t *testing.T) {
	for _, skip := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "extras")
		source := &Source{Type: SourceTypeGitHTTPS, Raw: "file:///does-not-exist", CloneURL: "file:///does-not-exist"}
		_, err := InstallRemoteExtra(RemoteExtraRequest{Name: "rules", RawSource: source.Raw, Source: source, ExtrasRoot: root,
			Opts: InstallOptions{SkipAudit: skip, AuditThreshold: "not-a-level"}})
		if err == nil || !strings.Contains(err.Error(), "threshold") {
			t.Fatalf("skip=%t got %v", skip, err)
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("invalid threshold created administrative root: %v", err)
		}
	}
}
