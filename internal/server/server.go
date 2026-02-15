package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/version"
)

// Server holds the HTTP server state
type Server struct {
	cfg     *config.Config
	addr    string
	mux     *http.ServeMux
	handler http.Handler
	mu      sync.Mutex // protects write operations and config reloads

	startTime time.Time // for uptime reporting in health check

	// Project mode fields (empty/nil for global mode)
	projectRoot string
	projectCfg  *config.ProjectConfig
}

// New creates a new Server for global mode
func New(cfg *config.Config, addr string) *Server {
	s := &Server{
		cfg:  cfg,
		addr: addr,
		mux:  http.NewServeMux(),
	}
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	return s
}

// NewProject creates a new Server for project mode
func NewProject(cfg *config.Config, projectCfg *config.ProjectConfig, projectRoot, addr string) *Server {
	s := &Server{
		cfg:         cfg,
		addr:        addr,
		mux:         http.NewServeMux(),
		projectRoot: projectRoot,
		projectCfg:  projectCfg,
	}
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	return s
}

// IsProjectMode returns true when serving a project-scoped dashboard
func (s *Server) IsProjectMode() bool {
	return s.projectRoot != ""
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
		return nil
	}
	newCfg, err := config.Load()
	if err != nil {
		return err
	}
	s.cfg = newCfg
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

// StartWithContext starts the HTTP server and shuts down gracefully when ctx is cancelled.
func (s *Server) StartWithContext(ctx context.Context) error {
	s.startTime = time.Now()

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Skillshare UI running at http://%s\n", s.addr)
		errCh <- srv.ListenAndServe()
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
	s.mux.HandleFunc("GET /api/skills/{name}", s.handleGetSkill)
	s.mux.HandleFunc("GET /api/skills/{name}/files/{filepath...}", s.handleGetSkillFile)
	s.mux.HandleFunc("DELETE /api/skills/{name}", s.handleUninstallSkill)

	// Targets
	s.mux.HandleFunc("GET /api/targets", s.handleListTargets)
	s.mux.HandleFunc("POST /api/targets", s.handleAddTarget)
	s.mux.HandleFunc("PATCH /api/targets/{name}", s.handleUpdateTarget)
	s.mux.HandleFunc("DELETE /api/targets/{name}", s.handleRemoveTarget)

	// Sync
	s.mux.HandleFunc("POST /api/sync", s.handleSync)
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

	// Update & Check
	s.mux.HandleFunc("POST /api/update", s.handleUpdate)
	s.mux.HandleFunc("GET /api/check", s.handleCheck)

	// Repo uninstall
	s.mux.HandleFunc("DELETE /api/repos/{name}", s.handleUninstallRepo)

	// Version check
	s.mux.HandleFunc("GET /api/version", s.handleVersionCheck)

	// Backups
	s.mux.HandleFunc("GET /api/backups", s.handleListBackups)
	s.mux.HandleFunc("POST /api/backup", s.handleCreateBackup)
	s.mux.HandleFunc("POST /api/backup/cleanup", s.handleCleanupBackups)
	s.mux.HandleFunc("POST /api/restore", s.handleRestore)

	// Trash
	s.mux.HandleFunc("GET /api/trash", s.handleListTrash)
	s.mux.HandleFunc("POST /api/trash/{name}/restore", s.handleRestoreTrash)
	s.mux.HandleFunc("DELETE /api/trash/{name}", s.handleDeleteTrash)
	s.mux.HandleFunc("POST /api/trash/empty", s.handleEmptyTrash)

	// Git
	s.mux.HandleFunc("GET /api/git/status", s.handleGitStatus)
	s.mux.HandleFunc("POST /api/push", s.handlePush)
	s.mux.HandleFunc("POST /api/pull", s.handlePull)

	// Audit
	s.mux.HandleFunc("GET /api/audit/rules", s.handleGetAuditRules)
	s.mux.HandleFunc("PUT /api/audit/rules", s.handlePutAuditRules)
	s.mux.HandleFunc("POST /api/audit/rules", s.handleInitAuditRules)
	s.mux.HandleFunc("GET /api/audit", s.handleAuditAll)
	s.mux.HandleFunc("GET /api/audit/{name}", s.handleAuditSkill)

	// Log
	s.mux.HandleFunc("GET /api/log", s.handleListLog)
	s.mux.HandleFunc("DELETE /api/log", s.handleClearLog)

	// Config
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /api/config/available-targets", s.handleAvailableTargets)

	// SPA fallback — must be last
	s.mux.Handle("/", spaHandler())
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
