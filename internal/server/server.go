package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/version"
)

// Server holds the HTTP server state
type Server struct {
	cfg      *config.Config
	registry *config.Registry
	addr     string
	mux      *http.ServeMux
	handler  http.Handler
	mu       sync.RWMutex // protects config: Lock for writes/reloads, RLock for reads

	startTime time.Time // for uptime reporting in health check

	// Project mode fields (empty/nil for global mode)
	projectRoot string
	projectCfg  *config.ProjectConfig

	// uiDistDir, when non-empty, serves UI from this disk directory
	// instead of the embedded SPA. Used for runtime-downloaded UI assets.
	uiDistDir string

	// basePath is the URL prefix under which the UI and API are served
	// (e.g. "/app"). Empty means serve at root.
	basePath string

	// onReady is called after the listener is bound but before serving.
	// Used to open the browser only after the port is confirmed available.
	onReady func()
}

// NormalizeBasePath ensures the base path starts with "/" and has no trailing slash.
// An empty or "/" input returns "".
func NormalizeBasePath(p string) string {
	p = strings.TrimRight(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// wrapBasePath wraps the handler chain with StripPrefix and bare-path redirect
// when basePath is set. Skips wrapping in dev mode (no uiDistDir) and prints a warning.
func (s *Server) wrapBasePath() {
	if s.basePath == "" {
		return
	}
	if s.uiDistDir == "" {
		fmt.Fprintf(os.Stderr, "Warning: --base-path is ignored in dev mode (no UI assets). Start Vite without base path.\n")
		s.basePath = ""
		return
	}
	stripped := http.StripPrefix(s.basePath, s.handler)
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == s.basePath {
			http.Redirect(w, r, s.basePath+"/", http.StatusMovedPermanently)
			return
		}
		stripped.ServeHTTP(w, r)
	})
}

// New creates a new Server for global mode.
// uiDistDir, when non-empty, serves UI from disk instead of the embedded SPA.
func New(cfg *config.Config, addr, basePath, uiDistDir string) *Server {
	reg, _ := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
	if reg == nil {
		reg = &config.Registry{}
	}
	s := &Server{
		cfg:       cfg,
		registry:  reg,
		addr:      addr,
		mux:       http.NewServeMux(),
		basePath:  NormalizeBasePath(basePath),
		uiDistDir: uiDistDir,
	}
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	s.wrapBasePath()
	return s
}

// NewProject creates a new Server for project mode.
// uiDistDir, when non-empty, serves UI from disk instead of the embedded SPA.
func NewProject(cfg *config.Config, projectCfg *config.ProjectConfig, projectRoot, addr, basePath, uiDistDir string) *Server {
	reg, _ := config.LoadRegistry(filepath.Join(projectRoot, ".skillshare"))
	if reg == nil {
		reg = &config.Registry{}
	}
	s := &Server{
		cfg:         cfg,
		registry:    reg,
		addr:        addr,
		mux:         http.NewServeMux(),
		basePath:    NormalizeBasePath(basePath),
		projectRoot: projectRoot,
		projectCfg:  projectCfg,
		uiDistDir:   uiDistDir,
	}
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	s.wrapBasePath()
	return s
}

// IsProjectMode returns true when serving a project-scoped dashboard
func (s *Server) IsProjectMode() bool {
	return s.projectRoot != ""
}

// cloneTargets returns a shallow copy of the Targets map.
// Callers must hold s.mu (RLock or Lock).
func (s *Server) cloneTargets() map[string]config.TargetConfig {
	targets := make(map[string]config.TargetConfig, len(s.cfg.Targets))
	for k, v := range s.cfg.Targets {
		targets[k] = v
	}
	return targets
}

// parseOpts returns install.ParseOptions with GitLabHosts from the current config.
// In project mode, project config is used unconditionally (not a fallback to global).
func (s *Server) parseOpts() install.ParseOptions {
	if s.IsProjectMode() && s.projectCfg != nil {
		return install.ParseOptions{GitLabHosts: s.projectCfg.EffectiveGitLabHosts()}
	}
	return install.ParseOptions{GitLabHosts: s.cfg.EffectiveGitLabHosts()}
}

// configPath returns the config file path for the current mode
func (s *Server) configPath() string {
	if s.IsProjectMode() {
		return config.ProjectConfigPath(s.projectRoot)
	}
	return config.ConfigPath()
}

// saveConfig persists the config for the current mode
func (s *Server) saveConfig() error {
	if s.IsProjectMode() {
		return s.projectCfg.Save(s.projectRoot)
	}
	return s.cfg.Save()
}

// saveAndReloadConfig persists config to disk then reloads it into memory.
// Callers must hold s.mu.
func (s *Server) saveAndReloadConfig() error {
	if err := s.saveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if err := s.reloadConfig(); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	return nil
}

