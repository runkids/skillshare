package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

// formatTokenK formats token count with K suffix: 12400 → "12.4K", 830 → "830"
func formatTokenK(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}

// formatTokenComma formats with comma separators: 50123 → "50,123"
func formatTokenComma(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

type tokenPair struct {
	AlwaysLoaded int
	OnDemand     int
}

type tokenGroup struct {
	AlwaysLoaded int
	OnDemand     int
	Targets      []string
}

// groupByTokenCost groups targets with identical token counts
func groupByTokenCost(entries []analyzeTargetEntry) []tokenGroup {
	groups := make(map[tokenPair][]string)
	for _, e := range entries {
		key := tokenPair{e.AlwaysLoaded.EstimatedTokens, e.OnDemandMax.EstimatedTokens}
		groups[key] = append(groups[key], e.Name)
	}
	var result []tokenGroup
	for pair, targets := range groups {
		sort.Strings(targets)
		result = append(result, tokenGroup{
			AlwaysLoaded: pair.AlwaysLoaded,
			OnDemand:     pair.OnDemand,
			Targets:      targets,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Targets[0] < result[j].Targets[0]
	})
	return result
}

type budgetViolation struct {
	Type      string
	Actual    int
	Budget    int
	Offenders []tokenOffender
}

type tokenOffender struct {
	Name   string
	Tokens int
}

// checkBudget checks if any target exceeds the configured budget thresholds
func checkBudget(entries []analyzeTargetEntry, budget config.ContextBudgetConfig) []budgetViolation {
	alwaysThreshold := budget.AlwaysLoadedThreshold()
	onDemandThreshold := budget.OnDemandThreshold()

	var maxAlways, maxOnDemand int
	var worstAlwaysIdx, worstOnDemandIdx int
	for i, e := range entries {
		if e.AlwaysLoaded.EstimatedTokens > maxAlways {
			maxAlways = e.AlwaysLoaded.EstimatedTokens
			worstAlwaysIdx = i
		}
		if e.OnDemandMax.EstimatedTokens > maxOnDemand {
			maxOnDemand = e.OnDemandMax.EstimatedTokens
			worstOnDemandIdx = i
		}
	}

	var violations []budgetViolation
	if alwaysThreshold > 0 && maxAlways > alwaysThreshold && len(entries) > 0 {
		violations = append(violations, budgetViolation{
			Type:      "always_loaded",
			Actual:    maxAlways,
			Budget:    alwaysThreshold,
			Offenders: topOffenders(entries[worstAlwaysIdx].Skills, 3, true),
		})
	}
	if onDemandThreshold > 0 && maxOnDemand > onDemandThreshold && len(entries) > 0 {
		violations = append(violations, budgetViolation{
			Type:      "on_demand",
			Actual:    maxOnDemand,
			Budget:    onDemandThreshold,
			Offenders: topOffenders(entries[worstOnDemandIdx].Skills, 3, false),
		})
	}
	return violations
}

// topOffenders returns the top N skills by token count
func topOffenders(skills []analyzeSkillEntry, n int, byDescription bool) []tokenOffender {
	type pair struct {
		name   string
		tokens int
	}
	var pairs []pair
	for _, s := range skills {
		tokens := s.DescriptionTokens
		if !byDescription {
			tokens = s.BodyTokens
		}
		if tokens > 0 {
			pairs = append(pairs, pair{s.Name, tokens})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].tokens > pairs[j].tokens })
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	result := make([]tokenOffender, len(pairs))
	for i, p := range pairs {
		result[i] = tokenOffender{Name: p.name, Tokens: p.tokens}
	}
	return result
}

func printTokenSummary(entries []analyzeTargetEntry) {
	groups := groupByTokenCost(entries)
	for _, g := range groups {
		targets := strings.Join(g.Targets, ", ")
		fmt.Printf("  Context: ~%s always-loaded · ~%s on-demand (%s)\n",
			formatTokenK(g.AlwaysLoaded),
			formatTokenK(g.OnDemand),
			targets)
	}
}

func printBudgetWarning(violations []budgetViolation) {
	for _, v := range violations {
		label := "Always-loaded"
		if v.Type == "on_demand" {
			label = "On-demand"
		}
		ui.Warning("%s context is ~%s tokens (budget: %s)",
			label,
			formatTokenComma(v.Actual),
			formatTokenComma(v.Budget))
		if len(v.Offenders) > 0 {
			fmt.Printf("   Top %d:\n", len(v.Offenders))
			for _, o := range v.Offenders {
				fmt.Printf("     • %-35s ~%s tokens\n", o.Name, formatTokenComma(o.Tokens))
			}
		}
		fmt.Println("   Run `skillshare analyze` for details.")
	}
}

type contextCostJSON struct {
	Groups   []contextCostGroup   `json:"groups"`
	Warnings []contextCostWarning `json:"warnings,omitempty"`
}

type contextCostGroup struct {
	Targets            []string `json:"targets"`
	AlwaysLoadedTokens int      `json:"always_loaded_tokens"`
	OnDemandTokens     int      `json:"on_demand_tokens"`
}

type contextCostWarning struct {
	Type         string                `json:"type"`
	Actual       int                   `json:"actual"`
	Budget       int                   `json:"budget"`
	TopOffenders []contextCostOffender `json:"top_offenders"`
}

type contextCostOffender struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

func buildContextCostJSON(entries []analyzeTargetEntry, budget config.ContextBudgetConfig) *contextCostJSON {
	groups := groupByTokenCost(entries)
	result := &contextCostJSON{
		Groups: make([]contextCostGroup, len(groups)),
	}
	for i, g := range groups {
		result.Groups[i] = contextCostGroup{
			Targets:            g.Targets,
			AlwaysLoadedTokens: g.AlwaysLoaded,
			OnDemandTokens:     g.OnDemand,
		}
	}
	violations := checkBudget(entries, budget)
	for _, v := range violations {
		cw := contextCostWarning{
			Type:         v.Type,
			Actual:       v.Actual,
			Budget:       v.Budget,
			TopOffenders: make([]contextCostOffender, len(v.Offenders)),
		}
		for j, o := range v.Offenders {
			cw.TopOffenders[j] = contextCostOffender{Name: o.Name, Tokens: o.Tokens}
		}
		result.Warnings = append(result.Warnings, cw)
	}
	return result
}
