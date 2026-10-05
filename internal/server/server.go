package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	cfg         *config.Config
	skillsStore *install.MetadataStore
	agentsStore *install.MetadataStore
	addr        string
	mux         *http.ServeMux
	handler     http.Handler
	mu          sync.RWMutex // protects config: Lock for writes/reloads, RLock for reads

	startTime time.Time // for uptime reporting in health check

	// Project mode fields (empty/nil for global mode)
	projectRoot string
	projectCfg  *config.ProjectConfig
	// unresolvedTargets are project targets left out of cfg.Targets because
	// they cannot resolve (no path); sync reports them as failed.
	unresolvedTargets map[string]error

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
	skillsStore, _ := install.LoadMetadataWithMigration(cfg.EffectiveSkillsSource(), "")
	if skillsStore == nil {
		skillsStore = install.NewMetadataStore()
	}
	agentsStore, _ := install.LoadMetadataWithMigration(cfg.EffectiveAgentsSource(), "agent")
	if agentsStore == nil {
		agentsStore = install.NewMetadataStore()
	}
	s := &Server{
		cfg:         cfg,
		skillsStore: skillsStore,
		agentsStore: agentsStore,
		addr:        addr,
		mux:         http.NewServeMux(),
		basePath:    NormalizeBasePath(basePath),
		uiDistDir:   uiDistDir,
	}
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	s.wrapBasePath()
	return s
}

// NewProject creates a new Server for project mode.
// uiDistDir, when non-empty, serves UI from disk instead of the embedded SPA.
func NewProject(cfg *config.Config, projectCfg *config.ProjectConfig, projectRoot, addr, basePath, uiDistDir string) *Server {
	skillsDir := projectCfg.EffectiveSkillsSource(projectRoot)
	agentsDir := projectCfg.EffectiveAgentsSource(projectRoot)
	skillsStore, _ := install.LoadMetadataWithMigration(skillsDir, "")
	if skillsStore == nil {
		skillsStore = install.NewMetadataStore()
	}
	agentsStore, _ := install.LoadMetadataWithMigration(agentsDir, "agent")
	if agentsStore == nil {
		agentsStore = install.NewMetadataStore()
	}
	s := &Server{
		cfg:         cfg,
		skillsStore: skillsStore,
		agentsStore: agentsStore,
		addr:        addr,
		mux:         http.NewServeMux(),
		basePath:    NormalizeBasePath(basePath),
		projectRoot: projectRoot,
		projectCfg:  projectCfg,
		uiDistDir:   uiDistDir,
	}
	_, s.unresolvedTargets = config.ResolveValidProjectTargets(projectRoot, projectCfg)
	s.registerRoutes()
	s.handler = s.withConfigAutoReload(s.mux)
	s.wrapBasePath()
	return s
}

// IsProjectMode returns true when serving a project-scoped dashboard
func (s *Server) IsProjectMode() bool {
	return s.projectRoot != ""
}

// skillEntry returns a copy of the skills metadata entry for relPath, or nil.
// It takes s.mu.RLock, so the caller must not hold s.mu.
func (s *Server) skillEntry(relPath string) *install.MetadataEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyMetadataEntry(s.skillsStore.GetByPath(relPath))
}

// agentEntry returns a copy of the agents metadata entry for key, or nil.
// It takes s.mu.RLock, so the caller must not hold s.mu.
func (s *Server) agentEntry(key string) *install.MetadataEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyMetadataEntry(s.agentsStore.GetByPath(key))
}

