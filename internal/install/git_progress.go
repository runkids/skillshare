package install

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
)

// ProgressCallback receives incremental progress lines from long-running git
// commands (clone/fetch/pull) when running in TTY mode.
type ProgressCallback func(line string)

// runGitCommandWithProgress runs a git command with optional progress streaming.
// When onProgress is nil, stderr is buffered (quiet mode). When non-nil, stderr
// is streamed line-by-line using scanGitProgress to support both \r and \n.
func runGitCommandWithProgress(args []string, dir string, extraEnv []string, onProgress ProgressCallback) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(cmd.Env, extraEnv...)

	if onProgress == nil {
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return wrapGitError(stderr.String(), err, usedTokenAuth(extraEnv))
		}
		return nil
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	stderrText, scanErr := streamGitProgress(stderrPipe, onProgress)
	waitErr := cmd.Wait()

	if waitErr != nil {
		return wrapGitError(stderrText, waitErr, usedTokenAuth(extraEnv))
	}
	if scanErr != nil {
		return scanErr
	}

	return nil
}

func streamGitProgress(stderr io.Reader, onProgress ProgressCallback) (string, error) {
	var all bytes.Buffer
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanGitProgress)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		all.WriteString(line)
		all.WriteByte('\n')

		msg := strings.TrimSpace(sanitizeTokens(line))
		if msg != "" {
			onProgress(msg)
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return all.String(), err
	}

	return all.String(), nil
}

// scanGitProgress splits git progress stream on either carriage-return or
// newline so updates like "Receiving objects: 34%\r" are surfaced incrementally.
func scanGitProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}

	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}

	return 0, nil, nil
}