// reloadConfig reloads the config for the current mode
func (s *Server) reloadConfig() error {
	if s.IsProjectMode() {
		pcfg, err := config.LoadProject(s.projectRoot)
		if err != nil {
			return err
		}
		s.projectCfg = pcfg
		targets, err := config.ResolveProjectTargets(s.projectRoot, pcfg)
		if err != nil {
			return err
		}
		s.cfg.Targets = targets
		if reg, err := config.LoadRegistry(filepath.Join(s.projectRoot, ".skillshare")); err == nil {
			s.registry = reg
		}
		return nil
	}
	newCfg, err := config.Load()
	if err != nil {
		return err
	}
	s.cfg = newCfg
	if reg, err := config.LoadRegistry(filepath.Dir(config.ConfigPath())); err == nil {
		s.registry = reg
	}
	return nil
}

func (s *Server) refreshConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadConfig()
}

func (s *Server) shouldAutoReloadConfig(path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	if path == "/api/health" {
		return false
	}
	// Keep config editor recoverable even if config file is temporarily invalid.
	if path == "/api/config" {
		return false
	}
	return true
}

func (s *Server) withConfigAutoReload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.shouldAutoReloadConfig(r.URL.Path) {
			if err := s.refreshConfig(); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to reload config: "+err.Error())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server with graceful shutdown on SIGTERM/SIGINT.
func (s *Server) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.StartWithContext(ctx)
}

// SetOnReady sets a callback invoked after the listener is bound but before
// serving begins.  Used to open the browser only after the port is confirmed
// available.
func (s *Server) SetOnReady(fn func()) {
	s.onReady = fn
}

