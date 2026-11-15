package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// openCodeNPMExecutable recognizes npm's native-binary shim, not a general
// batch program. Resolve the installed binary without invoking cmd.exe or
// evaluating paths, environment variables, or commands from the shim.
func openCodeNPMExecutable(shim string) (string, error) {
	data, err := os.ReadFile(shim)
	if err != nil {
		return "", err
	}
	// npm/cmd-shim's no-shebang template, shared by current v1 and v2 packages.
	const header = "@ECHO off\nGOTO start\n:find_dp0\nSET dp0=%~dp0\nEXIT /b\n:start\nSETLOCAL\nCALL :find_dp0\n"
	body := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, prefix := range []string{`node_modules\`, `..\`} {
		for _, pkg := range []string{`opencode-ai`, `@opencode\cli`} {
			rel := prefix + pkg + `\bin\opencode.exe`
			if body == header+`"%dp0%\`+rel+`"   %*`+"\n" {
				// The local npm shim lives in node_modules/.bin. These are
				// fixed paths only; no batch-file text becomes a command.
				return filepath.Join(filepath.Dir(shim), filepath.FromSlash(strings.ReplaceAll(rel, `\`, "/"))), nil
			}
		}
	}
	return "", fmt.Errorf("unrecognized OpenCode npm launcher; put the installed native opencode.exe first on PATH, or set an account target's cli to its path")
}
