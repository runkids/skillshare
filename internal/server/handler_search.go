package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/hub"
	"skillshare/internal/install"
	"skillshare/internal/search"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.skillsSource()
	hubCfg := s.cfg.Hub
	if s.IsProjectMode() && s.projectCfg != nil {
		hubCfg = s.projectCfg.Hub
	}
	s.mu.RUnlock()

	query := r.URL.Query().Get("q")
	hubParam := r.URL.Query().Get("hub")

	limit := 0 // default: no limit for hub search
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	var results []search.SearchResult
	var err error
	switch {
	case hubParam == "@builtin":
		results, err = searchBuiltinIndex(source, query, limit)
	case hubParam != "":
		// SSH hub sources trigger a git clone using the host's SSH credentials.
		// In the web server, restrict that to URLs the user has explicitly saved
		// (via 'skillshare hub add') to avoid cloning arbitrary request-supplied
		// hosts.
		if install.IsSSHURL(hubParam) && !savedHubURLSet(hubCfg)[strings.TrimSpace(hubParam)] {
			writeError(w, http.StatusForbidden, "SSH hub sources must be added first via 'skillshare hub add'")
			return
		}
		results, err = search.SearchFromIndexURL(query, limit, hubParam)
	default:
		// GitHub search always needs a limit
		if limit <= 0 {
			limit = 20
		}
		results, err = search.Search(query, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type resultItem struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Source      string   `json:"source"`
		Skill       string   `json:"skill,omitempty"`
		Stars       int      `json:"stars"`
		Owner       string   `json:"owner"`
		Repo        string   `json:"repo"`
		Tags        []string `json:"tags,omitempty"`
	}

	items := make([]resultItem, 0, len(results))
	for _, r := range results {
		items = append(items, resultItem{
			Name:        r.Name,
			Description: r.Description,
			Source:      r.Source,
			Skill:       r.Skill,
			Stars:       r.Stars,
			Owner:       r.Owner,
			Repo:        r.Repo,
			Tags:        r.Tags,
		})
	}

	writeJSON(w, map[string]any{"results": items})
}

// savedHubURLSet returns the set of saved hub URLs for fast membership checks.
func savedHubURLSet(hub config.HubConfig) map[string]bool {
	set := make(map[string]bool, len(hub.Hubs))
	for _, e := range hub.Hubs {
		if u := strings.TrimSpace(e.URL); u != "" {
			set[u] = true
		}
	}
	return set
}

// searchBuiltinIndex builds the hub index from local skills and searches it in-memory.
func searchBuiltinIndex(sourcePath, query string, limit int) ([]search.SearchResult, error) {
	idx, err := hub.BuildIndex(sourcePath, false, false)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return nil, err
	}
	return search.SearchFromIndexJSON(query, limit, data)
}
