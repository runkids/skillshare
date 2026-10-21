package server

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
)

type projectResource projects.Resource
type projectItem = projects.Item

type projectBody struct {
	Root    string           `json:"root"`
	Name    string           `json:"name"`
	Targets []string         `json:"targets"`
	Skills  *projectResource `json:"skills"`
	Agents  *projectResource `json:"agents"`
	// Create refuses a root that is already declared.
	Create bool `json:"create"`
}

func (r *projectResource) config() *config.ResourceTargetConfig {
	if r == nil {
		return nil
	}
	return &config.ResourceTargetConfig{Mode: r.Mode, TargetNaming: r.TargetNaming, Include: r.Include, Exclude: r.Exclude}
}

func (s *Server) requireGlobalProjects(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.IsProjectMode() {
			writeError(w, http.StatusBadRequest, "projects belong in the global config")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadProjectsReadOnly(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items, err := s.projectService().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type convertible struct {
		Root    string   `json:"root"`
		Targets []string `json:"targets"`
		Tools   []string `json:"tools"`
		Agents  bool     `json:"agents"`
	}
	offers := []convertible{}
	for _, found := range s.cfg.ConvertibleProjects() {
		offers = append(offers, convertible{Root: found.Root, Targets: found.Targets, Tools: found.Project.Targets, Agents: found.Project.Agents != nil})
	}

	type tool struct {
		Name       string `json:"name"`
		SkillsPath string `json:"skillsPath"`
		AgentsPath string `json:"agentsPath"`
	}
	tools := []tool{}
	agentPaths := config.ProjectAgentTargets()
	for name, target := range config.ProjectTargets() {
		tools = append(tools, tool{Name: name, SkillsPath: target.Path, AgentsPath: agentPaths[name].Path})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })

	writeJSON(w, map[string]any{"projects": items, "convertible": offers, "tools": tools})
}

// handleSaveProject adds a project or replaces the one declared under the same root.
func (s *Server) handleSaveProject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body projectBody
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Root == "" {
		writeError(w, http.StatusBadRequest, "root is required")
		return
	}
	for _, rc := range []*projectResource{body.Skills, body.Agents} {
		if rc == nil {
			continue
		}
		if _, err := ssync.FilterSkills(nil, rc.Include, rc.Exclude); err != nil {
			writeError(w, http.StatusBadRequest, "invalid include/exclude pattern: "+err.Error())
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := projects.LockConfig(r.Context(), s.configPath())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer release()
	if err := s.reloadProjectsReadOnly(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	root, lookupErr := lookupProjectRoot(s.cfg.Projects, body.Root)
	if lookupErr != nil {
		if !errors.Is(lookupErr, projects.ErrNotFound) {
			writeError(w, http.StatusBadRequest, lookupErr.Error())
			return
		}
		root = body.Root
	}
	result, err := s.projectService().Put(root, config.ManagedProject{Name: body.Name, Targets: body.Targets, Skills: body.Skills.config(), Agents: body.Agents.config()}, projects.PutOptions{Create: body.Create})
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, projects.ErrSave), errors.Is(err, projects.ErrDetach):
			status = http.StatusInternalServerError
		case errors.Is(err, projects.ErrExists):
			status = http.StatusConflict
		case errors.Is(err, projects.ErrNotFound):
			status = http.StatusNotFound
		}

		s.writeProjectFailure(w, status, result, err, start, "update", root)
		return
	}
	s.installGlobalConfig(result.Expanded)
	action := "update"
	if body.Create {
		action = "add"
	}
	s.writeOpsLog("project", "ok", start, map[string]any{"action": action, "root": root, "scope": "ui"}, "")
	writeJSON(w, map[string]any{"success": true, "root": root})
}

func (s *Server) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := projects.LockConfig(r.Context(), s.configPath())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer release()
	if err := s.reloadProjectsReadOnly(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	root, lookupErr := lookupProjectRoot(s.cfg.Projects, r.URL.Query().Get("root"))
	if lookupErr != nil {
		if !errors.Is(lookupErr, projects.ErrNotFound) {
			writeError(w, http.StatusBadRequest, lookupErr.Error())
			return
		}
		root = r.URL.Query().Get("root")
	}
	if _, exists := s.cfg.Projects[root]; !exists {
		writeError(w, http.StatusNotFound, "project not found: "+root)
		return
	}
	result, err := s.projectService().Remove(root, projects.RemoveOptions{})
	if err != nil {
		s.writeProjectFailure(w, http.StatusInternalServerError, result, err, start, "remove", root)
		return
	}
	s.installGlobalConfig(result.Expanded)
	s.writeOpsLog("project", "ok", start, map[string]any{"action": "remove", "root": root, "scope": "ui"}, "")
	writeJSON(w, map[string]any{"success": true, "root": root})
}

// handleConvertProject moves the targets that write into one folder under projects.
// They keep their folders, mode and filters, so the next sync changes nothing.
func (s *Server) handleConvertProject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Root string `json:"root"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := projects.LockConfig(r.Context(), s.configPath())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer release()
	if err := s.reloadProjectsReadOnly(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, found := range s.cfg.ConvertibleProjects() {
		if found.Root != body.Root {
			continue
		}
		next := *s.cfg
		next.Targets = maps.Clone(s.cfg.Targets)
		for _, name := range found.Targets {
			delete(next.Targets, name)
		}
		candidate, err := projects.PrepareProjects(s.configPath(), &next, func(entries map[string]config.ManagedProject) error { entries[found.Root] = found.Project; return nil })
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := candidate.Save.Write(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.installGlobalConfig(candidate.Save.Expanded)
		s.writeOpsLog("project", "ok", start, map[string]any{"action": "convert", "root": found.Root, "targets": found.Targets, "scope": "ui"}, "")
		writeJSON(w, map[string]any{"success": true, "root": found.Root})
		return
	}
	writeError(w, http.StatusNotFound, "no targets to convert for "+body.Root)
}

func lookupProjectRoot(entries map[string]config.ManagedProject, root string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return projects.FindRoot(entries, root, cwd, home)
}

// projectService is used while the caller holds s.mu.
func (s *Server) projectService() *projects.Service {
	svc := projects.NewService(s.configPath(), func() (*config.Config, error) { return s.cfg, nil })
	svc.DetachSkillsTarget = detachSkillsTarget
	return svc
}

// reloadProjectsReadOnly is called under s.mu, and under the project file lock
// for mutations. Unlike the ordinary dashboard reload it never migrates on disk.
func (s *Server) reloadProjectsReadOnly() error {
	cfg, err := config.LoadReadOnlyPath(s.configPath())
	if err != nil {
		return err
	}
	s.installGlobalConfig(cfg)
	return nil
}

func (s *Server) writeProjectFailure(w http.ResponseWriter, status int, result projects.Result, err error, start time.Time, action, root string) {
	if result.Detach == nil || (len(result.Detach.Removed) == 0 && len(result.Detach.CopiedBack) == 0) {
		writeError(w, status, err.Error())
		return
	}
	message := fmt.Sprintf("%s; registration %s; completed detach: removed [%s], copied back [%s]", err, result.Registration, strings.Join(result.Detach.Removed, ", "), strings.Join(result.Detach.CopiedBack, ", "))
	s.writeOpsLog("project", "partial", start, map[string]any{"action": action, "root": root, "scope": "ui"}, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"error": message, "detach": result.Detach, "registration": result.Registration})
}
