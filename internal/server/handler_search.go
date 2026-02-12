package server

import (
	"net/http"
	"strconv"

	"skillshare/internal/search"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	indexURL := r.URL.Query().Get("hub")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	var results []search.SearchResult
	var err error
	if indexURL != "" {
		results, err = search.SearchFromIndexURL(query, limit, indexURL)
	} else {
		results, err = search.Search(query, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type resultItem struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
		Stars       int    `json:"stars"`
		Owner       string `json:"owner"`
		Repo        string `json:"repo"`
	}

	items := make([]resultItem, 0, len(results))
	for _, r := range results {
		items = append(items, resultItem{
			Name:        r.Name,
			Description: r.Description,
			Source:      r.Source,
			Stars:       r.Stars,
			Owner:       r.Owner,
			Repo:        r.Repo,
		})
	}

	writeJSON(w, map[string]any{"results": items})
}
