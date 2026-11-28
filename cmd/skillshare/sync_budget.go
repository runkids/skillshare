package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

const (
	budgetTypeAlwaysLoaded = "always_loaded"
	budgetTypeOnDemand     = "on_demand"
)

func formatTokenK(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}

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
	Target    string
	Actual    int
	Budget    int
	Offenders []tokenOffender
}

type tokenOffender struct {
	Name   string
	Tokens int
}

func checkBudget(entries []analyzeTargetEntry, budget config.ContextBudgetConfig) []budgetViolation {
	alwaysThreshold := budget.AlwaysLoadedThreshold()
	onDemandThreshold := budget.OnDemandThreshold()

	seen := make(map[string]bool)
	var violations []budgetViolation
	for _, e := range entries {
		if alwaysThreshold > 0 && e.AlwaysLoaded.EstimatedTokens > alwaysThreshold {
			key := budgetTypeAlwaysLoaded + ":" + e.Name
			if !seen[key] {
				seen[key] = true
				violations = append(violations, budgetViolation{
					Type:      budgetTypeAlwaysLoaded,
					Target:    e.Name,
					Actual:    e.AlwaysLoaded.EstimatedTokens,
					Budget:    alwaysThreshold,
					Offenders: topOffenders(e.Skills, 3, true),
				})
			}
		}
		if onDemandThreshold > 0 && e.OnDemandMax.EstimatedTokens > onDemandThreshold {
			key := budgetTypeOnDemand + ":" + e.Name
			if !seen[key] {
				seen[key] = true
				violations = append(violations, budgetViolation{
					Type:      budgetTypeOnDemand,
					Target:    e.Name,
					Actual:    e.OnDemandMax.EstimatedTokens,
					Budget:    onDemandThreshold,
					Offenders: topOffenders(e.Skills, 3, false),
				})
			}
		}
	}
	return violations
}

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
			pairs = append(pairs, pair{s.label(), tokens})
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
		line := fmt.Sprintf("Context  ~%s tokens always loaded · ~%s on demand",
			formatTokenK(g.AlwaysLoaded), formatTokenK(g.OnDemand))
		if len(groups) > 1 {
			line += " (" + strings.Join(g.Targets, ", ") + ")"
		}
		ui.Note(line)
	}
}

// printBudgetWarning prints one warning per budget type. Targets loading the same
// amount share a line, largest first, and the details name the three largest skills
// across them once: targets reading one folder would otherwise repeat the same block.
func printBudgetWarning(violations []budgetViolation, showDetails bool) {
	for _, typ := range []string{budgetTypeAlwaysLoaded, budgetTypeOnDemand} {
		label := "Always-loaded"
		if typ == budgetTypeOnDemand {
			label = "On-demand"
		}
		var amounts []int
		targets := map[int][]string{}
		largest := map[string]int{}
		budget := 0
		for _, v := range violations {
			if v.Type != typ {
				continue
			}
			budget = v.Budget
			if len(targets[v.Actual]) == 0 {
				amounts = append(amounts, v.Actual)
			}
			targets[v.Actual] = append(targets[v.Actual], v.Target)
			for _, o := range v.Offenders {
				largest[o.Name] = max(largest[o.Name], o.Tokens)
			}
		}
		if len(amounts) == 0 {
			continue
		}
		sort.Sort(sort.Reverse(sort.IntSlice(amounts)))
		count := 0
		for _, names := range targets {
			count += len(names)
		}
		ui.Warning("%s context is over budget (%s tokens) on %s",
			label, formatTokenComma(budget), plural(count, "target"))
		for _, amount := range amounts {
			names := targets[amount]
			sort.Strings(names)
			fmt.Printf("   %-35s ~%s tokens\n", strings.Join(names, ", "), formatTokenComma(amount))
		}
		if showDetails {
			if top := largestSkills(largest, 3); top != "" {
				fmt.Printf("   Largest: %s\n", top)
			}
			fmt.Println("   Run `skillshare analyze` for details.")
		}
	}
}

// largestSkills lists the n skills with the most tokens, largest first.
func largestSkills(tokens map[string]int, n int) string {
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if tokens[names[i]] != tokens[names[j]] {
			return tokens[names[i]] > tokens[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, 0, n)
	for _, name := range names[:min(n, len(names))] {
		parts = append(parts, fmt.Sprintf("%s ~%s", name, formatTokenComma(tokens[name])))
	}
	return strings.Join(parts, ", ")
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
	Target       string                `json:"target"`
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
			Target:       v.Target,
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
