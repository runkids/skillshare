package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
)

// piOverride reports a Pi project entry without command, url or type. Since Pi 1.0.1 it
// overrides only enabled, exposure and toolExposure of the global server with that name;
// Pi's /mcp writes one for "Enable/Disable in this project".
func piOverride(entry map[string]any) bool {
	return entry["command"] == nil && entry["url"] == nil && entry["type"] == nil
}

// ValidDirectTools accepts what pi-mcp-adapter documents: a switch, "search", or tool names.
// Loading converts it to Pi's exposure settings (see adoptAdapterTools); the config editor
// still checks the value a config may carry until then.
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
	for _, key := range sortedKeys(s.PiOptions) {
		if adapterPiOption(key, isObject(s.PiOptions[key])) {
			return fmt.Errorf("MCP %s: piOptions.%s is a pi-mcp-adapter setting that Pi's built-in MCP does not read", name, key)
		}
	}
	for key, part := range map[string]string{"includeTools": "allow", "excludeTools": "deny"} {
		if _, set := s.PiOptions[key]; set {
			return fmt.Errorf("MCP %s: piOptions.%s is a pi-mcp-adapter setting; use tools.%s", name, key, part)
		}
	}
	if err := validatePiBuiltinOptions(name, s.PiOptions); err != nil {
		return err
	}
	if _, set := s.PiOptions["auth"]; set {
		// Pi sends the provider's login token as the bearer token, so only to an https
		// server, or plain http on this machine.
		u, err := url.Parse(s.URL)
		if s.URL == "" || err != nil || u.Scheme != "https" && !(u.Scheme == "http" && loopbackHost(u.Hostname())) {
			return fmt.Errorf("MCP %s: piOptions.auth needs an https url, or http on localhost; Pi sends the provider's login token to it", name)
		}
	}
	return nil
}

// agentFieldsChanged reports a field outside the ownership hash, such as a piOptions key,
// that the config sets and the file does not have yet. These stay out of the hash: people
// added them to Pi's file by hand before Skillshare could set them, and
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
	piToolPolicy(out, s.Tools)
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
