package mcp

import (
	"fmt"
	"strings"
)

// Pi's extensions share a destination, but not transport or credential syntax.
// Sync config only: installing or starting either extension remains explicit.
func renderPi(s Server) (map[string]any, error) {
	if s.PiExtension == "" {
		return nil, fmt.Errorf("Pi requires piExtension: pi-mcp-adapter or pi-mcp-extension; install that package in Pi first")
	}
	format := clientFormats["pi"]
	if s.PiExtension == "pi-mcp-adapter" {
		format.refPrefix = "${"
	}
	// The extension passes through the parent's environment, but cannot rename
	// variables or interpolate headers. Never resolve credentials in Skillshare.
	if s.PiExtension == "pi-mcp-extension" {
		env := map[string]Value{}
		for key, value := range s.Env {
			if value.FromEnv != "" {
				if value.FromEnv != key {
					return nil, fmt.Errorf("pi-mcp-extension cannot rename environment variables; use matching names or pi-mcp-adapter")
				}
			} else {
				env[key] = value
			}
		}
		s.Env = env
	}
	out, err := renderAdditionalClient("pi", format, s)
	if err != nil {
		return nil, err
	}
	if s.PiExtension == "pi-mcp-extension" {
		out["transport"] = "stdio"
		if s.URL != "" {
			out["transport"] = "streamable-http"
		}
	} else {
		for _, key := range []string{"env", "headers"} {
			values, _ := out[key].(map[string]string)
			for name, value := range values {
				// Adapter leading ! invokes a command. Portable literals must stay literal.
				if strings.HasPrefix(value, "!") {
					values[name] = "!" + value
				}
				if strings.Contains(value, "$env:") {
					return nil, fmt.Errorf("Pi: use fromEnv instead of $env: interpolation")
				}
			}
		}
	}
	return out, nil
}
