package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestUpdateNotification_WritesToStderr(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = wErr

	// Capture stdout
	oldStdout := os.Stdout
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wOut

	UpdateNotification("1.0.0", "2.0.0", "skillshare upgrade", "")

	wErr.Close()
	wOut.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, rErr)

	var stdoutBuf bytes.Buffer
	io.Copy(&stdoutBuf, rOut)

	if stdoutBuf.Len() > 0 {
		t.Errorf("UpdateNotification must not write to stdout, got: %s", stdoutBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "1.0.0") || !strings.Contains(stderrBuf.String(), "2.0.0") {
		t.Errorf("UpdateNotification should write version info to stderr, got: %s", stderrBuf.String())
	}
}

func TestUpdateNotification_PlainRunLine(t *testing.T) {
	tests := []struct {
		name, cmd, hint, want string
	}{
		{name: "command", cmd: "brew upgrade skillshare", want: "  Run 'brew upgrade skillshare' to update\n"},
		{name: "no single command", hint: "update skillshare in your Nix configuration and rebuild", want: "  Update skillshare in your Nix configuration and rebuild\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout, oldStderr := os.Stdout, os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stdout, os.Stderr = w, w // a pipe, not a terminal
			UpdateNotification("1.0.0", "2.0.0", tt.cmd, tt.hint)
			w.Close()
			os.Stdout, os.Stderr = oldStdout, oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			if !strings.HasSuffix(buf.String(), tt.want) {
				t.Errorf("got %q, want it to end with %q", buf.String(), tt.want)
			}
		})
	}
}
