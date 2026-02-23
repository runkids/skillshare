package install

import (
	"bufio"
	"strings"
	"testing"
)

func TestScanGitProgress_SplitsOnCRAndLF(t *testing.T) {
	input := "Receiving objects: 10%\rReceiving objects: 40%\rResolving deltas: 100%\nDone\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Split(scanGitProgress)

	var lines []string
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	want := []string{
		"Receiving objects: 10%",
		"Receiving objects: 40%",
		"Resolving deltas: 100%",
		"Done",
	}
	if len(lines) != len(want) {
		t.Fatalf("line count = %d, want %d (%v)", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestRunGitCommandWithProgress_GitVersion(t *testing.T) {
	err := runGitCommandWithProgress([]string{"version"}, "", nil, func(string) {})
	if err != nil {
		t.Fatalf("runGitCommandWithProgress() error = %v", err)
	}
}
