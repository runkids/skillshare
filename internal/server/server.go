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
}

// New creates a new Server
func New(cfg *config.Config, addr string) *Server {
	s := &Server{
		cfg:  cfg,
		addr: addr,
		mux:  http.NewServeMux(),
	}
	s.registerRoutes()
	return s
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

	// Update
	s.mux.HandleFunc("POST /api/update", s.handleUpdate)

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
