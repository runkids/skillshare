package main

import (
	"fmt"
	"strings"
)

type resourceSelection struct {
	skills bool
	rules  bool
	hooks  bool
}

type resourceFlagOptions struct {
	defaultSelection resourceSelection
	allowAll         bool
}

func parseResourceFlags(args []string, opts resourceFlagOptions) (resourceSelection, []string, error) {
	selection := opts.defaultSelection
	rest := make([]string, 0, len(args))

	var sawResources bool
	var sawAll bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--resources":
			if i+1 >= len(args) {
				return resourceSelection{}, nil, fmt.Errorf("--resources requires a comma-separated value")
			}
			if !sawResources {
				selection = resourceSelection{}
				sawResources = true
			}
			if sawAll {
				return resourceSelection{}, nil, fmt.Errorf("--all and --resources cannot be used together")
			}
			if err := selection.addCSV(args[i+1]); err != nil {
				return resourceSelection{}, nil, err
			}
			i++
		case "--all":
			if opts.allowAll {
				if sawResources {
					return resourceSelection{}, nil, fmt.Errorf("--all and --resources cannot be used together")
				}
				sawAll = true
				selection = allResources()
				continue
			}
			rest = append(rest, args[i])
		default:
			rest = append(rest, args[i])
		}
	}

	if !selection.any() {
		return resourceSelection{}, nil, fmt.Errorf("at least one resource is required")
	}

	return selection, rest, nil
}

func allResources() resourceSelection {
	return resourceSelection{skills: true, rules: true, hooks: true}
}

func (s resourceSelection) any() bool {
	return s.skills || s.rules || s.hooks
}

func (s resourceSelection) includesManaged() bool {
	return s.rules || s.hooks
}

func (s resourceSelection) onlyManaged() bool {
	return !s.skills && s.includesManaged()
}

func (s resourceSelection) names() []string {
	names := make([]string, 0, 3)
	if s.skills {
		names = append(names, "skills")
	}
	if s.rules {
		names = append(names, "rules")
	}
	if s.hooks {
		names = append(names, "hooks")
	}
	return names
}

func (s *resourceSelection) addCSV(raw string) error {
	for _, part := range strings.Split(raw, ",") {
		value := strings.ToLower(strings.TrimSpace(part))
		switch value {
		case "":
			continue
		case "skills":
			s.skills = true
		case "rules":
			s.rules = true
		case "hooks":
			s.hooks = true
		default:
			return fmt.Errorf("unsupported resource %q", strings.TrimSpace(part))
		}
	}
	if !s.any() {
		return fmt.Errorf("at least one resource is required")
	}
	return nil
}
