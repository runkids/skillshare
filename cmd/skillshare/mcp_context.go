package main

import (
	"os"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	syncpkg "skillshare/internal/sync"
)

// backupMCPSource keeps a config's content in the file history before a sync saves it
// without the MCP settings 0.23.0 retired.
func backupMCPSource(path string) (string, error) {
	return syncpkg.StoreBackup(path, syncpkg.BackupReasonMigrate)
}

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
	service := &mcp.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir(), ConfigDirs: mcp.ConfigDirsFromEnv(), BackupSource: backupMCPSource}
	if mode == modeProject {
		service.ConfigPath = config.ProjectConfigPath(cwd)
		service.ProjectRoot = cwd
	}
	applyModeLabel(mode)
	return service, rest, nil
}
