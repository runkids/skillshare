package hooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var gitFriendlyName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
var gitEventName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var gitCompoundEnd = regexp.MustCompile(`(^|[;[:space:]])(fi|done|esac)$|[})]$`)

func validGitFriendlyName(name string) bool {
	return gitFriendlyName.MatchString(name) && !strings.Contains(name, "..") && !strings.HasSuffix(name, ".") && !knownEvent("git", name)
}

func validateGit(b Binding) error {
	if b.Events != nil || b.Code != "" {
		return fmt.Errorf("takes commands and optional files, not events or code")
	}
	if b.Scripts != nil {
		return fmt.Errorf("scripts are not supported yet; use config commands")
	}
	if len(b.Commands) == 0 {
		return fmt.Errorf("requires at least one command")
	}
	for _, name := range sortedKeys(b.Commands) {
		c := b.Commands[name]
		if !gitFriendlyName.MatchString(name) || strings.Contains(name, "..") || strings.HasSuffix(name, ".") {
			return fmt.Errorf("invalid friendly name %q", name)
		}
		if knownEvent("git", name) {
			return fmt.Errorf("friendly name %q is a reserved Git hook event; use a tool prefix", name)
		}
		if len(c.Events) == 0 {
			return fmt.Errorf("command %s requires events", name)
		}
		seen := map[string]bool{}
		for _, event := range c.Events {
			if !gitEventName.MatchString(event) || seen[event] {
				return fmt.Errorf("command %s: invalid or duplicate event %q", name, event)
			}
			seen[event] = true
		}
		if strings.TrimSpace(c.Command) == "" || strings.ContainsAny(c.Command, "\n\r\x00") {
			return fmt.Errorf("command %s must be non-empty and contain no newline, carriage return or NUL", name)
		}
		if gitCompoundEnd.MatchString(strings.TrimSpace(c.Command)) {
			return fmt.Errorf("command %s ends in a shell compound: Git appends arguments; put the script in files", name)
		}
		if strings.Contains(c.Command, "{files}") && len(b.Files) == 0 {
			return fmt.Errorf("command %s uses {files} without files", name)
		}
	}
	for file := range b.Files {
		if !fileName.MatchString(file) || !filepath.IsLocal(file) {
			return fmt.Errorf("file %q must be a plain file name", file)
		}
	}
	return nil
}
