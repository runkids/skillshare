package server

import (
	"net/http"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
)

// serverInstallContext runs the CLI's bare `skillshare install` (install
// everything recorded in config that is not on disk) for the dashboard.
type serverInstallContext struct{ s *Server }

var _ install.InstallContext = serverInstallContext{}

// SourcePath is the project's skills folder in project mode, else the global one.
func (c serverInstallContext) SourcePath() string { return c.s.skillsSource() }

func (c serverInstallContext) ConfigSkills() []install.SkillEntryDTO {
	if !c.s.IsProjectMode() {
		return install.MetadataSkillEntries(install.LoadMetadataOrNew(c.SourcePath()))
	}
	return config.ProjectSkillEntries(c.s.projectCfg.Skills)
}

func (c serverInstallContext) Reconcile() error {
	// Reload from the same folder the install wrote to; reloadSkillsStore reads the global one.
	if st, err := install.LoadMetadataWithMigration(c.SourcePath(), ""); err == nil && st != nil {
		c.s.skillsStore = st
	}
	c.s.reconcileSkillsConfig(c.SourcePath())
	return nil
}

func (c serverInstallContext) PostInstallSkill(displayName string) error {
	if !c.s.IsProjectMode() {
		return nil
	}
	dir := c.s.gitignoreDir()
	if dir == "" {
		return nil
	}
	return install.UpdateGitIgnore(dir, c.s.projectGitignorePrefix()+"/"+displayName)
}

func (c serverInstallContext) Mode() string          { return c.s.installLogMode() }
func (c serverInstallContext) GitLabHosts() []string { return c.s.parseOpts().GitLabHosts }
func (c serverInstallContext) AzureHosts() []string  { return c.s.parseOpts().AzureHosts }
func (c serverInstallContext) CNBHosts() []string    { return c.s.parseOpts().CNBHosts }
func (c serverInstallContext) GiteaHosts() []string  { return c.s.parseOpts().GiteaHosts }

type configEntryInfo struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Tracked bool   `json:"tracked"`
	Branch  string `json:"branch,omitempty"`
}

// handleMissingConfigEntries lists config entries that are not on disk, so the
// dashboard can offer to install them (bare `skillshare install`).
func (s *Server) handleMissingConfigEntries(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	missing := install.MissingFromConfig(serverInstallContext{s})
	s.mu.RUnlock()
	entries := make([]configEntryInfo, 0, len(missing))
	for _, m := range missing {
		entries = append(entries, configEntryInfo{Name: m.FullName(), Source: m.Source, Tracked: m.Tracked, Branch: m.Branch})
	}
	file := ".metadata.json"
	if s.IsProjectMode() {
		file = ".skillshare/config.yaml"
	}
	writeJSON(w, map[string]any{"entries": entries, "file": file})
}

// handleInstallFromConfig installs every config entry missing on disk, like
// bare `skillshare install`.
func (s *Server) handleInstallFromConfig(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.trackProjectLock()()

	opts := install.InstallOptions{
		Quiet:          true,
		AuditThreshold: s.auditThreshold(),
		SourceFollow:   s.skillsWalk().Follow,
	}
	if s.IsProjectMode() {
		// The dashboard's config-install action follows project lock advances,
		// matching ordinary CLI installs rather than JSON-only automation.
		opts.Update = true
		opts.AuditProjectRoot = s.projectRoot
		lock, err := install.LoadLock(projectdir.Resolve(s.projectRoot))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		opts.Lock = lock
	}

	result, err := install.InstallFromConfig(serverInstallContext{s}, opts)
	status, msg := "ok", ""
	if err != nil {
		status, msg = "error", err.Error()
	} else if len(result.FailedSkills) > 0 {
		// Match the CLI: nothing installed is an error, with each skill's reason.
		status = "partial"
		if len(result.InstalledSkills)+result.Skipped == 0 {
			reasons := make([]string, len(result.Failures))
			for i, f := range result.Failures {
				reasons[i] = f.Name + ": " + f.Err.Error()
			}
			status, msg = "error", strings.Join(reasons, "; ")
		}
	}
	s.writeOpsLog("install", status, start, map[string]any{
		"source":           "config",
		"mode":             s.installLogMode(),
		"threshold":        s.auditThreshold(),
		"scope":            "ui",
		"installed_skills": result.InstalledSkills,
		"failed_skills":    result.FailedSkills,
		"skipped_skills":   result.SkippedSkills,
	}, msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"installed":           result.Installed,
		"installedRepos":      result.InstalledRepos,
		"installedRepoSkills": result.InstalledRepoSkills,
		"skipped":             result.Skipped,
		"failed":              result.FailedSkills,
	})
}
