package mcp

import (
	"fmt"
	"sort"
)

// Render converts a portable definition to a native entry without reading env.
func Render(target string, s Server) (map[string]any, error) {
	if !validTarget(target) {
		return nil, fmt.Errorf("unsupported MCP target %q", target)
	}
	if err := s.Validate("server"); err != nil {
		return nil, err
	}
	if format, ok := clientFormats[target]; ok {
		return renderAdditionalClient(target, format, s)
	}
	out := map[string]any{}
	if target == "antigravity" {
		if s.BearerToken != nil {
			return nil, fmt.Errorf("Antigravity: bearerToken/fromEnv is not supported; complete OAuth authentication in Antigravity")
		}
		for _, values := range []map[string]Value{s.Env, s.Headers} {
			for _, value := range values {
				if value.FromEnv != "" {
					return nil, fmt.Errorf("Antigravity: fromEnv is not supported because native environment interpolation is not documented")
				}
			}
		}
	}
	resolve := func(v Value) string {
		if v.FromEnv == "" {
			return v.Literal
		}
		if target == "opencode" {
			return "{env:" + v.FromEnv + "}"
		}
		if target == "claude" || target == "grok" {
			return "${" + v.FromEnv + "}"
		}
		return "${env:" + v.FromEnv + "}"
	}
	if s.Command != "" {
		out["command"] = s.Command
		if target != "codex" && target != "grok" && target != "antigravity" {
			out["type"] = "stdio"
		}
		if len(s.Args) > 0 {
			out["args"] = s.Args
		}
		env := map[string]string{}
		var forwarded []string
		for key, v := range s.Env {
			if target == "codex" && v.FromEnv != "" {
				if key != v.FromEnv {
					return nil, fmt.Errorf("Codex requires environment reference %s to use the same variable name", key)
				}
				forwarded = append(forwarded, key)
			} else {
				env[key] = resolve(v)
			}
		}
		if len(env) > 0 {
			out["env"] = env
		}
		if len(forwarded) > 0 {
			sort.Strings(forwarded)
			out["env_vars"] = forwarded
		}
		if target == "opencode" {
			out["type"] = "local"
			out["command"] = append([]string{s.Command}, s.Args...)
			delete(out, "args")
			if len(env) > 0 {
				out["environment"] = env
				delete(out, "env")
			}
		}
	} else {
		out["url"] = s.URL
		if target == "antigravity" {
			out["serverUrl"] = s.URL
			delete(out, "url")
		}
		if target == "opencode" {
			out["type"] = "remote"
		}
		if target == "claude" || target == "vscode" {
			out["type"] = "http"
		}
		headers := map[string]string{}
		envHeaders := map[string]string{}
		for key, v := range s.Headers {
			if target == "codex" && v.FromEnv != "" {
				envHeaders[key] = v.FromEnv
			} else {
				headers[key] = resolve(v)
			}
		}
		if s.BearerToken != nil {
			if target == "codex" {
				out["bearer_token_env_var"] = s.BearerToken.FromEnv
			} else {
				headers["Authorization"] = "Bearer " + resolve(*s.BearerToken)
			}
		}
		if len(headers) > 0 {
			if target == "codex" {
				out["http_headers"] = headers
			} else {
				out["headers"] = headers
			}
		}
		if len(envHeaders) > 0 {
			out["env_http_headers"] = envHeaders
		}
	}
	return out, nil
}