// copyMetadataEntry returns a shallow copy so callers can read the entry after
// s.mu is released while other requests replace or mutate the store.
func copyMetadataEntry(e *install.MetadataEntry) *install.MetadataEntry {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

// skillsSource returns the skills source directory for the current mode.
// Caller must hold s.mu (RLock or Lock) when accessing s.cfg.
func (s *Server) skillsSource() string {
	if s.IsProjectMode() {
		return s.projectCfg.EffectiveSkillsSource(s.projectRoot)
	}
	return s.cfg.EffectiveSkillsSource()
}

// agentsSource returns the agents source directory for the current mode.
// Caller must hold s.mu (RLock or Lock) when accessing s.cfg.
func (s *Server) agentsSource() string {
	if s.IsProjectMode() {
		return s.projectCfg.EffectiveAgentsSource(s.projectRoot)
	}
	return s.cfg.EffectiveAgentsSource()
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

// parseOpts returns install.ParseOptions from the current config.
// In project mode, project config is used unconditionally (not a fallback to global).
func (s *Server) parseOpts() install.ParseOptions {
	if s.IsProjectMode() && s.projectCfg != nil {
		return install.ParseOptions{
			GitLabHosts: s.projectCfg.EffectiveGitLabHosts(),
			AzureHosts:  s.projectCfg.EffectiveAzureHosts(),
			CNBHosts:    s.projectCfg.EffectiveCNBHosts(),
			GiteaHosts:  s.projectCfg.EffectiveGiteaHosts(),
		}
	}
	return install.ParseOptions{
		GitLabHosts: s.cfg.EffectiveGitLabHosts(),
		AzureHosts:  s.cfg.EffectiveAzureHosts(),
		CNBHosts:    s.cfg.EffectiveCNBHosts(),
		GiteaHosts:  s.cfg.EffectiveGiteaHosts(),
	}
}

// gitignoreDir returns the directory containing the managed .gitignore.
// In project mode this is .skillshare/ (entries are "skills/<name>/");
// in global mode this is the source skill directory.
func (s *Server) gitignoreDir() string {
	if s.IsProjectMode() {
		dir, _ := config.ProjectGitignoreTarget(s.projectRoot, s.skillsSource())
		return dir
	}
	return s.cfg.EffectiveSkillsSource()
}

func (s *Server) projectGitignorePrefix() string {
	_, prefix := config.ProjectGitignoreTarget(s.projectRoot, s.skillsSource())
	return prefix
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
		s.cfg.Targets, s.unresolvedTargets = config.ResolveValidProjectTargets(s.projectRoot, pcfg)
		skillsDir := pcfg.EffectiveSkillsSource(s.projectRoot)
		agentsDir := pcfg.EffectiveAgentsSource(s.projectRoot)
		s.cfg.Source = skillsDir
		s.cfg.AgentsSource = agentsDir
		if st, err := install.LoadMetadata(skillsDir); err == nil {
			s.skillsStore = st
		}
		if st, err := install.LoadMetadata(agentsDir); err == nil {
			s.agentsStore = st
		}
		return nil
	}
	newCfg, err := config.Load()
	if err != nil {
		return err
	}
	s.cfg = newCfg
	if st, err := install.LoadMetadata(newCfg.EffectiveSkillsSource()); err == nil {
		s.skillsStore = st
	}
	if st, err := install.LoadMetadata(newCfg.EffectiveAgentsSource()); err == nil {
		s.agentsStore = st
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

	// Resources (skills + agents)
	s.mux.HandleFunc("GET /api/resources", s.handleListSkills)
	s.mux.HandleFunc("GET /api/resources/templates", s.handleGetTemplates)
	s.mux.HandleFunc("GET /api/resources/templates/preview", s.handlePreviewSkill)
	s.mux.HandleFunc("POST /api/resources", s.handleCreateSkill)
	s.mux.HandleFunc("GET /api/resources/{name}", s.handleGetSkill)
	s.mux.HandleFunc("GET /api/resources/{name}/files/{filepath...}", s.handleGetSkillFile)
	s.mux.HandleFunc("PUT /api/resources/{name}/content", s.handlePutSkillContent)
	s.mux.HandleFunc("PATCH /api/resources/{name}/source", s.handlePatchSkillSource)
	s.mux.HandleFunc("POST /api/resources/{name}/open-in-editor", s.handleOpenSkillInEditor)
	s.mux.HandleFunc("POST /api/resources/{name}/disable", s.handleDisableSkill)
	s.mux.HandleFunc("POST /api/resources/{name}/enable", s.handleEnableSkill)
	s.mux.HandleFunc("DELETE /api/resources/{name}", s.handleUninstallSkill)
	s.mux.HandleFunc("POST /api/resources/batch/targets", s.handleBatchSetTargets)
	s.mux.HandleFunc("POST /api/resources/batch/toggle", s.handleBatchToggleSkills)
	s.mux.HandleFunc("PATCH /api/resources/{name}/targets", s.handleSetSkillTargets)

	// Targets
	s.mux.HandleFunc("GET /api/targets", s.handleListTargets)
	s.mux.HandleFunc("POST /api/targets", s.handleAddTarget)
	s.mux.HandleFunc("PATCH /api/targets/{name}", s.handleUpdateTarget)
	s.mux.HandleFunc("DELETE /api/targets/{name}", s.handleRemoveTarget)
	s.mux.HandleFunc("GET /api/targets/{name}/skills-off-preview", s.handleSkillsOffPreview)
	s.mux.HandleFunc("GET /api/targets/{name}/instructions", s.handleGetTargetInstructions)
	s.mux.HandleFunc("PUT /api/targets/{name}/instructions", s.handlePutTargetInstructions)
	s.mux.HandleFunc("POST /api/targets/{name}/instructions/convert", s.handleConvertTargetInstructions)
	s.mux.HandleFunc("PUT /api/targets/{name}/instructions/setup", s.handlePutTargetInstructionsSetup)
	s.mux.HandleFunc("DELETE /api/targets/{name}/instructions/setup", s.handleDeleteTargetInstructionsSetup)
	s.mux.HandleFunc("GET /api/targets/{name}/files", s.handleListTargetFiles)
	s.mux.HandleFunc("POST /api/targets/{name}/files", s.handleAddTargetFile)
	s.mux.HandleFunc("DELETE /api/targets/{name}/files", s.handleRemoveTargetFile)
	s.mux.HandleFunc("GET /api/targets/{name}/files/content", s.handleGetTargetFileContent)
	s.mux.HandleFunc("PUT /api/targets/{name}/files/content", s.handlePutTargetFileContent)
	s.mux.HandleFunc("GET /api/targets/{name}/omp-extensions", s.requireLocalPlugin(s.handleOMPExtensions))
	s.mux.HandleFunc("POST /api/targets/{name}/omp-extensions/preview", s.requireLocalPlugin(s.handleOMPExtensionsPreview))
	s.mux.HandleFunc("POST /api/targets/{name}/omp-extensions/apply", s.requireLocalPlugin(s.handleOMPExtensionsApply))
	s.mux.HandleFunc("GET /api/targets/{name}/pi-extensions", s.requireLocalPlugin(s.handlePiExtensions))
	s.mux.HandleFunc("POST /api/targets/{name}/pi-extensions/preview", s.requireLocalPlugin(s.handlePiExtensionsPreview))
	s.mux.HandleFunc("POST /api/targets/{name}/pi-extensions/apply", s.requireLocalPlugin(s.handlePiExtensionsApply))

	// Instruction files: shared files (tool targets are global only) and the project AGENTS.md
	s.mux.HandleFunc("GET /api/instructions", s.handleListSharedInstructions)
	s.mux.HandleFunc("POST /api/instructions", s.handleCreateSharedInstructions)
	s.mux.HandleFunc("POST /api/instructions/assign", s.requireGlobalInstructions(s.handleAssignSharedInstructions))
	s.mux.HandleFunc("GET /api/instructions/{name}/content", s.handleGetSharedInstructionsContent)
	s.mux.HandleFunc("PUT /api/instructions/{name}/content", s.handlePutSharedInstructionsContent)
	s.mux.HandleFunc("POST /api/instructions/{name}/restore", s.requireGlobalInstructions(s.handleRestoreSharedInstructions))
	s.mux.HandleFunc("POST /api/instructions/{name}/resolve", s.handleResolveSharedInstructions)
	s.mux.HandleFunc("GET /api/instructions/{name}/restore-preview", s.requireGlobalInstructions(s.handleSharedInstructionsRestorePreview))
	s.mux.HandleFunc("PUT /api/instructions/{name}/targets/{target}/mode", s.requireGlobalInstructions(s.handlePutSharedInstructionsMode))
	s.mux.HandleFunc("POST /api/instructions/{name}/locations", s.handleAddSharedInstructionsLocation)
	s.mux.HandleFunc("DELETE /api/instructions/{name}/locations", s.handleDeleteSharedInstructionsLocation)
	s.mux.HandleFunc("PUT /api/instructions/{name}/locations/mode", s.handlePutSharedInstructionsLocationMode)
	s.mux.HandleFunc("GET /api/instructions/{name}/locations/restore-preview", s.handleSharedInstructionsLocationRestorePreview)
	s.mux.HandleFunc("GET /api/instructions/project", s.requireProjectInstructions(s.handleGetProjectInstructions))
	s.mux.HandleFunc("PUT /api/instructions/project", s.requireProjectInstructions(s.handlePutProjectInstructions))
	s.mux.HandleFunc("POST /api/instructions/project/shim", s.requireProjectInstructions(s.handleProjectInstructionsShim))

	// Projects (global config only)
	s.mux.HandleFunc("GET /api/projects", s.requireGlobalProjects(s.handleListProjects))
	s.mux.HandleFunc("PUT /api/projects", s.requireGlobalProjects(s.handleSaveProject))
	s.mux.HandleFunc("DELETE /api/projects", s.requireGlobalProjects(s.handleRemoveProject))
	s.mux.HandleFunc("POST /api/projects/convert", s.requireGlobalProjects(s.handleConvertProject))

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
	s.mux.HandleFunc("GET /api/hub/drafts", s.handleHubDrafts)
	s.mux.HandleFunc("POST /api/hub/drafts", s.handleHubDrafts)
	s.mux.HandleFunc("GET /api/hub/drafts/candidates", s.handleHubDraftCandidates)
	s.mux.HandleFunc("POST /api/hub/drafts/import", s.handleHubDraftImport)
	s.mux.HandleFunc("GET /api/hub/drafts/{id}", s.handleHubDraft)
	s.mux.HandleFunc("PUT /api/hub/drafts/{id}", s.handleHubDraft)
	s.mux.HandleFunc("DELETE /api/hub/drafts/{id}", s.handleHubDraft)
	s.mux.HandleFunc("POST /api/hub/drafts/{id}/export", s.handleHubDraftExport)
	s.mux.HandleFunc("GET /api/hub/index", s.handleHubIndex)
	s.mux.HandleFunc("POST /api/hub/refs", s.handleHubRefs)
	s.mux.HandleFunc("GET /api/hub/saved", s.handleGetHubSaved)
	s.mux.HandleFunc("PUT /api/hub/saved", s.handlePutHubSaved)
	s.mux.HandleFunc("POST /api/hub/saved", s.handlePostHubSaved)
	s.mux.HandleFunc("DELETE /api/hub/saved/{label}", s.handleDeleteHubSaved)

	// Search & Install
	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/preview", s.handlePreview)
	s.mux.HandleFunc("POST /api/discover", s.handleDiscover)
	s.mux.HandleFunc("POST /api/install", s.handleInstall)
	s.mux.HandleFunc("POST /api/install/batch", s.handleInstallBatch)
	s.mux.HandleFunc("POST /api/uninstall/batch", s.handleBatchUninstall)

	// Update & Check
	s.mux.HandleFunc("POST /api/update", s.handleUpdate)
	s.mux.HandleFunc("GET /api/update/stream", s.handleUpdateStream)
	s.mux.HandleFunc("GET /api/update/missing-tracked-repos", s.handleMissingTrackedRepos)
	s.mux.HandleFunc("POST /api/update/rehydrate", s.handleRehydrateTrackedRepos)
	s.mux.HandleFunc("GET /api/check/stream", s.handleCheckStream)
	s.mux.HandleFunc("GET /api/check", s.handleCheck)

	// Repo uninstall
	s.mux.HandleFunc("DELETE /api/repos/{name}", s.handleUninstallRepo)

	// Version check / app lifecycle
	s.mux.HandleFunc("GET /api/version", s.handleVersionCheck)
	s.mux.HandleFunc("POST /api/upgrade", s.handleUpgrade)
	s.mux.HandleFunc("POST /api/restart", s.handleRestart)

	// Doctor (health check)
	s.mux.HandleFunc("GET /api/doctor", s.handleDoctor)

	// Backups
	s.mux.HandleFunc("GET /api/backups", s.handleListBackups)
	s.mux.HandleFunc("DELETE /api/backups", s.handleDeleteAllBackups)
	s.mux.HandleFunc("DELETE /api/backups/{timestamp}", s.handleDeleteBackup)
	s.mux.HandleFunc("POST /api/backup", s.handleCreateBackup)
	s.mux.HandleFunc("POST /api/backup/cleanup", s.handleCleanupBackups)
	s.mux.HandleFunc("POST /api/restore", s.handleRestore)
	s.mux.HandleFunc("POST /api/restore/validate", s.handleValidateRestore)

	// File history (backups of single files skillshare rewrote)
	s.mux.HandleFunc("GET /api/file-backups", s.handleListFileBackups)
	s.mux.HandleFunc("GET /api/file-backups/versions", s.handleFileBackupVersions)
	s.mux.HandleFunc("GET /api/file-backups/version", s.handleFileBackupVersion)
	s.mux.HandleFunc("POST /api/file-backups/restore", s.handleRestoreFileBackup)

	// Trash
	s.mux.HandleFunc("GET /api/trash", s.handleListTrash)
	s.mux.HandleFunc("POST /api/trash/{name}/restore", s.handleRestoreTrash)
	s.mux.HandleFunc("DELETE /api/trash/{name}", s.handleDeleteTrash)
	s.mux.HandleFunc("POST /api/trash/empty", s.handleEmptyTrash)

	// Extras
	s.mux.HandleFunc("GET /api/plugins", s.requireLocalPlugin(s.handlePluginList))
	s.mux.HandleFunc("GET /api/plugins/{name}/files", s.requireLocalPlugin(s.handlePluginFiles))
	s.mux.HandleFunc("GET /api/plugins/{name}/files/{filepath...}", s.requireLocalPlugin(s.handlePluginFile))
	s.mux.HandleFunc("POST /api/plugins/discover", s.requireLocalPlugin(s.handlePluginDiscover))
	s.mux.HandleFunc("POST /api/plugins/preview", s.requireLocalPlugin(s.handlePluginPreview))
	s.mux.HandleFunc("POST /api/plugins/apply", s.requireLocalPlugin(s.handlePluginApply))
	s.mux.HandleFunc("GET /api/mcp", s.requireLocalMCP(s.handleMCPList))
	s.mux.HandleFunc("POST /api/mcp", s.requireLocalMCP(s.handleMCPConfigure))
	s.mux.HandleFunc("GET /api/mcp/check", s.requireLocalMCP(s.handleMCPCheck))
	s.mux.HandleFunc("POST /api/mcp/probe", s.requireLocalMCP(s.handleMCPProbe))
	s.mux.HandleFunc("POST /api/mcp/preview", s.requireLocalMCP(s.handleMCPPreview))
	s.mux.HandleFunc("POST /api/mcp/render", s.requireLocalMCP(s.handleMCPRender))
	s.mux.HandleFunc("POST /api/mcp/import", s.requireLocalMCP(s.handleMCPImport))
	s.mux.HandleFunc("POST /api/mcp/restore", s.requireLocalMCP(s.handleMCPRestore))
	s.mux.HandleFunc("GET /api/hooks", s.requireLocalHooks(s.handleHooksList))
	s.mux.HandleFunc("POST /api/hooks", s.requireLocalHooks(s.handleHooksConfigure))
	s.mux.HandleFunc("POST /api/hooks/preview", s.requireLocalHooks(s.handleHooksPreview))
	s.mux.HandleFunc("POST /api/hooks/render", s.requireLocalHooks(s.handleHooksRender))
	s.mux.HandleFunc("POST /api/hooks/import", s.requireLocalHooks(s.handleHooksImport))
	s.mux.HandleFunc("POST /api/hooks/restore", s.requireLocalHooks(s.handleHooksRestore))
	s.mux.HandleFunc("GET /api/hooks/catalog", s.requireLocalHooks(s.handleHooksCatalog))
	s.mux.HandleFunc("GET /api/extras", s.handleExtras)
	s.mux.HandleFunc("GET /api/extras/memory/notes", s.handleMemoryNotes)
	s.mux.HandleFunc("POST /api/extras/memory/init", s.handleMemoryInit)
	s.mux.HandleFunc("PUT /api/extras/memory/index", s.handleMemoryIndex)
	s.mux.HandleFunc("POST /api/extras/memory/move", s.handleMemoryMove)
	s.mux.HandleFunc("GET /api/extras/memory/notes/content", s.handleMemoryContent)
	s.mux.HandleFunc("PUT /api/extras/memory/notes/content", s.handleMemoryWrite)
	s.mux.HandleFunc("DELETE /api/extras/memory/notes/content", s.handleMemoryDelete)
	s.mux.HandleFunc("GET /api/extras/memory/guidance", s.handleMemoryGuidance)
	s.mux.HandleFunc("POST /api/extras/memory/guidance/plan", s.handleMemoryGuidancePlan)
	s.mux.HandleFunc("POST /api/extras/memory/guidance/apply", s.handleMemoryGuidanceApply)
	s.mux.HandleFunc("GET /api/extras/extensions", s.handleExtrasExtensions)
	s.mux.HandleFunc("GET /api/extras/diff", s.handleExtrasDiff)
	s.mux.HandleFunc("POST /api/extras", s.handleExtrasCreate)
	s.mux.HandleFunc("POST /api/extras/sync", s.handleExtrasSync)
	s.mux.HandleFunc("PATCH /api/extras/{name}/mode", s.handleExtrasMode)
	s.mux.HandleFunc("DELETE /api/extras/{name}", s.handleExtrasDelete)
	s.mux.HandleFunc("POST /api/extras/{name}/targets", s.handleExtrasAddTarget)
	s.mux.HandleFunc("DELETE /api/extras/{name}/targets", s.handleExtrasRemoveTarget)

	// Extensions (transform extensions management)
	s.mux.HandleFunc("GET /api/extensions", s.handleExtensionsList)
	s.mux.HandleFunc("POST /api/extensions/install", s.handleExtensionsInstall)
	s.mux.HandleFunc("POST /api/extensions/open", s.handleExtensionsOpen)
	s.mux.HandleFunc("DELETE /api/extensions/{name}", s.handleExtensionsRemove)

	// Git
	s.mux.HandleFunc("GET /api/git/status", s.handleGitStatus)
	s.mux.HandleFunc("POST /api/git/root", s.handleSetGitRoot)
	s.mux.HandleFunc("GET /api/git/branches", s.handleGitBranches)
	s.mux.HandleFunc("POST /api/git/checkout", s.handleGitCheckout)
	s.mux.HandleFunc("POST /api/git/commit", s.handleGitCommit)
	s.mux.HandleFunc("POST /api/git/discard", s.handleGitDiscard)
	s.mux.HandleFunc("POST /api/git/absorb-nested", s.handleAbsorbNested)
	s.mux.HandleFunc("POST /api/push", s.handlePush)
	s.mux.HandleFunc("POST /api/pull", s.handlePull)

	// Audit
	s.mux.HandleFunc("GET /api/audit/stream", s.handleAuditStream)
	s.mux.HandleFunc("PATCH /api/audit/policy", s.handleAuditPolicy)
	s.mux.HandleFunc("GET /api/audit/rules/compiled", s.handleGetCompiledRules)
	s.mux.HandleFunc("POST /api/audit/rules/toggle", s.handleToggleRule)
	s.mux.HandleFunc("POST /api/audit/rules/reset", s.handleResetRules)
	s.mux.HandleFunc("GET /api/audit/rules", s.handleGetAuditRules)
	s.mux.HandleFunc("PUT /api/audit/rules", s.handlePutAuditRules)
	s.mux.HandleFunc("POST /api/audit/rules", s.handleInitAuditRules)
	s.mux.HandleFunc("GET /api/audit", s.handleAuditAll)
	s.mux.HandleFunc("GET /api/audit/{name}", s.handleAuditSkill)

	// Analyze (context-window budget)
	s.mux.HandleFunc("GET /api/analyze", s.handleAnalyze)

	// Log
	s.mux.HandleFunc("GET /api/log", s.handleListLog)
	s.mux.HandleFunc("GET /api/log/stats", s.handleLogStats)
	s.mux.HandleFunc("DELETE /api/log", s.handleClearLog)

	// Config
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
	s.mux.HandleFunc("GET /api/config/available-targets", s.handleAvailableTargets)

	// Skillignore
	s.mux.HandleFunc("GET /api/skillignore", s.handleGetSkillignore)
	s.mux.HandleFunc("PUT /api/skillignore", s.handlePutSkillignore)

	// Agentignore
	s.mux.HandleFunc("GET /api/agentignore", s.handleGetAgentignore)
	s.mux.HandleFunc("PUT /api/agentignore", s.handlePutAgentignore)

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
