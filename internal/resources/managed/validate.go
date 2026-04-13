package managed

import (
	"fmt"
	"strings"
)

type RuleInput struct {
	Tool         string
	RelativePath string
	Content      []byte
}

type HookHandlerInput struct {
	Type           string
	Name           string
	Description    string
	Command        string
	URL            string
	Prompt         string
	Timeout        string
	TimeoutSeconds *int
	StatusMessage  string
}

type HookInput struct {
	Tool     string
	Event    string
	Matcher  string
	Handlers []HookHandlerInput
}

func validateManagedFamily(kind ResourceKind, targetName, targetPath string) error {
	if strings.TrimSpace(targetName) == "" && strings.TrimSpace(targetPath) == "" {
		return nil
	}
	if _, ok := ResolveManagedFamily(kind, targetName, targetPath); ok {
		return nil
	}
	return fmt.Errorf("tool %q does not support managed %s", strings.TrimSpace(targetName), kind)
}

// ValidateManagedRuleSave checks whether a managed rule mutation targets a supported family.
func ValidateManagedRuleSave(in RuleInput) error {
	return validateManagedFamily(ResourceKindRules, in.Tool, "")
}

// ValidateManagedHookSave checks whether a managed hook mutation targets a supported family.
func ValidateManagedHookSave(in HookInput) error {
	return validateManagedFamily(ResourceKindHooks, in.Tool, "")
}
