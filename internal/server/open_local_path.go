package server

import (
	"errors"
	"os/exec"
	"runtime"
)

var errOpenLocalPathUnsupported = errors.New("opening local files is not supported on this platform")

func openLocalPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		return errOpenLocalPathUnsupported
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
