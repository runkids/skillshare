package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// Result holds command execution results
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// RunCLI executes the skillshare CLI with given arguments
func (sb *Sandbox) RunCLI(args ...string) *Result {
	sb.T.Helper()

	cmd := exec.Command(sb.BinaryPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+sb.Home,
		"SKILLSHARE_CONFIG="+sb.ConfigPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			sb.T.Logf("failed to run CLI: %v", err)
			exitCode = -1
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

// RunCLIWithInput executes CLI with stdin input (for interactive prompts)
func (sb *Sandbox) RunCLIWithInput(input string, args ...string) *Result {
	sb.T.Helper()

	cmd := exec.Command(sb.BinaryPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+sb.Home,
		"SKILLSHARE_CONFIG="+sb.ConfigPath,
	)
	cmd.Stdin = bytes.NewBufferString(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			sb.T.Logf("failed to run CLI: %v", err)
			exitCode = -1
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

// RunCLIInDir executes the CLI with working directory set to dir
func (sb *Sandbox) RunCLIInDir(dir string, args ...string) *Result {
	sb.T.Helper()
	cmd := exec.Command(sb.BinaryPath, args...)
	cmd.Dir = dir
	cmd.Env = sb.envInDir(dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			sb.T.Logf("failed to run CLI: %v", err)
			exitCode = -1
		}
	}
	return &Result{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}
}

// RunCLIInDirWithInput executes CLI with working directory and stdin input
func (sb *Sandbox) RunCLIInDirWithInput(dir, input string, args ...string) *Result {
	sb.T.Helper()
	cmd := exec.Command(sb.BinaryPath, args...)
	cmd.Dir = dir
	cmd.Env = sb.envInDir(dir)
	cmd.Stdin = bytes.NewBufferString(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			sb.T.Logf("failed to run CLI: %v", err)
			exitCode = -1
		}
	}
	return &Result{ExitCode: exitCode, Stdout: stdout.String(), Stderr: stderr.String()}
}

func (sb *Sandbox) envInDir(dir string) []string {
	pwd, err := filepath.Abs(dir)
	if err != nil {
		sb.T.Fatalf("failed to resolve CLI working directory: %v", err)
	}
	// os/exec preserves an explicitly inherited PWD. Keep it aligned with Dir
	// so os.Getwd retains the requested path through symlinked directories.
	return append(os.Environ(),
		"HOME="+sb.Home,
		"SKILLSHARE_CONFIG="+sb.ConfigPath,
		"PWD="+pwd,
	)
}

// Output returns combined stdout and stderr
func (r *Result) Output() string {
	return r.Stdout + r.Stderr
}

// RunCLIEnv executes the CLI with additional environment variables.
// The extra env vars are appended after the standard HOME and
// SKILLSHARE_CONFIG, so they override any pre-existing values.
func (sb *Sandbox) RunCLIEnv(extraEnv map[string]string, args ...string) *Result {
	sb.T.Helper()

	cmd := exec.Command(sb.BinaryPath, args...)
	env := append(os.Environ(),
		"HOME="+sb.Home,
		"SKILLSHARE_CONFIG="+sb.ConfigPath,
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			sb.T.Logf("failed to run CLI: %v", err)
			exitCode = -1
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}
