package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ValidDirectTools accepts what pi-mcp-adapter documents: a switch, "search", or tool names.
func ValidDirectTools(value any) bool {
	valid := true
	switch v := value.(type) {
	case nil, bool:
	case string:
		valid = v == "search"
	case []string:
		valid = len(v) > 0 && !slices.Contains(v, "")
	case []any:
		valid = len(v) > 0
		for _, item := range v {
			if tool, ok := item.(string); !ok || tool == "" {
				valid = false
			}
		}
	default:
		valid = false
	}
	return valid
}

func (s Server) validateDirectTools(name string) error {
	if s.DirectTools == nil {
		return nil
	}
	switch {
	case !ValidDirectTools(s.DirectTools):
		return fmt.Errorf("MCP %s: directTools must be true, false, \"search\" or a list of tool names", name)
	case s.Disabled:
		return fmt.Errorf("MCP %s: directTools cannot be set on a disabled entry; it only switches the server off", name)
	}
	return nil
}

func (s Server) validatePiOptions(name string) error {
	if len(s.PiOptions) == 0 {
		return nil
	}
	if s.Disabled {
		return fmt.Errorf("MCP %s: piOptions cannot be set on a disabled entry; it only switches the server off", name)
	}
	if field := piOptionCommand("", s.PiOptions); field != "" {
		return fmt.Errorf("MCP %s: Pi option %s cannot contain a command beginning with !; keep it in Pi or use an environment reference", name, field)
	}
	for _, key := range append(additionalManagedFields("pi"), "directTools", "type", "settings", "autoEnableCodemode") {
		if _, set := s.PiOptions[key]; set && key != "enabled" {
			return fmt.Errorf("MCP %s: piOptions cannot set %s; Skillshare writes that field from the server's own settings", name, key)
		}
	}
	for _, key := range adapterPiOptions {
		if _, set := s.PiOptions[key]; set {
			return fmt.Errorf("MCP %s: piOptions.%s is a pi-mcp-adapter setting that Pi's built-in MCP does not read", name, key)
		}
	}
	return validatePiBuiltinOptions(name, s.PiOptions)
}

// agentFieldsChanged reports a field outside the ownership hash, Pi's directTools or a
// piOptions key, that the config sets and the file does not have yet. These stay out of
// the hash: people added them to Pi's file by hand before Skillshare could set them, and
// hashing them would turn their next sync into a conflict.
func agentFieldsChanged(target string, current, want map[string]any) bool {
	for key, value := range want {
		if slices.Contains(additionalManagedFields(target), key) && !(target == "pi" && key == "enabled") {
			continue
		}
		a, _ := json.Marshal(value)
		b, _ := json.Marshal(current[key])
		if !bytes.Equal(a, b) {
			return true
		}
	}
	return false
}

// renderPi writes an entry of Pi's built-in MCP (Pi >= 0.99.0). Sync writes configuration
// only; it never installs or starts anything.
func renderPi(s Server) (map[string]any, error) {
	out, err := renderAdditionalClient("pi", clientFormats["pi"], s)
	if err != nil {
		return nil, err
	}
	maps.Copy(out, s.PiOptions)
	for _, key := range []string{"env", "headers"} {
		values, _ := out[key].(map[string]string)
		for _, value := range values {
			// Portable literals must not become executable secret commands.
			if strings.HasPrefix(value, "!") {
				return nil, fmt.Errorf("Pi built-in: a literal beginning with ! would run a command; use fromEnv instead")
			}
			if strings.Contains(value, "$env:") {
				return nil, fmt.Errorf("Pi: use fromEnv instead of $env: interpolation")
			}
		}
	}
	return out, nil
}
