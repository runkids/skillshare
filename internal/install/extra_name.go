package install

import (
	"fmt"
	"regexp"
)

var extraNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidateExtraName checks the shared config and installation directory rules.
func ValidateExtraName(name string) error {
	if name == "" {
		return fmt.Errorf("extra name cannot be empty")
	}
	if name == "skills" || name == "extras" {
		return fmt.Errorf("extra name %q is reserved", name)
	}
	if !extraNamePattern.MatchString(name) {
		return fmt.Errorf("extra name is invalid: must start with a letter or digit and contain only letters, digits, hyphens, or underscores")
	}
	return nil
}
