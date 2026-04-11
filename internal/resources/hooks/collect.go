package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"sort"
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

type collectedHookGroup struct {
	GroupID       string
	SourceTool    string
	Scope         inspect.Scope
	Event         string
	Matcher       string
	Path          string
	Collectible   bool
	CollectReason string
	Items         []inspect.HookItem
}

type collectAppliedWrite struct {
	id          string
	hadPrior    bool
	priorRecord Record
}

// Collect imports discovered hook files into managed hook storage.
func Collect(projectRoot string, discovered []inspect.HookItem, opts CollectOptions) (CollectResult, error) {
	strategy, err := normalizeStrategy(opts.Strategy)
	if err != nil {
		return CollectResult{}, err
	}

	store := NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectResult{}, err
	}

	takenIDs := make(map[string]bool, len(existing)+len(discovered))
	existingByID := make(map[string]Record, len(existing))
	for _, record := range existing {
		takenIDs[record.ID] = true
		existingByID[record.ID] = record
	}

	groups, err := groupInspectHooks(discovered)
	if err != nil {
		return CollectResult{}, err
	}
	if err := rejectCanonicalIDCollisions(groups); err != nil {
		return CollectResult{}, err
	}
	result := CollectResult{}
	plannedWrites := make([]Save, 0, len(groups))
	for _, group := range groups {
		if !group.Collectible {
			reason := strings.TrimSpace(group.CollectReason)
			if reason == "" {
				reason = "hook group is not collectible"
			}
			return CollectResult{}, fmt.Errorf("cannot collect %s: %s", group.GroupID, reason)
		}

		id, err := canonicalRelativePath(group.SourceTool, group.Event, group.Matcher)
		if err != nil {
			return CollectResult{}, err
		}

		save := Save{
			ID:       id,
			Tool:     strings.ToLower(strings.TrimSpace(group.SourceTool)),
			Event:    strings.TrimSpace(group.Event),
			Matcher:  strings.TrimSpace(group.Matcher),
			Handlers: handlersFromInspectHooks(sortedHookItems(group.Items)),
		}

		exists := takenIDs[id]
		switch {
		case !exists:
			plannedWrites = append(plannedWrites, save)
			takenIDs[id] = true
			result.Created = append(result.Created, id)
		case strategy == StrategySkip:
			result.Skipped = append(result.Skipped, id)
		case strategy == StrategyOverwrite:
			plannedWrites = append(plannedWrites, save)
			result.Overwritten = append(result.Overwritten, id)
		case strategy == StrategyDuplicate:
			duplicateID := nextDuplicateIDFromTaken(takenIDs, id)
			save.ID = duplicateID
			plannedWrites = append(plannedWrites, save)
			takenIDs[duplicateID] = true
			result.Created = append(result.Created, duplicateID)
		}
	}

	applied := make([]collectAppliedWrite, 0, len(plannedWrites))
	for _, write := range plannedWrites {
		prior, hadPrior := existingByID[write.ID]
		applied = append(applied, collectAppliedWrite{
			id:          write.ID,
			hadPrior:    hadPrior,
			priorRecord: prior,
		})

		if _, err := store.Put(write); err != nil {
			rollbackErr := rollbackAppliedHookWrites(store, applied[:len(applied)-1])
			if rollbackErr != nil {
				return CollectResult{}, fmt.Errorf("apply collected hooks: %w; rollback failed: %v", err, rollbackErr)
			}
			return CollectResult{}, err
		}
		existingByID[write.ID] = Record{
			ID:           write.ID,
			RelativePath: write.ID,
			Tool:         write.Tool,
			Event:        write.Event,
			Matcher:      write.Matcher,
			Handlers:     append([]Handler(nil), write.Handlers...),
		}
	}

	return result, nil
}

