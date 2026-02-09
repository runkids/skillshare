package server

import (
	"fmt"
	"net/http"
	"sync"

	"skillshare/internal/config"
)

// Server holds the HTTP server state
type Server struct {
	cfg  *config.Config
	addr string
	mux  *http.ServeMux
	mu   sync.Mutex // protects write operations (sync, install, uninstall, config)

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

// Start starts the HTTP server (blocking)
func (s *Server) Start() error {
	fmt.Printf("Skillshare UI running at http://%s\n", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
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
	s.mux.HandleFunc("DELETE /api/targets/{name}", s.handleRemoveTarget)

	// Sync
	s.mux.HandleFunc("POST /api/sync", s.handleSync)
	s.mux.HandleFunc("GET /api/diff", s.handleDiff)

	// Collect
	s.mux.HandleFunc("GET /api/collect/scan", s.handleCollectScan)
	s.mux.HandleFunc("POST /api/collect", s.handleCollect)

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

	// Git
	s.mux.HandleFunc("GET /api/git/status", s.handleGitStatus)
	s.mux.HandleFunc("POST /api/push", s.handlePush)
	s.mux.HandleFunc("POST /api/pull", s.handlePull)

	// Config
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /api/config/available-targets", s.handleAvailableTargets)

	// SPA fallback — must be last
	s.mux.Handle("/", spaHandler())
}

// handleHealth responds with a simple OK
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}
