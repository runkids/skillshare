package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"skillshare/internal/hub"
	"skillshare/internal/search"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
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
		results, err = s.searchBuiltinIndex(query, limit)
	case hubParam != "":
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

// searchBuiltinIndex builds the hub index from local skills and searches it in-memory.
func (s *Server) searchBuiltinIndex(query string, limit int) ([]search.SearchResult, error) {
	sourcePath := s.cfg.Source
	if s.IsProjectMode() {
		sourcePath = filepath.Join(s.projectRoot, ".skillshare", "skills")
	}
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
