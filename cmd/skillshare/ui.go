package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/server"
	"skillshare/internal/ui"
	"skillshare/internal/uidist"
	versionpkg "skillshare/internal/version"
)

const (
	uiStartMode = "start"
	uiStopMode  = "stop"
)

type uiBackgroundOptions struct {
	mode       runMode
	host       string
	port       string
	basePath   string
	noOpen     bool
	appWindow  bool
	clearCache bool
}

type currentUIState struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	BasePath string `json:"basePath"`
}

func cmdUI(args []string) error {
	if wantsHelp(args) {
		printUIHelp()
		return nil
	}

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	uiMode, uiRest := splitUIMode(rest)
	switch uiMode {
	case uiStartMode:
		opts, err := parseUIBackgroundOptions(mode, uiRest)
		if err != nil {
			return err
		}
		return startUIInBackground(resolveRememberedUIOptions(opts))
	case uiStopMode:
		opts, err := parseUIBackgroundOptions(mode, uiRest)
		if err != nil {
			return err
		}
		return cmdUIStop(resolveRememberedUIOptions(opts))
	}

	port := "19420"
	host := "127.0.0.1"
	basePath := ""
	noOpen := false
	clearCache := false

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
		case "--base-path", "-b":
			if i+1 < len(rest) {
				i++
				basePath = rest[i]
			} else {
				return fmt.Errorf("--base-path requires a value")
			}
		case "--no-open":
			noOpen = true
		case "--clear-cache":
			clearCache = true
		default:
			return fmt.Errorf("unknown flag: %s", rest[i])
		}
	}

	if clearCache {
		if err := uidist.ClearCache(); err != nil {
			return fmt.Errorf("failed to clear UI cache: %w", err)
		}
		ui.Success("UI cache cleared.")
		return nil
	}

	// Env var fallback for base path
	if basePath == "" {
		basePath = os.Getenv("SKILLSHARE_UI_BASE_PATH")
	}

	// Auto-detect project mode
	mode = resolveUIMode(mode)
	applyModeLabel(mode)

	addr := host + ":" + port
	url := uiURL(uiBackgroundOptions{host: host, port: port, basePath: basePath})

	if mode == modeProject {
		return startProjectUI(addr, url, basePath, noOpen)
	}
	return startGlobalUI(addr, url, basePath, noOpen)
}

func splitUIMode(args []string) (string, []string) {
	if len(args) > 0 && (args[0] == uiStartMode || args[0] == uiStopMode) {
		return args[0], args[1:]
	}
	return "", args
}

func parseUIBackgroundOptions(mode runMode, args []string) (uiBackgroundOptions, error) {
	opts := uiBackgroundOptions{
		mode:     mode,
		host:     envOrDefaultUI("SKILLSHARE_UI_HOST", "127.0.0.1"),
		port:     envOrDefaultUI("SKILLSHARE_UI_PORT", "19420"),
		basePath: envOrDefaultUI("SKILLSHARE_UI_BASE_PATH", ""),
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			value, next, err := requireUIFlagValue(args, i, "--port")
			if err != nil {
				return uiBackgroundOptions{}, err
			}
			if err := validateUIPort(value); err != nil {
				return uiBackgroundOptions{}, err
			}
			opts.port = value
			i = next
		case "--host":
			value, next, err := requireUIFlagValue(args, i, "--host")
			if err != nil {
				return uiBackgroundOptions{}, err
			}
			opts.host = value
			i = next
		case "--base-path", "-b":
			value, next, err := requireUIFlagValue(args, i, args[i])
			if err != nil {
				return uiBackgroundOptions{}, err
			}
			opts.basePath = value
			i = next
		case "--no-open":
			opts.noOpen = true
		case "--app":
			opts.appWindow = true
		case "--clear-cache":
			opts.clearCache = true
		default:
			return uiBackgroundOptions{}, fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	if err := validateUIPort(opts.port); err != nil {
		return uiBackgroundOptions{}, err
	}
	return opts, nil
}

func requireUIFlagValue(args []string, index int, name string) (string, int, error) {
	next := index + 1
	if next >= len(args) || args[next] == "" {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return args[next], next, nil
}

func validateUIPort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("--port must be a number between 1 and 65535")
	}
	return nil
}

func resolveUIMode(mode runMode) runMode {
	if mode != modeAuto {
		return mode
	}
	cwd, _ := os.Getwd()
	if projectConfigExists(cwd) {
		return modeProject
	}
	return modeGlobal
}

