package plugin

import (
	"path/filepath"
	"testing"
)

func TestOpenCodeImportFileURLPreservesDrive(t *testing.T) {
	for _, id := range []string{"file://C:/plugins/my%20plugin/index.js", "file:///C:/plugins/my%20plugin/index.js"} {
		want := filepath.FromSlash("C:/plugins/my plugin/index.js")
		if got := openCodeImportPath(id); got != want {
			t.Errorf("%s decoded to %q, want %q", id, got, want)
		}
	}
}
