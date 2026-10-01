package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// GitRunner is intentionally limited to management operations. Hook execution is
// never an operation of the hooks service, including previews and recovery.
type GitRunner interface {
	Run(dir string, stdin []byte, args ...string) ([]byte, error)
}

type execGitRunner struct{ env []string }

func (g execGitRunner) Run(dir string, stdin []byte, args ...string) ([]byte, error) {
	if err := allowGit(args); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	env := g.env
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = sanitizedGitEnv(env)
	cmd.Stdin = bytes.NewReader(stdin)
	// Do not include stderr in errors: config parse failures can contain private
	// user settings. Callers distinguish missing values using the exit code.
	return cmd.Output()
}

func allowGit(args []string) error {
	deny := func() error { return fmt.Errorf("Git operation is not permitted by hooks management") }
	if slices.Equal(args, []string{"version"}) || slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir"}) {
		return nil
	}
	if len(args) >= 3 && args[0] == "hook" && args[1] == "list" {
		event := args[2:]
		if len(event) == 2 && event[0] == "--show-scope" {
			event = event[1:]
		}
		if len(event) == 1 && gitEventName.MatchString(event[0]) {
			return nil
		}
		return deny()
	}
	if len(args) < 2 || args[0] != "config" {
		return deny()
	}
	file, scope, op := "", "", ""
	var values []string
	fixed := false
	pathValues := false
	for i := 1; i < len(args); i++ {
		a := args[i]
		if op != "" {
			values = append(values, a)
			continue
		}
		switch a {
		case "--global", "--system", "--local":
			if scope != "" {
				return deny()
			}
			scope = a
		case "--file":
			if scope != "" || i+1 >= len(args) {
				return deny()
			}
			i++
			file, scope = args[i], "--file"
			if file != "-" && !filepath.IsAbs(file) {
				return deny()
			}
		case "--includes", "--no-includes", "--null", "--show-origin", "--show-scope":
		case "--path":
			pathValues = true
		case "--fixed-value":
			fixed = true
		case "--get", "--get-all", "--get-regexp", "--list", "--add", "--unset", "--remove-section":
			op = a
		default:
			return deny()
		}
	}
	switch op {
	case "--get", "--get-all", "--get-regexp":
		if len(values) == 1 && !fixed {
			return nil
		}
	case "--list":
		if len(values) == 0 && !fixed {
			return nil
		}
	case "--add", "--unset":
		if !pathValues && file != "" && file != "-" && len(values) == 2 && values[0] == "include.path" && (op == "--add" && !fixed || op == "--unset" && fixed) {
			return nil
		}
	case "--remove-section":
		if !pathValues && file != "" && file != "-" && len(values) == 1 && strings.HasPrefix(values[0], "hook.") && validGitFriendlyName(strings.TrimPrefix(values[0], "hook.")) && !fixed {
			return nil
		}
	}
	return deny()
}

func sanitizedGitEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		key = strings.ToUpper(key) // Windows environment names are case-insensitive.
		switch key {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_PREFIX", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "LC_ALL":
			continue
		}
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, item)
	}
	return append(out, "LC_ALL=C")
}

func (s *Service) gitRunner() GitRunner {
	if s.Git != nil {
		return s.Git
	}
	env := os.Environ()
	set := func(key, value string) {
		env = slices.DeleteFunc(env, func(v string) bool { return strings.HasPrefix(v, key+"=") })
		env = append(env, key+"="+value)
	}
	if s.Home != "" {
		set("HOME", s.Home)
	}
	if s.ConfigDirs["xdg"] != "" {
		set("XDG_CONFIG_HOME", s.ConfigDirs["xdg"])
	}
	if s.GitGlobalConfig != "" {
		set("GIT_CONFIG_GLOBAL", s.GitGlobalConfig)
	}
	return execGitRunner{env: env}
}

func gitNotFoundValue(err error) bool {
	e, ok := err.(*exec.ExitError)
	return ok && e.ExitCode() == 1
}
