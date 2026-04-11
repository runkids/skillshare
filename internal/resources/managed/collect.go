package managed

import (
	"fmt"

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
		return CollectResult{}, fmt.Errorf("collect managed rules: %w", err)
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
		return CollectResult{}, fmt.Errorf("collect managed hooks: %w", err)
	}
	return CollectResult{
		Created:     append([]string{}, result.Created...),
		Overwritten: append([]string{}, result.Overwritten...),
		Skipped:     append([]string{}, result.Skipped...),
	}, nil
}

func previewRuleCollect(projectRoot string, items []inspect.RuleItem, force bool) (CollectPreviewResult, error) {
	strategy := managedrules.StrategySkip
	if force {
		strategy = managedrules.StrategyOverwrite
	}
	result, err := managedrules.PreviewCollect(projectRoot, items, managedrules.CollectOptions{Strategy: strategy})
	if err != nil {
		return CollectPreviewResult{}, fmt.Errorf("collect managed rules: %w", err)
	}
	return previewResultFromRuleCollect(result), nil
}

func previewHookCollect(projectRoot string, items []inspect.HookItem, force bool) (CollectPreviewResult, error) {
	strategy := managedhooks.StrategySkip
	if force {
		strategy = managedhooks.StrategyOverwrite
	}
	result, err := managedhooks.PreviewCollect(projectRoot, items, managedhooks.CollectOptions{Strategy: strategy})
	if err != nil {
		return CollectPreviewResult{}, fmt.Errorf("collect managed hooks: %w", err)
	}
	return previewResultFromHookCollect(result), nil
}

func managedRuleCollectID(item inspect.RuleItem) (string, error) {
	return managedrules.ManagedIDForDiscoveredRule(item)
}

func managedHookCollectID(tool, event, matcher string) (string, error) {
	return managedhooks.CanonicalRelativePath(tool, event, matcher)
}

func previewResultFromRuleCollect(result managedrules.CollectResult) CollectPreviewResult {
	pulled := append([]string{}, result.Created...)
	pulled = append(pulled, result.Overwritten...)
	return CollectPreviewResult{
		Pulled:  pulled,
		Skipped: append([]string{}, result.Skipped...),
	}
}

func previewResultFromHookCollect(result managedhooks.CollectResult) CollectPreviewResult {
	pulled := append([]string{}, result.Created...)
	pulled = append(pulled, result.Overwritten...)
	return CollectPreviewResult{
		Pulled:  pulled,
		Skipped: append([]string{}, result.Skipped...),
	}
}
