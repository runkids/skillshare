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
	Local             bool              `json:"local,omitempty"`
	Disabled          bool              `json:"disabled,omitempty"`
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

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	follow := s.skillFollowSet()
	targets := s.cfg.Targets
	defaultMode := s.cfg.Mode
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsForAnalyzeWithOptions(source, ssync.DiscoveryOptions{Follow: follow})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries := make([]analyzeTargetResponse, 0)
	for name, target := range targets {
		filtered, ferr := ssync.TargetSkills(name, target, defaultMode, source, discovered)
		if ferr != nil || len(filtered) == 0 {
			continue
		}

		skills := make([]analyzeSkillResponse, 0, len(filtered))
		var totalDescChars, totalBodyChars, totalDescTokens, totalBodyTokens int
		for _, sk := range filtered {
			totalDescChars += sk.DescChars
			totalBodyChars += sk.BodyChars
			totalDescTokens += sk.DescTokens
			totalBodyTokens += sk.BodyTokens
			skills = append(skills, analyzeSkillResponse{
				Name:              sk.FlatName,
				DescriptionChars:  sk.DescChars,
				DescriptionTokens: sk.DescTokens,
				BodyChars:         sk.BodyChars,
				BodyTokens:        sk.BodyTokens,
				LintIssues:        sk.LintIssues,
				Local:             sk.Local,
				Disabled:          sk.Disabled,
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
				EstimatedTokens: totalDescTokens,
			},
			OnDemandMax: analyzeCharTokensResponse{
				Chars:           totalBodyChars,
				EstimatedTokens: totalBodyTokens,
			},
			Skills: skills,
		})
	}

	slices.SortFunc(entries, func(a, b analyzeTargetResponse) int {
		return cmp.Compare(b.AlwaysLoaded.Chars, a.AlwaysLoaded.Chars)
	})

	writeJSON(w, map[string]any{"targets": entries})
}
