package rules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/inspect"
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
	id              string
	hadPriorContent bool
	priorContent    []byte
}

func invalidCollectf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCollect, fmt.Sprintf(format, args...))
}

// Collect imports discovered rule files into managed rules storage.
func Collect(projectRoot string, discovered []inspect.RuleItem, opts CollectOptions) (CollectResult, error) {
	strategy, err := normalizeStrategy(opts.Strategy)
	if err != nil {
		return CollectResult{}, err
	}
	if err := rejectCanonicalManagedIDCollisions(discovered); err != nil {
		return CollectResult{}, err
	}

	store := NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectResult{}, err
	}

	takenIDs := make(map[string]bool, len(existing)+len(discovered))
	for _, record := range existing {
		takenIDs[record.ID] = true
	}

	type plannedWrite struct {
		id      string
		content []byte
	}
	plannedWrites := make([]plannedWrite, 0, len(discovered))
	result := CollectResult{}

	for _, item := range discovered {
		if !item.Collectible {
			reason := strings.TrimSpace(item.CollectReason)
			if reason == "" {
				reason = "rule is not collectible"
			}
			return CollectResult{}, invalidCollectf("cannot collect %s: %s", item.Path, reason)
		}

		id, err := managedIDForDiscoveredRule(item)
		if err != nil {
			return CollectResult{}, err
		}
		exists := takenIDs[id]

		switch {
		case !exists:
			plannedWrites = append(plannedWrites, plannedWrite{
				id:      id,
				content: []byte(item.Content),
			})
			takenIDs[id] = true
			result.Created = append(result.Created, id)
		case strategy == StrategySkip:
			result.Skipped = append(result.Skipped, id)
		case strategy == StrategyOverwrite:
			plannedWrites = append(plannedWrites, plannedWrite{
				id:      id,
				content: []byte(item.Content),
			})
			takenIDs[id] = true
			result.Overwritten = append(result.Overwritten, id)
		case strategy == StrategyDuplicate:
			duplicateID := nextDuplicateIDFromTaken(takenIDs, id)
			plannedWrites = append(plannedWrites, plannedWrite{
				id:      duplicateID,
				content: []byte(item.Content),
			})
			takenIDs[duplicateID] = true
			result.Created = append(result.Created, duplicateID)
		}
	}

	currentContent := make(map[string][]byte, len(existing)+len(plannedWrites))
	for _, record := range existing {
		currentContent[record.ID] = append([]byte(nil), record.Content...)
	}

	applied := make([]collectAppliedWrite, 0, len(plannedWrites))
	for _, write := range plannedWrites {
		prior, hadPrior := currentContent[write.id]
		applied = append(applied, collectAppliedWrite{
			id:              write.id,
			hadPriorContent: hadPrior,
			priorContent:    append([]byte(nil), prior...),
		})

		if _, err := store.Put(Save{ID: write.id, Content: write.content}); err != nil {
			rollbackErr := rollbackAppliedWrites(store, applied[:len(applied)-1])
			if rollbackErr != nil {
				return CollectResult{}, fmt.Errorf("apply collected rules: %w; rollback failed: %v", err, rollbackErr)
			}
			return CollectResult{}, err
		}
		currentContent[write.id] = append([]byte(nil), write.content...)
	}

	return result, nil
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
		if entry.hadPriorContent {
			if _, err := store.Put(Save{ID: entry.id, Content: entry.priorContent}); err != nil && firstErr == nil {
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
