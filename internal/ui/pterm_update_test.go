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

	UpdateNotification("1.0.0", "2.0.0", "skillshare upgrade")

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