// StartWithContext starts the HTTP server and shuts down gracefully when ctx is cancelled.
func (s *Server) StartWithContext(ctx context.Context) error {
	s.startTime = time.Now()

	// Bind the port first so callers know immediately if it's in use.
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	if s.basePath != "" {
		fmt.Printf("Skillshare UI running at http://%s%s/\n", s.addr, s.basePath)
	} else {
		fmt.Printf("Skillshare UI running at http://%s\n", s.addr)
	}

	if s.onReady != nil {
		s.onReady()
	}

	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	fmt.Println("\nShutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	fmt.Println("Server stopped gracefully")
	return nil
}

// registerRoutes sets up all API and static file routes
func (s *Server) registerRoutes() {
	// Health check
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Overview
	s.mux.HandleFunc("GET /api/overview", s.handleOverview)

	// Skills
	s.mux.HandleFunc("GET /api/skills", s.handleListSkills)
	s.mux.HandleFunc("GET /api/skills/templates", s.handleGetTemplates)
	s.mux.HandleFunc("POST /api/skills", s.handleCreateSkill)
	s.mux.HandleFunc("GET /api/skills/{name}", s.handleGetSkill)
	s.mux.HandleFunc("GET /api/skills/{name}/files/{filepath...}", s.handleGetSkillFile)
	s.mux.HandleFunc("DELETE /api/skills/{name}", s.handleUninstallSkill)

	// Targets
	s.mux.HandleFunc("GET /api/targets", s.handleListTargets)
	s.mux.HandleFunc("POST /api/targets", s.handleAddTarget)
	s.mux.HandleFunc("PATCH /api/targets/{name}", s.handleUpdateTarget)
	s.mux.HandleFunc("DELETE /api/targets/{name}", s.handleRemoveTarget)

	// Sync matrix
	s.mux.HandleFunc("GET /api/sync-matrix", s.handleSyncMatrix)
	s.mux.HandleFunc("POST /api/sync-matrix/preview", s.handleSyncMatrixPreview)

	// Sync
	s.mux.HandleFunc("POST /api/sync", s.handleSync)
	s.mux.HandleFunc("GET /api/diff/stream", s.handleDiffStream)
	s.mux.HandleFunc("GET /api/diff", s.handleDiff)

	// Collect
	s.mux.HandleFunc("GET /api/collect/scan", s.handleCollectScan)
	s.mux.HandleFunc("POST /api/collect", s.handleCollect)

	// Hub
	s.mux.HandleFunc("GET /api/hub/index", s.handleHubIndex)
	s.mux.HandleFunc("GET /api/hub/saved", s.handleGetHubSaved)
	s.mux.HandleFunc("PUT /api/hub/saved", s.handlePutHubSaved)
	s.mux.HandleFunc("POST /api/hub/saved", s.handlePostHubSaved)
	s.mux.HandleFunc("DELETE /api/hub/saved/{label}", s.handleDeleteHubSaved)

	// Search & Install
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("POST /api/discover", s.handleDiscover)
	s.mux.HandleFunc("POST /api/install", s.handleInstall)
	s.mux.HandleFunc("POST /api/install/batch", s.handleInstallBatch)
	s.mux.HandleFunc("POST /api/uninstall/batch", s.handleBatchUninstall)

	// Update & Check
	s.mux.HandleFunc("POST /api/update", s.handleUpdate)
	s.mux.HandleFunc("GET /api/update/stream", s.handleUpdateStream)
	s.mux.HandleFunc("GET /api/check/stream", s.handleCheckStream)
	s.mux.HandleFunc("GET /api/check", s.handleCheck)

	// Repo uninstall
	s.mux.HandleFunc("DELETE /api/repos/{name}", s.handleUninstallRepo)

	// Version check
	s.mux.HandleFunc("GET /api/version", s.handleVersionCheck)

	// Doctor (health check)
	s.mux.HandleFunc("GET /api/doctor", s.handleDoctor)

	// Backups
	s.mux.HandleFunc("GET /api/backups", s.handleListBackups)
	s.mux.HandleFunc("POST /api/backup", s.handleCreateBackup)
	s.mux.HandleFunc("POST /api/backup/cleanup", s.handleCleanupBackups)
	s.mux.HandleFunc("POST /api/restore", s.handleRestore)
	s.mux.HandleFunc("POST /api/restore/validate", s.handleValidateRestore)

	// Trash
	s.mux.HandleFunc("GET /api/trash", s.handleListTrash)
	s.mux.HandleFunc("POST /api/trash/{name}/restore", s.handleRestoreTrash)
	s.mux.HandleFunc("DELETE /api/trash/{name}", s.handleDeleteTrash)
	s.mux.HandleFunc("POST /api/trash/empty", s.handleEmptyTrash)

	// Extras
	s.mux.HandleFunc("GET /api/extras", s.handleExtras)
	s.mux.HandleFunc("GET /api/extras/diff", s.handleExtrasDiff)
	s.mux.HandleFunc("POST /api/extras", s.handleExtrasCreate)
	s.mux.HandleFunc("POST /api/extras/sync", s.handleExtrasSync)
	s.mux.HandleFunc("PATCH /api/extras/{name}/mode", s.handleExtrasMode)
	s.mux.HandleFunc("DELETE /api/extras/{name}", s.handleExtrasDelete)

	// Git
	s.mux.HandleFunc("GET /api/git/status", s.handleGitStatus)
	s.mux.HandleFunc("GET /api/git/branches", s.handleGitBranches)
	s.mux.HandleFunc("POST /api/git/checkout", s.handleGitCheckout)
	s.mux.HandleFunc("POST /api/push", s.handlePush)
	s.mux.HandleFunc("POST /api/pull", s.handlePull)

	// Audit
	s.mux.HandleFunc("GET /api/audit/stream", s.handleAuditStream)
	s.mux.HandleFunc("GET /api/audit/rules/compiled", s.handleGetCompiledRules)
	s.mux.HandleFunc("POST /api/audit/rules/toggle", s.handleToggleRule)
	s.mux.HandleFunc("POST /api/audit/rules/reset", s.handleResetRules)
	s.mux.HandleFunc("GET /api/audit/rules", s.handleGetAuditRules)
	s.mux.HandleFunc("PUT /api/audit/rules", s.handlePutAuditRules)
	s.mux.HandleFunc("POST /api/audit/rules", s.handleInitAuditRules)
	s.mux.HandleFunc("GET /api/audit", s.handleAuditAll)
	s.mux.HandleFunc("GET /api/audit/{name}", s.handleAuditSkill)

	// Log
	s.mux.HandleFunc("GET /api/log", s.handleListLog)
	s.mux.HandleFunc("GET /api/log/stats", s.handleLogStats)
	s.mux.HandleFunc("DELETE /api/log", s.handleClearLog)

	// Config
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /api/config/available-targets", s.handleAvailableTargets)

	// Skillignore
	s.mux.HandleFunc("GET /api/skillignore", s.handleGetSkillignore)
	s.mux.HandleFunc("PUT /api/skillignore", s.handlePutSkillignore)

	// SPA fallback — must be last
	if s.uiDistDir != "" {
		s.mux.Handle("/", spaHandlerFromDisk(s.uiDistDir, s.basePath))
	} else {
		s.mux.Handle("/", uiPlaceholderHandler())
	}
}

// handleHealth responds with status, version, and uptime
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := int64(0)
	if !s.startTime.IsZero() {
		uptime = int64(time.Since(s.startTime).Seconds())
	}
	writeJSON(w, map[string]any{
		"status":         "ok",
		"version":        version.Version,
		"uptime_seconds": uptime,
	})
}
