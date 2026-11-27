//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHooksOpenCodeV2FileInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic POSIX version probe")
	}
	sb := newHooksSandbox(t)
	defer sb.Cleanup()
	bin := filepath.Join(sb.Home, "bin")
	probe := filepath.Join(bin, "opencode")
	sb.WriteFile(probe, "#!/bin/sh\n[ \"$#\" = 1 ] && [ \"$1\" = --version ] || exit 91\nprintf 'opencode v2.3.0\\n'\n")
	if err := os.Chmod(probe, 0755); err != nil {
		t.Fatal(err)
	}
	sb.SetEnv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	sb.SetEnv("XDG_CONFIG_HOME", filepath.Join(sb.Home, ".config"))
	for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG_CONTENT"} {
		sb.SetEnv(key, "")
	}
	input := filepath.Join(sb.Root, "entry.json")
	sb.WriteFile(input, `{"bindings":{"opencode":{"code":"export default {}\n"}}}`)
	dir := filepath.Join(sb.Home, ".config/opencode/plugins/skillshare-native")
	preview := sb.RunCLI("hooks", "add", "native", "--file", input, "--dry-run", "--json", "-g")
	preview.AssertSuccess(t)
	if !strings.Contains(preview.Stdout, "package.json") || strings.Contains(preview.Stdout, "skillshare-native.ts") {
		t.Fatal(preview.Stdout)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote package", err)
	}
}
