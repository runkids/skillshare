package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The copied test executable is a harmless stand-in for OpenCode. No installed
// CLI, npm package, batch file or plugin code runs in these tests.
func TestMain(m *testing.M) {
	if version := os.Getenv("SKILLSHARE_TEST_OPENCODE_VERSION"); version != "" {
		if len(os.Args) != 2 || os.Args[1] != "--version" {
			os.Exit(2)
		}
		fmt.Println(version)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestOpenCodeDefaultProbe(t *testing.T) {
	exe, err := os.Executable()
	must(t, err)
	binary, err := os.ReadFile(exe)
	must(t, err)
	shim, err := os.ReadFile("testdata/opencode-npm.cmd")
	must(t, err)
	for _, tt := range []struct {
		name, pkg, version                string
		local, explicit, invalid, missing bool
	}{
		{name: "native v1", version: "1.18.31"},
		{name: "native v2 account", version: "2.3.0", explicit: true},
		{name: "unknown native version", version: "3.0.0"},
		{name: "npm v1", pkg: "opencode-ai", version: "1.18.31"},
		{name: "npm v2", pkg: "@opencode/cli", version: "2.3.0"},
		{name: "npm local account", pkg: "@opencode/cli", version: "2.3.0", local: true, explicit: true},
		{name: "unknown npm version", pkg: "@opencode/cli", version: "3.0.0"},
		{name: "unrecognized shim", pkg: "opencode-ai", version: "1.18.31", invalid: true},
		{name: "missing native binary", pkg: "opencode-ai", version: "1.18.31", missing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pkg != "" && runtime.GOOS != "windows" {
				t.Skip("npm .cmd lookup is Windows-only")
			}
			root := filepath.Join(t.TempDir(), "npm prefix with spaces")
			must(t, os.MkdirAll(root, 0755))
			binDir := root
			bin := filepath.Join(root, "opencode")
			if runtime.GOOS == "windows" {
				bin += ".exe"
				t.Setenv("PATHEXT", ".EXE;.CMD")
			}
			if tt.pkg != "" {
				target := `node_modules\` + strings.ReplaceAll(tt.pkg, "/", `\`) + `\bin\opencode.exe`
				if tt.local {
					binDir = filepath.Join(root, "node_modules", ".bin")
					target = `..\` + strings.ReplaceAll(tt.pkg, "/", `\`) + `\bin\opencode.exe`
					must(t, os.MkdirAll(binDir, 0755))
				}
				body := strings.ReplaceAll(string(shim), `node_modules\opencode-ai\bin\opencode.exe`, target)
				if tt.invalid {
					body = "@echo 1.18.31\n"
				}
				must(t, os.WriteFile(filepath.Join(binDir, "opencode.cmd"), []byte(strings.ReplaceAll(body, "\n", "\r\n")), 0644))
				bin = filepath.Join(root, "node_modules", filepath.FromSlash(tt.pkg), "bin", "opencode.exe")
				must(t, os.MkdirAll(filepath.Dir(bin), 0755))
			}
			if !tt.missing {
				must(t, os.WriteFile(bin, binary, 0755))
			}
			t.Setenv("PATH", binDir)
			t.Setenv("SKILLSHARE_TEST_OPENCODE_VERSION", tt.version)
			s := &Service{}
			if tt.explicit {
				cli := bin
				if tt.pkg != "" {
					cli = filepath.Join(binDir, "opencode.cmd")
				}
				s.scopedAccount = "work"
				s.Accounts = map[string]Account{"work": {CLI: cli}}
			}
			v2, err := s.openCode2()
			wantError := tt.invalid || tt.missing || tt.version == "3.0.0"
			if (err != nil) != wantError || !wantError && v2 != strings.HasPrefix(tt.version, "2.") {
				t.Fatalf("openCode2() = %v, %v; want error=%v, version=%s", v2, err, wantError, tt.version)
			}
		})
	}
}
