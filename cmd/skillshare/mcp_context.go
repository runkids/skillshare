package main

import (
	"os"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
)

func mcpContext(args []string) (*mcp.Service, []string, error) {
	// Do not interpret flags belonging to a spawned server after --.
	var command []string
	for i, arg := range args {
		if arg == "--" {
			command = args[i:]
			args = args[:i]
			break
		}
	}
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return nil, nil, err
	}
	rest = append(rest, command...)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	if mode == modeAuto {
		mode = modeGlobal
		if projectConfigExists(cwd) {
			mode = modeProject
		}
	}
	service := &mcp.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir(), ConfigDirs: map[string]string{}}
	if mode == modeProject {
		service.ConfigPath = config.ProjectConfigPath(cwd)
		service.ProjectRoot = cwd
	}
	for key, env := range map[string]string{"codex": "CODEX_HOME", "claude": "CLAUDE_CONFIG_DIR", "xdg": "XDG_CONFIG_HOME", "appdata": "APPDATA"} {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			service.ConfigDirs[key] = value
		}
	}
	applyModeLabel(mode)
	return service, rest, nil
}
