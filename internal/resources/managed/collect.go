package managed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/inspect"
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
)

// PreviewCollectRules reports which rule items would be collected.
func PreviewCollectRules(projectRoot string, items []inspect.RuleItem, force bool) (CollectPreviewResult, error) {
	return previewRuleCollect(projectRoot, items, force)
}

// CollectRules collects discovered rules into managed storage.
func CollectRules(projectRoot string, items []inspect.RuleItem, strategy managedrules.Strategy) (CollectResult, error) {
	result, err := managedrules.Collect(projectRoot, items, managedrules.CollectOptions{Strategy: strategy})
	if err != nil {
		return CollectResult{}, err
	}
	return CollectResult{
		Created:     append([]string{}, result.Created...),
		Overwritten: append([]string{}, result.Overwritten...),
		Skipped:     append([]string{}, result.Skipped...),
	}, nil
}

// PreviewCollectHooks reports which hook items would be collected.
func PreviewCollectHooks(projectRoot string, items []inspect.HookItem, force bool) (CollectPreviewResult, error) {
	return previewHookCollect(projectRoot, items, force)
}

// CollectHooks collects discovered hooks into managed storage.
func CollectHooks(projectRoot string, items []inspect.HookItem, strategy managedhooks.Strategy) (CollectResult, error) {
	result, err := managedhooks.Collect(projectRoot, items, managedhooks.CollectOptions{Strategy: strategy})
	if err != nil {
		return CollectResult{}, err
	}
	return CollectResult{
		Created:     append([]string{}, result.Created...),
		Overwritten: append([]string{}, result.Overwritten...),
		Skipped:     append([]string{}, result.Skipped...),
	}, nil
}

func previewRuleCollect(projectRoot string, items []inspect.RuleItem, force bool) (CollectPreviewResult, error) {
	store := managedrules.NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectPreviewResult{}, err
	}

	taken := make(map[string]struct{}, len(existing))
	for _, record := range existing {
		taken[record.ID] = struct{}{}
	}

	result := CollectPreviewResult{}
	seen := make(map[string]string)
	for _, item := range items {
		id, err := managedRuleCollectID(item)
		if err != nil {
			return CollectPreviewResult{}, err
		}
		if prior, ok := seen[id]; ok && prior != item.Path {
			return CollectPreviewResult{}, fmt.Errorf("collect managed rules: cannot collect %s and %s: canonical managed id %q collides", prior, item.Path, id)
		}
		seen[id] = item.Path

		if _, exists := taken[id]; exists && !force {
			result.Skipped = append(result.Skipped, id)
			continue
		}
		result.Pulled = append(result.Pulled, id)
		taken[id] = struct{}{}
	}
	return result, nil
}

func previewHookCollect(projectRoot string, items []inspect.HookItem, force bool) (CollectPreviewResult, error) {
	store := managedhooks.NewStore(projectRoot)
	existing, err := store.List()
	if err != nil {
		return CollectPreviewResult{}, err
	}

	taken := make(map[string]struct{}, len(existing))
	for _, record := range existing {
		taken[record.ID] = struct{}{}
	}

	result := CollectPreviewResult{}
	groups, err := groupCollectibleHooks(items)
	if err != nil {
		return CollectPreviewResult{}, err
	}

	seen := make(map[string]string)
	for _, group := range groups {
		id, err := managedHookCollectID(group.Tool, group.Event, group.Matcher)
		if err != nil {
			return CollectPreviewResult{}, err
		}
		if prior, ok := seen[id]; ok && prior != group.GroupID {
			return CollectPreviewResult{}, fmt.Errorf("collect managed hooks: cannot collect %s and %s: canonical managed id %q collides", prior, group.GroupID, id)
		}
		seen[id] = group.GroupID

		if _, exists := taken[id]; exists && !force {
			result.Skipped = append(result.Skipped, id)
			continue
		}
		result.Pulled = append(result.Pulled, id)
		taken[id] = struct{}{}
	}
	return result, nil
}

type collectibleHookGroup struct {
	GroupID string
	Tool    string
	Event   string
	Matcher string
}

func groupCollectibleHooks(items []inspect.HookItem) ([]collectibleHookGroup, error) {
	groupMap := make(map[string]collectibleHookGroup)
	order := make([]string, 0, len(items))
	for _, item := range items {
		groupID := strings.TrimSpace(item.GroupID)
		if groupID == "" {
			return nil, fmt.Errorf("collect managed hooks: cannot collect hook with empty group id")
		}
		group, exists := groupMap[groupID]
		if !exists {
			groupMap[groupID] = collectibleHookGroup{
				GroupID: groupID,
				Tool:    strings.TrimSpace(item.SourceTool),
				Event:   strings.TrimSpace(item.Event),
				Matcher: strings.TrimSpace(item.Matcher),
			}
			order = append(order, groupID)
			continue
		}
		if group.Tool != strings.TrimSpace(item.SourceTool) || group.Event != strings.TrimSpace(item.Event) || group.Matcher != strings.TrimSpace(item.Matcher) {
			return nil, fmt.Errorf("collect managed hooks: cannot collect %s: hook items disagree on source tool, event, or matcher", groupID)
		}
	}

	groups := make([]collectibleHookGroup, 0, len(order))
	for _, groupID := range order {
		groups = append(groups, groupMap[groupID])
	}
	return groups, nil
}

func managedRuleCollectID(item inspect.RuleItem) (string, error) {
	tool := strings.ToLower(strings.TrimSpace(item.SourceTool))
	if tool == "" {
		return "", fmt.Errorf("collect managed rules: cannot collect %s: missing source tool", item.Path)
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
		return "", fmt.Errorf("collect managed rules: cannot collect %s: invalid rule filename", item.Path)
	}
	return tool + "/" + base, nil
}

func managedHookCollectID(tool, event, matcher string) (string, error) {
	cleanTool := sanitizeHookPathSegment(tool)
	cleanEvent := sanitizeHookPathSegment(event)
	cleanMatcher := matcherIdentitySegment(matcher)
	if cleanTool == "" {
		return "", fmt.Errorf("collect managed hooks: cannot collect hook: missing tool")
	}
	if cleanEvent == "" {
		return "", fmt.Errorf("collect managed hooks: cannot collect hook: missing event")
	}
	if cleanMatcher == "" {
		return "", fmt.Errorf("collect managed hooks: cannot collect hook: missing matcher")
	}
	return path.Join(cleanTool, cleanEvent, cleanMatcher+".yaml"), nil
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
