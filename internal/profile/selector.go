package profile

import (
	"errors"
	"fmt"
)

var ErrProfileUnavailable = errors.New("active profile unavailable")

type UnavailableError struct {
	Name, ConfigPath, Reason string
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("active profile %q is unavailable in %s: %s; run 'skillshare profile use <name>' or 'skillshare profile use --none'", e.Name, e.ConfigPath, e.Reason)
}

func (e *UnavailableError) Unwrap() error { return ErrProfileUnavailable }

// Selector's zero value selects all skills.
type Selector struct {
	name       string
	definition Definition
	err        error
}

func Resolve(c Catalog, state State, configPath string, stateErr error) Selector {
	if stateErr != nil {
		return Selector{err: &UnavailableError{Name: state.Active, ConfigPath: configPath, Reason: stateErr.Error()}}
	}
	if state.Active == "" {
		return Selector{}
	}
	d, ok := c.Definitions[state.Active]
	if !ok {
		return Selector{name: state.Active, err: &UnavailableError{Name: state.Active, ConfigPath: configPath, Reason: "not defined"}}
	}
	if !d.Usable {
		return Selector{name: state.Active, err: &UnavailableError{Name: state.Active, ConfigPath: configPath, Reason: d.Reason}}
	}
	return Selector{name: state.Active, definition: d}
}

func (s Selector) Active() bool { return s.name != "" || s.err != nil }
func (s Selector) Selects(flatName string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.name == "" {
		return true, nil
	}
	return s.definition.Selects(flatName), nil
}
