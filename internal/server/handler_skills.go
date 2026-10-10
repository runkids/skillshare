package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/uninstall"
	"skillshare/internal/utils"
)

type skillItem struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // "skill" or "agent"
	FlatName    string   `json:"flatName"`
	RelPath     string   `json:"relPath"`
	SourcePath  string   `json:"sourcePath"`
	LinkName    string   `json:"linkName,omitempty"`
	LinkTarget  string   `json:"linkTarget,omitempty"`
	IsInRepo    bool     `json:"isInRepo"`
	RepoPath    string   `json:"repoPath,omitempty"`
	Targets     []string `json:"targets,omitempty"`
	InstalledAt string   `json:"installedAt,omitempty"`
	Source      string   `json:"source,omitempty"`
	Type        string   `json:"type,omitempty"`
	RepoURL     string   `json:"repoUrl,omitempty"`
	Version     string   `json:"version,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	Disabled    bool     `json:"disabled"`
	// ManualOnly mirrors disable-model-invocation: installed and invocable by name, but the
	// model does not load it on its own. The list TUI toggles it with M.
	ManualOnly bool `json:"manualOnly,omitempty"`
}

// ponytail: one small read per skill on the list; carry it on DiscoveredSkill if lists get slow.
func manualOnly(skillDir string) bool {
	return strings.EqualFold(utils.ParseFrontmatterField(filepath.Join(skillDir, "SKILL.md"), "disable-model-invocation"), "true")
}

// enrichSkillBranch fills item.Branch from metadata, falling back to
// git.GetCurrentBranch for tracked repos without branch in metadata.
func enrichSkillBranch(item *skillItem) {
	if item.Branch == "" && item.IsInRepo {
		if branch, err := git.GetCurrentBranch(item.SourcePath); err == nil {
			item.Branch = branch
		}
	}
}

func enrichSkillLink(item *skillItem, source string, walk sourcewalk.Options, linkTargets map[string]string) {
	if walk.Follow == nil {
		return
	}
	name := strings.SplitN(filepath.ToSlash(item.RelPath), "/", 2)[0]
	target, checked := linkTargets[name]
	if !checked {
		target, _ = walk.Follow.Resolve(filepath.Join(source, name))
		linkTargets[name] = target
	}
	if target != "" {
		item.LinkName, item.LinkTarget = name, target
	}
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	kindFilter := r.URL.Query().Get("kind") // "", "skill", "agent"

	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.skillsSource()
	agentsSource := s.agentsSource()
	walk := s.skillsWalk()
	s.mu.RUnlock()

	var items []skillItem
	sourceLinks := []sourceLinkItem{}

	// Skills
	if kindFilter == "" || kindFilter == "skill" {
		discovered, err := sync.DiscoverSourceSkillsAll(source, walk)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// After discovery, so a traversal failure inside a target shows on its link.
		if sourceLinks, err = listSourceLinks(source, walk); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Resolve each first-level entry once with the same policy used for discovery.
		linkTargets := make(map[string]string)
		for _, d := range discovered {
			item := skillItem{
				Name:       filepath.Base(d.SourcePath),
				Kind:       "skill",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				RepoPath:   d.RepoRelPath,
				Targets:    d.Targets,
				Disabled:   d.Disabled,
				ManualOnly: manualOnly(d.SourcePath),
			}
			enrichSkillLink(&item, source, walk, linkTargets)

			if entry := s.discoveredSkillEntry(d.RelPath, d.RepoRelPath); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
				item.Branch = entry.Branch
			}
			enrichSkillBranch(&item)

			items = append(items, item)
		}
	}

	// Agents — recursive discovery (supports --into subdirectories)
	if (kindFilter == "" || kindFilter == "agent") && agentsSource != "" {
		discovered, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range discovered {
			item := skillItem{
				Name:       d.Name,
				Kind:       "agent",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				RepoPath:   d.RepoRelPath,
				Disabled:   d.Disabled,
				Targets:    d.Targets,
			}

			// Read from centralized agents metadata store
			agentKey := strings.TrimSuffix(d.RelPath, ".md")
			if entry := s.agentEntry(agentKey); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
			} else if d.RepoRelPath != "" {
				repoPath := filepath.Join(agentsSource, filepath.FromSlash(d.RepoRelPath))
				if repoURL, err := git.GetRemoteURL(repoPath); err == nil {
					item.Source = repoURL
					item.RepoURL = repoURL
				}
				if version, err := git.GetCurrentFullHash(repoPath); err == nil {
					item.Version = version
				}
			}

			items = append(items, item)
		}
	}

	_, linked := dashboardRepos(source, walk)
	writeJSON(w, map[string]any{"resources": items, "sourceLinks": sourceLinks, "sourceLinkWarnings": sync.SourceLinkWarnings(walk, false), "linked_repos": linked})
}

func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.skillsSource()
	walk := s.skillsWalk()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "skill" && kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	// Find the skill by flat name (exact) first, then fall back to base name.
	if kind != "agent" {
		discovered, err := sync.DiscoverSourceSkillsAll(source, walk)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Two-pass: prefer FlatName exact match over baseName fallback.
		var match *sync.DiscoveredSkill
		for i := range discovered {
			if discovered[i].FlatName == name {
				match = &discovered[i]
				break
			}
		}
		if match == nil {
			for i := range discovered {
				if filepath.Base(discovered[i].SourcePath) == name {
					match = &discovered[i]
					break
				}
			}
		}

		if match != nil {
			d := match
			baseName := filepath.Base(d.SourcePath)
			item := skillItem{
				Name:       baseName,
				Kind:       "skill",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				RepoPath:   d.RepoRelPath,
				Targets:    d.Targets,
				Disabled:   d.Disabled,
				ManualOnly: manualOnly(d.SourcePath),
			}
			enrichSkillLink(&item, source, walk, make(map[string]string))

			if entry := s.discoveredSkillEntry(d.RelPath, d.RepoRelPath); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
				item.Branch = entry.Branch
			}
			enrichSkillBranch(&item)

			// Read SKILL.md content
			skillMdContent := ""
			skillMdPath := filepath.Join(d.SourcePath, "SKILL.md")
			if data, err := os.ReadFile(skillMdPath); err == nil {
				skillMdContent = string(data)
			}

			// List files from the followed root, keeping response paths relative to it.
			walkRoot := d.SourcePath
			if resolved, ok := walk.Follow.Resolve(walkRoot); ok {
				walkRoot = resolved
			}
			files := make([]string, 0)
			filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && path != walkRoot && utils.IsHidden(info.Name()) {
					return filepath.SkipDir
				}
				if !info.IsDir() {
					rel, _ := filepath.Rel(walkRoot, path)
					// Normalize separators
					rel = strings.ReplaceAll(rel, "\\", "/")
					files = append(files, rel)
				}
				return nil
			})

			writeJSON(w, map[string]any{
				"resource":       item,
				"skillMdContent": skillMdContent,
				"files":          files,
			})
			return
		}
	}

	// Fallback: check agents source (recursive — supports --into subdirectories)
	if kind != "skill" && agentsSource != "" {
		agentDiscovered, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range agentDiscovered {
			if !matchesAgentName(d, name) {
				continue
			}

			data, readErr := os.ReadFile(d.SourcePath)
			if readErr != nil {
				continue
			}

			item := skillItem{
				Name:       d.Name,
				Kind:       "agent",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				RepoPath:   d.RepoRelPath,
				Disabled:   d.Disabled,
				Targets:    d.Targets,
			}

			agentKey := strings.TrimSuffix(d.RelPath, ".md")
			if entry := s.agentEntry(agentKey); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
			} else if d.RepoRelPath != "" {
				repoPath := filepath.Join(agentsSource, filepath.FromSlash(d.RepoRelPath))
				if repoURL, err := git.GetRemoteURL(repoPath); err == nil {
					item.Source = repoURL
					item.RepoURL = repoURL
				}
				if version, err := git.GetCurrentFullHash(repoPath); err == nil {
					item.Version = version
				}
			}

			writeJSON(w, map[string]any{
				"resource":       item,
				"skillMdContent": string(data),
				"files":          []string{filepath.Base(d.RelPath)},
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleGetSkillFile(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	s.mu.RUnlock()

	name := r.PathValue("name")
	fp := r.PathValue("filepath")

	// Reject path traversal attempts
	if strings.Contains(fp, "..") {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	// Find the skill
	discovered, err := sync.DiscoverSourceSkills(source, s.skillsWalk())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		// Resolve and verify the file is within the skill directory
		absPath := filepath.Join(d.SourcePath, filepath.FromSlash(fp))
		absPath = filepath.Clean(absPath)
		skillDir := filepath.Clean(d.SourcePath) + string(filepath.Separator)
		if !strings.HasPrefix(absPath, skillDir) {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			} else {
				writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
			}
			return
		}

		// Determine content type from extension
		ct := "text/plain"
		switch strings.ToLower(filepath.Ext(absPath)) {
		case ".md":
			ct = "text/markdown"
		case ".json":
			ct = "application/json"
		case ".yaml", ".yml":
			ct = "text/yaml"
		}

		writeJSON(w, map[string]any{
			"content":     string(data),
			"contentType": ct,
			"filename":    filepath.Base(absPath),
		})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleUninstallRepo(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(r.PathValue("name"))
	if !validSourceName(name) {
		writeError(w, http.StatusBadRequest, "invalid or missing tracked repository name")
		return
	}
	cleanName := filepath.Clean(filepath.FromSlash(name))

	repoName, repoPath, resolveErr := s.resolveTrackedRepo(cleanName)
	if resolveErr != nil {
		writeError(w, http.StatusBadRequest, resolveErr.Error())
		return
	}
	if repoPath == "" {
		writeError(w, http.StatusBadRequest, "not a tracked repository: "+cleanName)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	res := s.uninstallSkills([]uninstall.Item{s.repoUninstallItem(repoName, repoPath)}, force)[0]
	if code := uninstallForceCode(res.Err); code != "" {
		writeCodedError(w, http.StatusConflict, code, uninstallErrorMessage(res), nil)
		return
	}
	if res.Err != nil {
		writeError(w, uninstallErrorStatus(res.Err), uninstallErrorMessage(res))
		return
	}

	s.writeOpsLog("uninstall", "ok", start, map[string]any{
		"name":  repoName,
		"type":  "repo",
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": repoName, "movedToTrash": true})
}

func (s *Server) handleUninstallSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "skill" && kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	if kind == "agent" {
		agentsSource := s.agentsSource()
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "agent not found: "+name)
			return
		}
		agent, err := resolveAgentResource(agentsSource, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		displayName := agentMetaKey(agent.RelPath)
		if err := s.uninstallAgents(agentsSource, []uninstall.Agent{{Name: displayName, File: agent.SourcePath}})[0]; err != nil {
			writeError(w, http.StatusInternalServerError, "failed to trash agent: "+err.Error())
			return
		}

		s.writeOpsLog("uninstall", "ok", start, map[string]any{
			"name":  displayName,
			"type":  "agent",
			"scope": "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true, "name": displayName, "movedToTrash": true})
		return
	}

	// Find skill path. Disabled skills are listed in .skillignore, but the UI
	// still shows them, so single-resource uninstall must resolve them too (#190).
	source := s.skillsSource()
	walk := s.skillsWalk()
	discovered, err := sync.DiscoverSourceSkillsAll(source, walk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	d, err := resolveUninstallSkill(discovered, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if d != nil {
		// Followed checkouts allow single-skill removal; managed repos do not.
		_, followed := walk.Follow.Resolve(d.SourcePath)
		if d.IsInRepo && !followed {
			writeError(w, http.StatusBadRequest, "cannot uninstall skill from tracked repo; use 'skillshare uninstall' for the whole repo")
			return
		}

		res := s.uninstallSkills([]uninstall.Item{skillUninstallItem(d, followed)}, false)[0]
		if res.Err != nil {
			status := uninstallErrorStatus(res.Err)
			if errors.Is(res.Err, sourcefs.ErrLinkedSkillRoot) {
				status = http.StatusBadRequest
			}
			writeError(w, status, uninstallErrorMessage(res))
			return
		}

		s.writeOpsLog("uninstall", "ok", start, map[string]any{
			"name":  res.Item.TrashName,
			"type":  "skill",
			"scope": "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true, "name": name, "movedToTrash": true})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

// resolveTrackedRepo resolves a repo name (flat or nested) to its slash-form
// name and absolute path under s.cfg.EffectiveSkillsSource(). Returns ("", "", nil)
// if not found, and a non-nil error for ambiguous matches or internal failures.
func (s *Server) resolveTrackedRepo(input string, walks ...sourcewalk.Options) (string, string, error) {
	if len(walks) == 0 {
		walks = []sourcewalk.Options{s.skillsWalk()}
	}
	return install.ResolveTrackedRepo(s.cfg.EffectiveSkillsSource(), input, walks[0])
}
