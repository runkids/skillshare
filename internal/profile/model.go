// Package profile owns profile definitions and selection rules.
package profile

import (
	"fmt"
	"regexp"
	"strings"

	"skillshare/internal/pattern"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type SkillSelection struct {
	Include        []string `json:"include"`
	Exclude        []string `json:"exclude"`
	IncludePresent bool     `json:"-"`
	ExcludePresent bool     `json:"-"`
}

type Definition struct {
	Name, Description string
	Skills            *SkillSelection
	Usable            bool
	Reason            string
	unknown           bool
}

type Catalog struct {
	Present     bool
	Definitions map[string]Definition
	Reason      string
}

type SelectionPreview struct {
	Selected, Unmatched, InertExcludes []string
}

// Preview lists the selected names and include patterns that match no name.
func Preview(d Definition, names []string) (SelectionPreview, error) {
	var result SelectionPreview
	if !d.Usable {
		return result, fmt.Errorf("profile %q is unusable: %s", d.Name, d.Reason)
	}
	for _, name := range names {
		if d.Selects(name) {
			result.Selected = append(result.Selected, name)
		}
	}
	if d.Skills == nil {
		return result, nil
	}
	include, exclude, err := pattern.Normalize(d.Skills.Include, d.Skills.Exclude)
	if err != nil {
		return result, err
	}
	if len(include) == 0 {
		result.InertExcludes = exclude
	}
	for _, p := range include {
		found := false
		for _, name := range names {
			if _, matched := pattern.FirstMatch(name, []string{p}); matched {
				found = true
				break
			}
		}
		if !found {
			result.Unmatched = append(result.Unmatched, p)
		}
	}
	return result, nil
}

func ValidateName(name string) error {
	if name == "none" || !validName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use lowercase letters, digits, underscores or hyphens (1–64 characters); none is reserved", name)
	}
	return nil
}

func ValidateDefinition(d Definition) error {
	if err := ValidateName(d.Name); err != nil {
		return err
	}
	if strings.ContainsAny(d.Description, "\r\n") {
		return fmt.Errorf("profile %q: description must be one line", d.Name)
	}
	if d.Skills == nil {
		return nil
	}
	if d.Skills.ExcludePresent && !d.Skills.IncludePresent {
		return fmt.Errorf("profile %q: exclude requires an include; add include: [\"*\"]", d.Name)
	}
	_, _, err := pattern.Normalize(d.Skills.Include, d.Skills.Exclude)
	return err
}

// Selects applies the profile's additional empty-include rule.
func (d Definition) Selects(name string) bool {
	if !d.Usable {
		return false
	}
	if d.Skills == nil {
		return true
	}
	include, exclude, err := pattern.Normalize(d.Skills.Include, d.Skills.Exclude)
	return err == nil && len(include) > 0 && pattern.Match(name, include, exclude)
}
