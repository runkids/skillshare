package main

import (
	"skillshare/internal/config"
)

func cmdLogProject(args []string, cwd string) error {
	return runLog(args, config.ProjectConfigPath(cwd))
}
