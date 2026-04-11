package server

import (
	"net/http"

	"skillshare/internal/inspect"
)

type discoveredRuleResponseItem struct {
	inspect.RuleItem
	Stats contentStats `json:"stats"`
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	projectRoot := ""
	if s.IsProjectMode() {
		projectRoot = s.projectRoot
	}

	items, warnings, err := inspect.ScanRules(projectRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan rules: "+err.Error())
		return
	}

	rules := make([]discoveredRuleResponseItem, 0, len(items))
	for _, item := range items {
		rules = append(rules, discoveredRuleResponseItem{
			RuleItem: item,
			Stats:    buildContentStats(item.Content),
		})
	}

	writeJSON(w, map[string]any{
		"rules":    rules,
		"warnings": warnings,
	})
}
