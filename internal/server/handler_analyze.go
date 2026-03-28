package server

import (
	"cmp"
	"net/http"
	"slices"

	ssync "skillshare/internal/sync"
)

type analyzeCharTokensResponse struct {
	Chars           int `json:"chars"`
	EstimatedTokens int `json:"estimated_tokens"`
}

type analyzeSkillResponse struct {
	Name              string            `json:"name"`
	DescriptionChars  int               `json:"description_chars"`
	DescriptionTokens int               `json:"description_tokens"`
	BodyChars         int               `json:"body_chars"`
	BodyTokens        int               `json:"body_tokens"`
	LintIssues        []ssync.LintIssue `json:"lint_issues,omitempty"`
	Path              string            `json:"path"`
	IsTracked         bool              `json:"is_tracked"`
	Targets           []string          `json:"targets,omitempty"`
	Description       string            `json:"description,omitempty"`
}

type analyzeTargetResponse struct {
	Name         string                    `json:"name"`
	SkillCount   int                       `json:"skill_count"`
	AlwaysLoaded analyzeCharTokensResponse `json:"always_loaded"`
	OnDemandMax  analyzeCharTokensResponse `json:"on_demand_max"`
	Skills       []analyzeSkillResponse    `json:"skills"`
}

const analyzeCharsPerToken = 4

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.cfg.Source
	targets := s.cfg.Targets
	defaultMode := s.cfg.Mode
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsForAnalyze(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries := make([]analyzeTargetResponse, 0)
	for name, target := range targets {
		sc := target.SkillsConfig()
		tMode := sc.Mode
		if tMode == "" {
			tMode = defaultMode
		}

		var filtered []ssync.DiscoveredSkill
		if tMode == "symlink" {
			filtered = discovered
		} else {
			var ferr error
			filtered, ferr = ssync.FilterSkills(discovered, sc.Include, sc.Exclude)
			if ferr != nil {
				continue
			}
			filtered = ssync.FilterSkillsByTarget(filtered, name)
		}

		if len(filtered) == 0 {
			continue
		}

		skills := make([]analyzeSkillResponse, 0, len(filtered))
		var totalDescChars, totalBodyChars int
		for _, sk := range filtered {
			totalDescChars += sk.DescChars
			totalBodyChars += sk.BodyChars
			skills = append(skills, analyzeSkillResponse{
				Name:              sk.FlatName,
				DescriptionChars:  sk.DescChars,
				DescriptionTokens: sk.DescChars / analyzeCharsPerToken,
				BodyChars:         sk.BodyChars,
				BodyTokens:        sk.BodyChars / analyzeCharsPerToken,
				LintIssues:        sk.LintIssues,
				Path:              sk.RelPath,
				IsTracked:         sk.IsInRepo,
				Targets:           sk.Targets,
				Description:       sk.Description,
			})
		}

		slices.SortFunc(skills, func(a, b analyzeSkillResponse) int {
			return cmp.Compare(b.DescriptionChars, a.DescriptionChars)
		})

		entries = append(entries, analyzeTargetResponse{
			Name:       name,
			SkillCount: len(skills),
			AlwaysLoaded: analyzeCharTokensResponse{
				Chars:           totalDescChars,
				EstimatedTokens: totalDescChars / analyzeCharsPerToken,
			},
			OnDemandMax: analyzeCharTokensResponse{
				Chars:           totalBodyChars,
				EstimatedTokens: totalBodyChars / analyzeCharsPerToken,
			},
			Skills: skills,
		})
	}

	slices.SortFunc(entries, func(a, b analyzeTargetResponse) int {
		return cmp.Compare(b.AlwaysLoaded.Chars, a.AlwaysLoaded.Chars)
	})

	writeJSON(w, map[string]any{"targets": entries})
}
