package devcontainer_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func prepareWrapperEnv(t *testing.T) (wrapper string, workspace string, demo string, tmpBin string, argsFile string, fakeGo string, home string) {
	t.Helper()

	root := repoRoot(t)
	wrapper = filepath.Join(root, ".devcontainer", "bin", "skillshare")

	tmp := t.TempDir()
	workspace = filepath.Join(tmp, "workspace")
	home = filepath.Join(tmp, "home")
	demo = filepath.Join(home, "demo-project")
	tmpBin = filepath.Join(tmp, "skillshare-dev")
	argsFile = filepath.Join(tmp, "captured-args.txt")
	fakeGo = filepath.Join(tmp, "fake-go")

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(demo, 0o755); err != nil {
		t.Fatalf("mkdir demo project: %v", err)
	}

	writeFile(t, filepath.Join(workspace, "go.mod"), "module skillshare-test\n")
	writeExecutable(t, fakeGo, `#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" != "build" ]; then
  echo "unexpected fake go command: $*" >&2
  exit 1
fi

out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then
    out="$arg"
    break
  fi
  prev="$arg"
done

if [ -z "$out" ]; then
  echo "missing -o output path" >&2
  exit 1
fi

cat > "$out" <<'FAKE_BIN'
#!/usr/bin/env bash
set -euo pipefail
pwd
printf '%s\n' "$@" > "${SKILLSHARE_TEST_ARGS_FILE:?}"
FAKE_BIN
chmod +x "$out"
`)

	return wrapper, workspace, demo, tmpBin, argsFile, fakeGo, home
}

func runWrapper(t *testing.T, wrapper string, dir string, env []string, args ...string) (stdout string, stderr string) {
	t.Helper()
	cmd := exec.Command(wrapper, args...)
	cmd.Dir = dir
	cmd.Env = env

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		t.Fatalf("run wrapper (%v) failed: %v\nstdout:\n%s\nstderr:\n%s", args, err, outBuf.String(), errBuf.String())
	}

	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String())
}

func readCapturedArgs(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	return strings.Fields(string(raw))
}

func canonicalExistingDir(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve path %s: %v", path, err)
	}
	return resolved
}

func TestSkillshareWrapper_PreservesCallerCWD(t *testing.T) {
	wrapper, workspace, demo, tmpBin, argsFile, fakeGo, home := prepareWrapperEnv(t)
	caller := filepath.Join(home, "caller")
	if err := os.MkdirAll(caller, 0o755); err != nil {
		t.Fatalf("mkdir caller: %v", err)
	}

	env := append(os.Environ(),
		"HOME="+home,
		"SKILLSHARE_DEV_WORKSPACE_ROOT="+workspace,
		"SKILLSHARE_DEV_PROJECT_ROOT="+demo,
		"SKILLSHARE_DEV_TMP_BINARY="+tmpBin,
		"SKILLSHARE_DEV_GO_BIN="+fakeGo,
		"SKILLSHARE_TEST_ARGS_FILE="+argsFile,
	)

	stdout, stderr := runWrapper(t, wrapper, caller, env, "status")
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}
	expectedCWD := canonicalExistingDir(t, caller)
	actualCWD := canonicalExistingDir(t, stdout)
	if actualCWD != expectedCWD {
		t.Fatalf("expected cwd %q, got %q", expectedCWD, actualCWD)
	}

	gotArgs := readCapturedArgs(t, argsFile)
	wantArgs := []string{"status"}
	if strings.Join(gotArgs, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("expected args %v, got %v", wantArgs, gotArgs)
	}
}

func TestSkillshareWrapper_ProjectModeRedirectsFromWorkspaceRoot(t *testing.T) {
	wrapper, workspace, demo, tmpBin, argsFile, fakeGo, home := prepareWrapperEnv(t)
	env := append(os.Environ(),
		"HOME="+home,
		"SKILLSHARE_DEV_WORKSPACE_ROOT="+workspace,
		"SKILLSHARE_DEV_PROJECT_ROOT="+demo,
		"SKILLSHARE_DEV_TMP_BINARY="+tmpBin,
		"SKILLSHARE_DEV_GO_BIN="+fakeGo,
		"SKILLSHARE_TEST_ARGS_FILE="+argsFile,
	)

	stdout, stderr := runWrapper(t, wrapper, workspace, env, "status", "-p")
	expectedDemo := canonicalExistingDir(t, demo)
	actualCWD := canonicalExistingDir(t, stdout)
	if actualCWD != expectedDemo {
		t.Fatalf("expected cwd %q, got %q", expectedDemo, actualCWD)
	}
	if !strings.Contains(stderr, "project mode redirected to "+demo) {
		t.Fatalf("expected redirect message in stderr, got: %q", stderr)
	}

	gotArgs := readCapturedArgs(t, argsFile)
	wantArgs := []string{"status", "-p"}
	if strings.Join(gotArgs, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("expected args %v, got %v", wantArgs, gotArgs)
	}
}
