package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/server"
	"skillshare/internal/ui"
)

func cmdUI(args []string) error {
	port := "19420"
	host := "127.0.0.1"
	noOpen := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 < len(args) {
				i++
				port = args[i]
			} else {
				return fmt.Errorf("--port requires a value")
			}
		case "--host":
			if i+1 < len(args) {
				i++
				host = args[i]
			} else {
				return fmt.Errorf("--host requires a value")
			}
		case "--no-open":
			noOpen = true
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	cfg, err := loadUIConfig()
	if err != nil {
		return err
	}

	addr := host + ":" + port
	url := "http://" + addr

	if !noOpen {
		ui.Success("Opening %s in your browser...", url)
		openBrowser(url)
	}

	srv := server.New(cfg, addr)
	return srv.Start()
}

func loadUIConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("skillshare is not initialized: run 'skillshare init' first")
	}

	source := strings.TrimSpace(cfg.Source)
	if source == "" {
		return nil, fmt.Errorf("invalid config: source is empty (run 'skillshare init' first)")
	}

	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source directory not found: %s (run 'skillshare init' first)", source)
		}
		return nil, fmt.Errorf("failed to access source directory %s: %w", source, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s (run 'skillshare init' first)", source)
	}

	return cfg, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	cmd.Start()
}
