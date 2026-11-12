// Package gitenv runs git without the caller's repository selection.
//
// Git exports GIT_DIR and related variables to hooks, so skillshare started
// from a hook would otherwise run its own git commands against the hook's
// repository instead of the directory it names (#852).
package gitenv

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// repoVars are the variables `git rev-parse --local-env-vars` lists that pick
// a repository, work tree, index, object store, or config file.
// GIT_CONFIG_PARAMETERS and GIT_CONFIG_COUNT stay: they carry user settings
// such as credentials, and install's token auth appends to them.
var repoVars = map[string]bool{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_COMMON_DIR":                   true,
	"GIT_CONFIG":                       true,
	"GIT_DIR":                          true,
	"GIT_GRAFT_FILE":                   true,
	"GIT_IMPLICIT_WORK_TREE":           true,
	"GIT_INDEX_FILE":                   true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_PREFIX":                       true,
	"GIT_REPLACE_REF_BASE":             true,
	"GIT_SHALLOW_FILE":                 true,
	"GIT_WORK_TREE":                    true,
}

// Environ returns os.Environ() without the repository selection variables.
func Environ() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		if !repoVars[strings.ToUpper(key)] { // Windows names are case-insensitive.
			out = append(out, item)
		}
	}
	return out
}

// Command is exec.Command("git", args...) with Env set to Environ().
// Add variables with cmd.Env = append(cmd.Env, ...).
func Command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Env = Environ()
	return cmd
}

// CommandContext is the context-aware form of Command.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = Environ()
	return cmd
}