func resolveRememberedUIOptions(opts uiBackgroundOptions) uiBackgroundOptions {
	// Explicit command-line/env options should win; the remembered state is only
	// used when the user asks to stop/reuse the default UI without flags.
	if os.Getenv("SKILLSHARE_UI_HOST") != "" || os.Getenv("SKILLSHARE_UI_PORT") != "" || os.Getenv("SKILLSHARE_UI_BASE_PATH") != "" {
		return opts
	}
	state, err := readCurrentUIState()
	if err != nil {
		return opts
	}
	if opts.host == "127.0.0.1" && opts.port == "19420" && opts.basePath == "" {
		opts.host = state.Host
		opts.port = state.Port
		opts.basePath = state.BasePath
	}
	return opts
}

func startUIInBackground(opts uiBackgroundOptions) error {
	opts.mode = resolveUIMode(opts.mode)
	if uiServerReady(opts) {
		_ = writeCurrentUIState(opts)
		url := uiURL(opts)
		ui.Info("Skillshare UI already running: %s", url)
		if !opts.noOpen {
			openUIWindow(url, opts.appWindow)
		}
		return nil
	}
	if err := ensureUIPortAvailable(opts); err != nil {
		return err
	}
	if opts.clearCache {
		if err := uidist.ClearCache(); err != nil {
			return fmt.Errorf("failed to clear UI cache: %w", err)
		}
	}
	if err := prepareUIForBackground(); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := openUILog()
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, uiChildArgs(opts)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	detachUICommand(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	if err := writeUIPidFile(opts, pid); err != nil {
		return err
	}
	if err := writeCurrentUIState(opts); err != nil {
		return err
	}

	url := uiURL(opts)
	if waitForUIServer(opts, 5*time.Second) {
		if !opts.noOpen {
			openUIWindow(url, opts.appWindow)
		}
		ui.Success("Skillshare UI running in background: %s", url)
		return nil
	}
	ui.Info("Skillshare UI starting in background: %s", url)
	ui.Info("Log: %s", logFile.Name())
	return nil
}

func cmdUIStop(opts uiBackgroundOptions) error {
	stopped, err := stopUI(opts)
	if err != nil {
		return err
	}
	if stopped {
		ui.Success("Skillshare UI stopped: %s", uiURL(opts))
	} else {
		ui.Info("Skillshare UI is not running: %s", uiURL(opts))
	}
	return nil
}

func cmdUIRestart(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	opts, err := parseUIBackgroundOptions(mode, rest)
	if err != nil {
		return err
	}
	opts = resolveRememberedUIOptions(opts)
	if !waitForUIServerDown(opts, 10*time.Second) {
		return fmt.Errorf("UI server did not stop before restart: %s", uiURL(opts))
	}
	return startUIInBackground(opts)
}

func uiChildArgs(opts uiBackgroundOptions) []string {
	args := []string{"ui", "--no-open", "--host", opts.host, "--port", opts.port}
	if opts.basePath != "" {
		args = append(args, "--base-path", opts.basePath)
	}
	if opts.mode == modeProject {
		args = append(args, "--project")
	} else if opts.mode == modeGlobal {
		args = append(args, "--global")
	}
	return args
}

func uiRestartArgs(opts uiBackgroundOptions) []string {
	args := []string{"__ui-restart", "--no-open", "--host", opts.host, "--port", opts.port}
	if opts.basePath != "" {
		args = append(args, "--base-path", opts.basePath)
	}
	if opts.clearCache {
		args = append(args, "--clear-cache")
	}
	if opts.mode == modeProject {
		args = append(args, "--project")
	} else if opts.mode == modeGlobal {
		args = append(args, "--global")
	}
	return args
}

func stopUI(opts uiBackgroundOptions) (bool, error) {
	if !uiServerReady(opts) {
		_ = os.Remove(uiPidFile(opts))
		_ = clearCurrentUIStateIfMatches(opts)
		return false, nil
	}
	pids := uiPIDs(opts)
	if len(pids) == 0 {
		return false, fmt.Errorf("UI server is running at %s, but no owning process could be found", uiURL(opts))
	}
	var killErr error
	for _, pid := range pids {
		if err := stopProcess(pid); err != nil {
			killErr = errors.Join(killErr, err)
		}
	}
	if waitForUIServerDown(opts, 3*time.Second) {
		_ = os.Remove(uiPidFile(opts))
		_ = clearCurrentUIStateIfMatches(opts)
		return true, killErr
	}
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			killErr = errors.Join(killErr, err)
			continue
		}
		if err := process.Kill(); err != nil {
			killErr = errors.Join(killErr, err)
		}
	}
	if waitForUIServerDown(opts, 2*time.Second) {
		_ = os.Remove(uiPidFile(opts))
		_ = clearCurrentUIStateIfMatches(opts)
		return true, killErr
	}
	return false, errors.Join(killErr, fmt.Errorf("UI server did not stop: %s", uiURL(opts)))
}

func stopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return process.Kill()
	}
	return process.Signal(os.Interrupt)
}

func uiPIDs(opts uiBackgroundOptions) []int {
	seen := map[int]struct{}{}
	if pid, err := readUIPidFile(opts); err == nil && pid > 0 {
		seen[pid] = struct{}{}
	}
	for _, pid := range pidsListeningOnPort(opts.port) {
		seen[pid] = struct{}{}
	}
	pids := make([]int, 0, len(seen))
	for pid := range seen {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

func writeUIPidFile(opts uiBackgroundOptions, pid int) error {
	if err := os.MkdirAll(config.CacheDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(uiPidFile(opts), []byte(strconv.Itoa(pid)+"\n"), 0644)
}

func readUIPidFile(opts uiBackgroundOptions) (int, error) {
	body, err := os.ReadFile(uiPidFile(opts))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(body)))
}

func uiPidFile(opts uiBackgroundOptions) string {
	name := strings.NewReplacer(":", "_", ".", "_").Replace(uiAddr(opts))
	return filepath.Join(config.CacheDir(), "ui-"+name+".pid")
}

func writeCurrentUIState(opts uiBackgroundOptions) error {
	if err := os.MkdirAll(config.CacheDir(), 0755); err != nil {
		return err
	}
	body, err := json.Marshal(currentUIState{Host: opts.host, Port: opts.port, BasePath: opts.basePath})
	if err != nil {
		return err
	}
	return os.WriteFile(currentUIStateFile(), append(body, '\n'), 0644)
}

func readCurrentUIState() (currentUIState, error) {
	body, err := os.ReadFile(currentUIStateFile())
	if err != nil {
		return currentUIState{}, err
	}
	var state currentUIState
	if err := json.Unmarshal(body, &state); err != nil {
		return currentUIState{}, err
	}
	if state.Host == "" || validateUIPort(state.Port) != nil {
		return currentUIState{}, fmt.Errorf("invalid UI state")
	}
	return state, nil
}

func clearCurrentUIStateIfMatches(opts uiBackgroundOptions) error {
	state, err := readCurrentUIState()
	if err != nil {
		return nil
	}
	if state.Host == opts.host && state.Port == opts.port && state.BasePath == opts.basePath {
		return os.Remove(currentUIStateFile())
	}
	return nil
}

func currentUIStateFile() string {
	return filepath.Join(config.CacheDir(), "ui-current.json")
}

func pidsListeningOnPort(port string) []int {
	if runtime.GOOS == "windows" {
		return nil
	}
	out, err := exec.Command("lsof", "-tiTCP:"+port, "-sTCP:LISTEN").Output()
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(out))
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func openUILog() (*os.File, error) {
	dir := config.CacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "ui.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

func ensureUIPortAvailable(opts uiBackgroundOptions) error {
	listener, err := net.Listen("tcp", uiAddr(opts))
	if err != nil {
		return fmt.Errorf("UI server is not ready and %s is unavailable: %w", uiAddr(opts), err)
	}
	return listener.Close()
}

func uiAddr(opts uiBackgroundOptions) string {
	return opts.host + ":" + opts.port
}

func uiURL(opts uiBackgroundOptions) string {
	url := "http://" + uiAddr(opts)
	if bp := server.NormalizeBasePath(opts.basePath); bp != "" {
		url += bp + "/"
	}
	return url
}

func uiHealthURL(opts uiBackgroundOptions) string {
	connectOpts := opts
	connectOpts.host = uiConnectHost(opts.host)
	url := uiURL(connectOpts)
	if strings.HasSuffix(url, "/") {
		return url + "api/health"
	}
	return url + "/api/health"
}

func uiConnectHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

func waitForUIServer(opts uiBackgroundOptions, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if uiServerReady(opts) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func waitForUIServerDown(opts uiBackgroundOptions, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !uiServerReady(opts) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !uiServerReady(opts)
}

func uiServerReady(opts uiBackgroundOptions) bool {
	client := http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get(uiHealthURL(opts))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func prepareUIForBackground() error {
	if versionpkg.Version == "" || versionpkg.Version == "dev" {
		_, err := ensureUIAvailable()
		return err
	}
	if _, ok := uidist.IsCached(versionpkg.Version); ok {
		return nil
	}
	sp := ui.StartSpinner("Downloading UI assets...")
	if _, err := ensureUIAvailable(); err != nil {
		sp.Fail("Download failed")
		return err
	}
	sp.Success("UI assets ready")
	return nil
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
	if err := uidist.Download(ver, downloadProgress("Downloading UI assets...", sp.Update)); err != nil {
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

func startProjectUI(addr, url, basePath string, noOpen bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if !projectConfigExists(cwd) {
		return fmt.Errorf("project not initialized: run 'skillshare init -p' first")
	}

	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}
	// A target that cannot resolve (no path) must not keep the dashboard from
	// starting; its sync reports it as failed.
	_, unresolved := config.ResolveValidProjectTargets(cwd, projCfg)
	for _, name := range slices.Sorted(maps.Keys(unresolved)) {
		ui.Warning("%s: %v", name, unresolved[name])
	}
	rt, err := newProjectRuntime(cwd, projCfg, unresolved)
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

	srv := server.NewProject(cfg, rt.config, cwd, addr, basePath, uiDir)
	if !noOpen {
		srv.SetOnReady(func() {
			ui.Success("Opening %s in your browser... (project mode)", url)
			openBrowser(url)
		})
	}
	return srv.Start()
}

func startGlobalUI(addr, url, basePath string, noOpen bool) error {
	cfg, err := loadUIConfig()
	if err != nil {
		return err
	}

	uiDir, err := ensureUIAvailable()
	if err != nil {
		return err
	}

	srv := server.New(cfg, addr, basePath, uiDir)
	if !noOpen {
		srv.SetOnReady(func() {
			ui.Success("Opening %s in your browser...", url)
			openBrowser(url)
		})
	}
	return srv.Start()
}

func loadUIConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("skillshare is not initialized (global): run 'skillshare init' first")
	}

	source := strings.TrimSpace(cfg.EffectiveSkillsSource())
	if source == "" {
		return nil, fmt.Errorf("invalid global config (%s): source is empty; fix 'sources.skills' or run 'skillshare init --source <path>'", config.ConfigPath())
	}

	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("global source directory not found: %s (config: %s)\nThe config file exists but its source directory is missing. Create it with 'mkdir -p %s', or point 'sources.skills' in the config at an existing directory", source, config.ConfigPath(), source)
		}
		return nil, fmt.Errorf("failed to access global source directory %s: %w", source, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("global source path is not a directory: %s (config: %s)", source, config.ConfigPath())
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

func openUIWindow(url string, appWindow bool) {
	if appWindow && openBrowserAppWindow(url) == nil {
		return
	}
	openBrowser(url)
}

func openBrowserAppWindow(url string) error {
	switch runtime.GOOS {
	case "darwin":
		for _, app := range []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium"} {
			if err := exec.Command("open", "-na", app, "--args", "--app="+url).Start(); err == nil {
				return nil
			}
		}
	case "windows":
		for _, name := range []string{"chrome", "msedge", "brave", "chromium"} {
			if _, err := exec.LookPath(name); err != nil {
				continue
			}
			return exec.Command("cmd", "/c", "start", "", name, "--app="+url).Start()
		}
	default:
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			path, err := exec.LookPath(name)
			if err != nil {
				continue
			}
			return exec.Command(path, "--app="+url).Start()
		}
	}
	return fmt.Errorf("no browser app window launcher found")
}

func envOrDefaultUI(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func printUIHelp() {
	printHelp("skillshare ui [options]\n       skillshare ui start [options]\n       skillshare ui stop [options]", "Launch the web dashboard.",
		helpGroup{title: "Modes", rows: []helpRow{
			{"(default)", "Run the UI server in the foreground"},
			{"start", "Start or reuse a background UI server"},
			{"stop", "Stop a background UI server"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--port <port>", "Server port (default: 19420)"},
			{"--host <host>", "Server host (default: 127.0.0.1)"},
			{"-b, --base-path", "Base path prefix for reverse proxy"},
			{"--no-open", "Don't open browser automatically"},
			{"--app", "Open as desktop-style app window when supported (start only)"},
			{"--clear-cache", "Clear cached UI assets and exit (foreground) or before start"},
			{"-p, --project", "Use project-level config"},
			{"-g, --global", "Use global config"},
		}},
		helpExamples(
			helpRow{"skillshare ui", "Launch dashboard in foreground"},
			helpRow{"skillshare ui start", "Start dashboard in background"},
			helpRow{"skillshare ui stop", "Stop background dashboard"},
			helpRow{"skillshare ui --port 8080", "Launch on custom port"},
			helpRow{"skillshare ui --no-open", "Launch without opening browser"},
			helpRow{"skillshare ui -p", "Launch in project mode"},
			helpRow{"skillshare ui --clear-cache", "Clear cached UI files"},
		),
	)
}
