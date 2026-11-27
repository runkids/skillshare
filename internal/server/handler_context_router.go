package server

import (
	"net/http"
	"os"

	"skillshare/internal/config"
	"skillshare/internal/contextrouter"
)

// handleContextRouter answers the Context page's Router tab: the folders that
// have an agents-context-router config, and one of them measured against its
// byte caps. It reads only. The router's script is never run, and the config
// is loaded read-only, since the ordinary reload can migrate a legacy config
// on disk. ?root= picks a folder from the list; any other path is 404, so the
// route cannot be pointed at an arbitrary folder.
func (s *Server) handleContextRouter(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	var in contextrouter.DiscoverInput
	if s.IsProjectMode() {
		in.ProjectRoot = s.projectRoot
	} else {
		if err := s.reloadProjectsReadOnly(); err != nil {
			s.mu.Unlock()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		in.SkillsSource = s.cfg.EffectiveSkillsSource()
		in.AgentsSource = s.cfg.EffectiveAgentsSource()
		for root, project := range s.cfg.Projects {
			in.Projects = append(in.Projects, contextrouter.Project{Path: config.ExpandPath(root), Name: project.Name})
		}
		in.Home, _ = os.UserHomeDir()
	}
	// Release s.mu before reading the folders: every other API request waits on it.
	s.mu.Unlock()

	roots := contextrouter.Discover(in)
	selected := ""
	if len(roots) > 0 {
		selected = roots[0].Path
	}
	if want := r.URL.Query().Get("root"); want != "" {
		selected = ""
		for _, root := range roots {
			if root.Path == want {
				selected = want
			}
		}
		if selected == "" {
			writeError(w, http.StatusNotFound, "no context router in that folder")
			return
		}
	}

	var report *contextrouter.Report
	if selected != "" {
		measured, err := contextrouter.Measure(r.Context(), selected, contextrouter.DefaultLimits)
		if err != nil {
			if r.Context().Err() != nil {
				return // the client left
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		report = &measured
	}
	writeJSON(w, map[string]any{"roots": roots, "selected": selected, "router": report})
}
