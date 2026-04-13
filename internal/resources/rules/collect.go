package rules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/inspect"
	managedpi "skillshare/internal/resources/managed/pi"
)

type Strategy string

const (
	StrategySkip      Strategy = "skip"
	StrategyOverwrite Strategy = "overwrite"
	StrategyDuplicate Strategy = "duplicate"
)

type CollectOptions struct {
	Strategy Strategy
}

type CollectResult struct {
	Created     []string
	Overwritten []string
	Skipped     []string
}

type collectAppliedWrite struct {
	id        string
	hadPrior  bool
	priorRule Record
}

type collectPlan struct {
	result CollectResult
	writes []Save
}

func invalidCollectf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCollect, fmt.Sprintf(format, args...))
}

// PreviewCollect validates discovered rules and reports what Collect would do.
func PreviewCollect(projectRoot string, discovered []inspect.RuleItem, opts CollectOptions) (CollectResult, error) {
	strategy, err := normalizeStrategy(opts.Strategy)
	if err != nil {
		return CollectResult{}, err
	}

	store := NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectResult{}, err
	}

	plan, err := planCollect(existing, discovered, strategy)
	if err != nil {
		return CollectResult{}, err
	}
	return plan.result, nil
}

// Collect imports discovered rule files into managed rules storage.
func Collect(projectRoot string, discovered []inspect.RuleItem, opts CollectOptions) (CollectResult, error) {
	strategy, err := normalizeStrategy(opts.Strategy)
	if err != nil {
		return CollectResult{}, err
	}

	store := NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectResult{}, err
	}

	plan, err := planCollect(existing, discovered, strategy)
	if err != nil {
		return CollectResult{}, err
	}

	existingByID := make(map[string]Record, len(existing)+len(plan.writes))
	for _, record := range existing {
		existingByID[record.ID] = record
	}

	applied := make([]collectAppliedWrite, 0, len(plan.writes))
	for _, write := range plan.writes {
		prior, hadPrior := existingByID[write.ID]
		applied = append(applied, collectAppliedWrite{
			id:        write.ID,
			hadPrior:  hadPrior,
			priorRule: prior,
		})

		record, err := store.Put(write)
		if err != nil {
			rollbackErr := rollbackAppliedWrites(store, applied[:len(applied)-1])
			if rollbackErr != nil {
				return CollectResult{}, fmt.Errorf("apply collected rules: %w; rollback failed: %v", err, rollbackErr)
			}
			return CollectResult{}, err
		}
		existingByID[write.ID] = record
	}

	return plan.result, nil
}

func planCollect(existing []Record, discovered []inspect.RuleItem, strategy Strategy) (collectPlan, error) {
	if err := rejectCanonicalManagedIDCollisions(discovered); err != nil {
		return collectPlan{}, err
	}

	takenIDs := make(map[string]bool, len(existing)+len(discovered))
	for _, record := range existing {
		takenIDs[record.ID] = true
	}

	plan := collectPlan{
		result: CollectResult{},
		writes: make([]Save, 0, len(discovered)),
	}

	for _, item := range discovered {
		if !item.Collectible {
			reason := strings.TrimSpace(item.CollectReason)
			if reason == "" {
				reason = "rule is not collectible"
			}
			return collectPlan{}, invalidCollectf("cannot collect %s: %s", item.Path, reason)
		}

		id, err := managedIDForDiscoveredRule(item)
		if err != nil {
			return collectPlan{}, err
		}
		exists := takenIDs[id]

		switch {
		case !exists:
			plan.writes = append(plan.writes, Save{
				ID:         id,
				Content:    []byte(item.Content),
				SourceType: "local",
			})
			takenIDs[id] = true
			plan.result.Created = append(plan.result.Created, id)
		case strategy == StrategySkip:
			plan.result.Skipped = append(plan.result.Skipped, id)
		case strategy == StrategyOverwrite:
			prior := managedRuleByID(existing, id)
			plan.writes = append(plan.writes, Save{
				ID:         id,
				Content:    []byte(item.Content),
				Targets:    append([]string(nil), prior.Targets...),
				SourceType: prior.SourceType,
				Disabled:   prior.Disabled,
			})
			takenIDs[id] = true
			plan.result.Overwritten = append(plan.result.Overwritten, id)
		case strategy == StrategyDuplicate:
			if managedpi.IsManagedRuleID(id) {
				return collectPlan{}, invalidCollectf("cannot collect %s: managed pi rules use fixed instruction surfaces", item.Path)
			}
			duplicateID := nextDuplicateIDFromTaken(takenIDs, id)
			plan.writes = append(plan.writes, Save{
				ID:         duplicateID,
				Content:    []byte(item.Content),
				SourceType: "local",
			})
			takenIDs[duplicateID] = true
			plan.result.Created = append(plan.result.Created, duplicateID)
		}
	}

	return plan, nil
}