func groupInspectHooks(discovered []inspect.HookItem) ([]collectedHookGroup, error) {
	groups := make(map[string]*collectedHookGroup)
	order := make([]string, 0, len(discovered))

	for _, item := range discovered {
		groupID := strings.TrimSpace(item.GroupID)
		if groupID == "" {
			return nil, fmt.Errorf("cannot collect hook with empty group id")
		}
		if strings.TrimSpace(item.SourceTool) == "" {
			return nil, fmt.Errorf("cannot collect %s: missing source tool", groupID)
		}
		if strings.TrimSpace(item.Event) == "" {
			return nil, fmt.Errorf("cannot collect %s: missing event", groupID)
		}
		if strings.TrimSpace(item.Matcher) == "" && strings.ToLower(strings.TrimSpace(item.SourceTool)) != "codex" {
			return nil, fmt.Errorf("cannot collect %s: missing matcher", groupID)
		}

		group, ok := groups[groupID]
		if !ok {
			copy := collectedHookGroup{
				GroupID:       groupID,
				SourceTool:    strings.TrimSpace(item.SourceTool),
				Scope:         item.Scope,
				Event:         strings.TrimSpace(item.Event),
				Matcher:       strings.TrimSpace(item.Matcher),
				Path:          strings.TrimSpace(item.Path),
				Collectible:   item.Collectible,
				CollectReason: strings.TrimSpace(item.CollectReason),
				Items:         []inspect.HookItem{item},
			}
			groups[groupID] = &copy
			order = append(order, groupID)
			continue
		}

		if group.SourceTool != strings.TrimSpace(item.SourceTool) || group.Event != strings.TrimSpace(item.Event) || group.Matcher != strings.TrimSpace(item.Matcher) {
			return nil, fmt.Errorf("cannot collect %s: hook items disagree on source tool, event, or matcher", groupID)
		}
		if group.Path != strings.TrimSpace(item.Path) {
			return nil, fmt.Errorf("cannot collect %s: hook items disagree on source path", groupID)
		}
		if group.Collectible != item.Collectible || group.CollectReason != strings.TrimSpace(item.CollectReason) {
			return nil, fmt.Errorf("cannot collect %s: hook items disagree on collectibility", groupID)
		}
		group.Items = append(group.Items, item)
	}

	out := make([]collectedHookGroup, 0, len(order))
	for _, groupID := range order {
		group := groups[groupID]
		if len(group.Items) == 0 {
			return nil, fmt.Errorf("cannot collect %s: missing hook actions", group.GroupID)
		}
		out = append(out, *group)
	}
	return out, nil
}

func rejectCanonicalIDCollisions(groups []collectedHookGroup) error {
	byID := make(map[string]collectedHookGroup, len(groups))
	for _, group := range groups {
		id, err := canonicalRelativePath(group.SourceTool, group.Event, group.Matcher)
		if err != nil {
			return err
		}
		if prior, ok := byID[id]; ok && prior.GroupID != group.GroupID {
			return fmt.Errorf("cannot collect %s and %s: canonical managed id %q collides", prior.GroupID, group.GroupID, id)
		}
		byID[id] = group
	}
	return nil
}

func handlersFromInspectHooks(items []inspect.HookItem) []Handler {
	if len(items) == 0 {
		return nil
	}
	handlers := make([]Handler, 0, len(items))
	for _, item := range items {
		handlers = append(handlers, Handler{
			Type:           strings.TrimSpace(item.ActionType),
			Command:        strings.TrimSpace(item.Command),
			URL:            strings.TrimSpace(item.URL),
			Prompt:         strings.TrimSpace(item.Prompt),
			Timeout:        strings.TrimSpace(item.Timeout),
			TimeoutSeconds: item.TimeoutSeconds,
			StatusMessage:  strings.TrimSpace(item.StatusMessage),
		})
	}
	return handlers
}

func sortedHookItems(items []inspect.HookItem) []inspect.HookItem {
	if len(items) < 2 {
		return append([]inspect.HookItem(nil), items...)
	}

	sorted := append([]inspect.HookItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].EntryIndex != sorted[j].EntryIndex {
			return sorted[i].EntryIndex < sorted[j].EntryIndex
		}
		return sorted[i].ActionIndex < sorted[j].ActionIndex
	})
	return sorted
}

func rollbackAppliedHookWrites(store *Store, applied []collectAppliedWrite) error {
	var firstErr error
	for i := len(applied) - 1; i >= 0; i-- {
		entry := applied[i]
		if entry.hadPrior {
			if _, err := store.Put(Save{
				ID:       entry.priorRecord.ID,
				Tool:     entry.priorRecord.Tool,
				Event:    entry.priorRecord.Event,
				Matcher:  entry.priorRecord.Matcher,
				Handlers: entry.priorRecord.Handlers,
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
		return "", fmt.Errorf("invalid collect strategy %q", strategy)
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

func canonicalRelativePath(tool, event, matcher string) (string, error) {
	cleanTool := sanitizeHookPathSegment(tool)
	cleanEvent := sanitizeHookPathSegment(event)
	cleanMatcher := matcherIdentitySegment(matcher)
	if cleanTool == "" {
		return "", fmt.Errorf("cannot collect hook: missing tool")
	}
	if cleanEvent == "" {
		return "", fmt.Errorf("cannot collect hook: missing event")
	}
	if cleanMatcher == "" {
		return "", fmt.Errorf("cannot collect hook: missing matcher")
	}
	return path.Join(cleanTool, cleanEvent, cleanMatcher+".yaml"), nil
}

func matcherIdentitySegment(matcher string) string {
	raw := strings.TrimSpace(matcher)
	slug := sanitizeHookPathSegment(raw)
	if slug == "" {
		slug = "matcher"
	}
	sum := sha256.Sum256([]byte(raw))
	return slug + "-" + hex.EncodeToString(sum[:6])
}

func sanitizeHookPathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var b strings.Builder
	needDash := false
	for i, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			if i > 0 && !needDash {
				b.WriteByte('-')
			}
			b.WriteByte(byte(r + ('a' - 'A')))
			needDash = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			needDash = false
		default:
			if !needDash {
				b.WriteByte('-')
				needDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}
