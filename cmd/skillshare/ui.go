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
	"skillshare/internal/uidist"
	versionpkg "skillshare/internal/version"
)

func cmdUI(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	port := "19420"
	host := "127.0.0.1"
	noOpen := false

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--port":
			if i+1 < len(rest) {
				i++
				port = rest[i]
			} else {
				return fmt.Errorf("--port requires a value")
			}
		case "--host":
			if i+1 < len(rest) {
				i++
				host = rest[i]
			} else {
				return fmt.Errorf("--host requires a value")
			}
		case "--no-open":
			noOpen = true
		case "--clear-cache":
			if err := uidist.ClearCache(); err != nil {
				return fmt.Errorf("failed to clear UI cache: %w", err)
			}
			ui.Success("UI cache cleared.")
			return nil
		default:
			return fmt.Errorf("unknown flag: %s", rest[i])
		}
	}

	// Auto-detect project mode
	if mode == modeAuto {
		cwd, _ := os.Getwd()
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	addr := host + ":" + port
	url := "http://" + addr

	if mode == modeProject {
		return startProjectUI(addr, url, noOpen)
	}
	return startGlobalUI(addr, url, noOpen)
}

// ensureUIAvailable checks whether the UI is cached and downloads it if needed.
// Returns the disk directory to serve from, or "" for dev mode (placeholder).
func ensureUIAvailable() (string, error) {
	ver := versionpkg.Version

	// Check cache first — works for all versions including "dev"
	// (e.g., Docker playground pre-populates cache for "dev")
	if dir, ok := uidist.IsCached(ver); ok {
		return dir, nil
	}

	if ver == "dev" || ver == "" {
		// Dev mode without cached UI: use placeholder, Vite serves the frontend
		return "", nil
	}

	// Download with spinner
	sp := ui.StartSpinner("Downloading UI assets...")
	if err := uidist.Download(ver); err != nil {
		sp.Fail("Download failed")
		fmt.Println()
		ui.Warning("Install with the full installer to get the web UI:")
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh")
		return "", fmt.Errorf("could not download UI assets: %w", err)
	}
	sp.Success("UI assets downloaded and cached")

	dir, ok := uidist.IsCached(ver)
	if !ok {
		return "", fmt.Errorf("UI assets were downloaded but cache verification failed")
	}
	return dir, nil
}

func startProjectUI(addr, url string, noOpen bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if !projectConfigExists(cwd) {
		return fmt.Errorf("project not initialized: run 'skillshare init -p' first")
	}

	rt, err := loadProjectRuntime(cwd)
	if err != nil {
		return err
	}

	// Build synthetic global config from project runtime
	cfg := &config.Config{
		Source:  rt.sourcePath,
		Targets: rt.targets,
		Mode:    "merge",
	}

	uiDir, err := ensureUIAvailable()
	if err != nil {
		return err
	}

	if !noOpen {
		ui.Success("Opening %s in your browser... (project mode)", url)
		openBrowser(url)
	}

	srv := server.NewProject(cfg, rt.config, cwd, addr, uiDir)
	return srv.Start()
}

func startGlobalUI(addr, url string, noOpen bool) error {
	cfg, err := loadUIConfig()
	if err != nil {
		return err
	}

	uiDir, err := ensureUIAvailable()
	if err != nil {
		return err
	}

	if !noOpen {
		ui.Success("Opening %s in your browser...", url)
		openBrowser(url)
	}

	srv := server.New(cfg, addr, uiDir)
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
	_ = cmd.Start()
}