func rejectCanonicalManagedIDCollisions(discovered []inspect.RuleItem) error {
	byID := make(map[string]inspect.RuleItem, len(discovered))
	for _, item := range discovered {
		id, err := managedIDForDiscoveredRule(item)
		if err != nil {
			return err
		}
		if prior, ok := byID[id]; ok && prior.Path != item.Path {
			return invalidCollectf("cannot collect %s and %s: canonical managed id %q collides", prior.Path, item.Path, id)
		}
		byID[id] = item
	}
	return nil
}

func rollbackAppliedWrites(store *Store, applied []collectAppliedWrite) error {
	var firstErr error
	for i := len(applied) - 1; i >= 0; i-- {
		entry := applied[i]
		if entry.hadPrior {
			if _, err := store.Put(Save{
				ID:         entry.priorRule.ID,
				Content:    entry.priorRule.Content,
				Targets:    append([]string(nil), entry.priorRule.Targets...),
				SourceType: entry.priorRule.SourceType,
				Disabled:   entry.priorRule.Disabled,
			}); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := store.Delete(entry.id); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func managedRuleByID(existing []Record, id string) Record {
	for _, record := range existing {
		if record.ID == id {
			return record
		}
	}
	return Record{}
}

func normalizeStrategy(strategy Strategy) (Strategy, error) {
	switch strings.TrimSpace(string(strategy)) {
	case "":
		return StrategySkip, nil
	case string(StrategySkip):
		return StrategySkip, nil
	case string(StrategyOverwrite):
		return StrategyOverwrite, nil
	case string(StrategyDuplicate):
		return StrategyDuplicate, nil
	default:
		return "", invalidCollectf("invalid collect strategy %q", strategy)
	}
}

func nextDuplicateIDFromTaken(taken map[string]bool, id string) string {
	ext := path.Ext(id)
	base := strings.TrimSuffix(path.Base(id), ext)
	dir := path.Dir(id)
	if dir == "." {
		dir = ""
	}

	candidateFor := func(suffix string) string {
		name := base + suffix + ext
		if dir == "" {
			return name
		}
		return path.Join(dir, name)
	}

	first := candidateFor("-copy")
	if !taken[first] {
		return first
	}

	for i := 2; ; i++ {
		candidate := candidateFor(fmt.Sprintf("-copy-%d", i))
		if !taken[candidate] {
			return candidate
		}
	}
}

// ManagedIDForDiscoveredRule returns the canonical managed rule ID for one discovered rule.
func ManagedIDForDiscoveredRule(item inspect.RuleItem) (string, error) {
	return managedIDForDiscoveredRule(item)
}

func managedIDForDiscoveredRule(item inspect.RuleItem) (string, error) {
	tool := strings.ToLower(strings.TrimSpace(item.SourceTool))
	if tool == "" {
		return "", invalidCollectf("cannot collect %s: missing source tool", item.Path)
	}

	p := filepath.ToSlash(strings.TrimSpace(item.Path))
	base := path.Base(p)

	switch tool {
	case "claude":
		if rel, ok := relativeAfterSegment(p, "/.claude/rules/"); ok {
			return "claude/" + rel, nil
		}
		if strings.EqualFold(base, "CLAUDE.md") {
			return "claude/CLAUDE.md", nil
		}
	case "codex":
		if rel, ok := relativeAfterSegment(p, "/.codex/rules/"); ok {
			return "codex/" + rel, nil
		}
		if strings.EqualFold(base, "AGENTS.md") {
			return "codex/AGENTS.md", nil
		}
	case "gemini":
		if rel, ok := relativeAfterSegment(p, "/.gemini/rules/"); ok {
			return "gemini/" + rel, nil
		}
		if strings.EqualFold(base, "GEMINI.md") {
			return "gemini/GEMINI.md", nil
		}
	case "pi":
		if id, ok := managedpi.ManagedRuleIDForDiscoveredPath(p); ok {
			return id, nil
		}
		return "", invalidCollectf("cannot collect %s: unsupported pi rule path", item.Path)
	}

	if base == "." || base == "/" || strings.TrimSpace(base) == "" {
		return "", invalidCollectf("cannot collect %s: invalid rule filename", item.Path)
	}
	return tool + "/" + base, nil
}

func relativeAfterSegment(p string, segment string) (string, bool) {
	lowerPath := strings.ToLower(p)
	lowerSegment := strings.ToLower(segment)
	idx := strings.Index(lowerPath, lowerSegment)
	if idx < 0 {
		return "", false
	}
	rel := p[idx+len(segment):]
	if strings.TrimSpace(rel) == "" {
		return "", false
	}
	rel = path.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return "", false
	}
	return rel, true
}
