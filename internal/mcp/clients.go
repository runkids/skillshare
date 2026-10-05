package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Additional clients share the portable model but differ in file scope and
// native field names. Empty refPrefix means interpolation is not verified.
type clientFormat struct {
	key, localType, remoteType, urlKey, refPrefix string
	globalPath, projectPath                       string
	stdioOnly                                     bool
}

// piAdapterPath is pi-mcp-adapter's file next to Pi's built-in mcp.json. Since 0.23.0
// Skillshare only reads it, to import from it, and removes the entries it wrote there.
func piAdapterPath(path string) string {
	return filepath.Join(filepath.Dir(path), "mcp-adapter.json")
}

var clientFormats = map[string]clientFormat{
	"pi":             {key: "mcpServers", urlKey: "url", refPrefix: "${", globalPath: ".pi/agent/mcp.json", projectPath: ".pi/mcp.json"},
	"omp":            {key: "mcpServers", localType: "stdio", remoteType: "http", urlKey: "url", refPrefix: "${", globalPath: ".omp/agent/mcp.json", projectPath: ".omp/mcp.json"},
	"amp":            {key: "amp.mcpServers", urlKey: "url", refPrefix: "${", globalPath: ".config/amp/settings.json", projectPath: ".amp/settings.json"},
	"claude-desktop": {key: "mcpServers", urlKey: "url", stdioOnly: true},
	"cline":          {key: "mcpServers", localType: "stdio", remoteType: "streamableHttp", urlKey: "url", refPrefix: "${env:"},
	"copilot":        {key: "mcpServers", localType: "local", remoteType: "http", urlKey: "url", refPrefix: "${", globalPath: ".copilot/mcp-config.json", projectPath: ".github/mcp.json"},
	"factory":        {key: "mcpServers", localType: "stdio", remoteType: "http", urlKey: "url", refPrefix: "${", globalPath: ".factory/mcp.json", projectPath: ".factory/mcp.json"},
	"gemini":         {key: "mcpServers", urlKey: "httpUrl", refPrefix: "${", globalPath: ".gemini/settings.json", projectPath: ".gemini/settings.json"},
	"goose":          {key: "extensions", localType: "stdio", remoteType: "streamable_http", urlKey: "uri", globalPath: ".config/goose/config.yaml"},
	"junie":          {key: "mcpServers", urlKey: "url", globalPath: ".junie/mcp/mcp.json", projectPath: ".junie/mcp/mcp.json"},
	"kiro":           {key: "mcpServers", urlKey: "url", refPrefix: "${", globalPath: ".kiro/settings/mcp.json", projectPath: ".kiro/settings/mcp.json"},
	"lmstudio":       {key: "mcpServers", urlKey: "url", globalPath: ".lmstudio/mcp.json"},
	"warp":           {key: "mcpServers", urlKey: "url", globalPath: ".warp/.mcp.json", projectPath: ".warp/.mcp.json"},
	"windsurf":       {key: "mcpServers", urlKey: "serverUrl", refPrefix: "${env:", globalPath: ".codeium/windsurf/mcp_config.json"},
}

func (s *Service) additionalClientPath(target string, format clientFormat) (string, error) {
	if s.ProjectRoot != "" {
		if format.projectPath == "" {
			return "", fmt.Errorf("%s supports only global MCP configuration; use global mode", target)
		}
		if target == "copilot" {
			// Copilot gives .mcp.json precedence over .github/mcp.json. It may
			// belong to Claude, so do not silently overwrite or shadow it.
			if _, err := os.Lstat(filepath.Join(s.ProjectRoot, ".mcp.json")); err == nil {
				return "", fmt.Errorf("Copilot: .mcp.json overrides .github/mcp.json; consolidate the project configuration before syncing")
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
		return filepath.Join(s.ProjectRoot, filepath.FromSlash(format.projectPath)), nil
	}
	home := s.Home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	platform := s.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	xdg := s.ConfigDirs["xdg"]
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	appdata := s.ConfigDirs["appdata"]
	if appdata == "" {
		appdata = filepath.Join(home, "AppData", "Roaming")
	}
	switch target {
	case "claude-desktop":
		switch platform {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
		case "windows":
			return filepath.Join(appdata, "Claude", "claude_desktop_config.json"), nil
		default:
			return "", fmt.Errorf("Claude Desktop MCP configuration is supported on macOS and Windows")
		}
	case "cline":
		// Same order as Cline: CLINE_MCP_SETTINGS_PATH, CLINE_DATA_DIR, then CLINE_DIR or ~/.cline.
		const file = "cline_mcp_settings.json"
		for _, key := range []string{"cline-mcp", "cline-data", "cline"} {
			if dir := s.ConfigDirs[key]; dir != "" && !filepath.IsAbs(dir) {
				return "", fmt.Errorf("Cline config paths must be absolute")
			}
		}
		if path := s.ConfigDirs["cline-mcp"]; path != "" {
			return path, nil
		}
		if dir := s.ConfigDirs["cline-data"]; dir != "" {
			return filepath.Join(dir, "settings", file), nil
		}
		if dir := s.ConfigDirs["cline"]; dir != "" {
			return filepath.Join(dir, "data", "settings", file), nil
		}
		data := filepath.Join(home, ".cline", "data")
		// The extension moves its old VS Code storage to ~/.cline once and then stops reading
		// it. Until that has happened, the old file is the one Cline reads.
		if _, err := os.Lstat(data); err != nil {
			if vscode, err := s.nativePath("vscode"); err == nil {
				legacy := filepath.Join(filepath.Dir(vscode), "globalStorage", "saoudrizwan.claude-dev", "settings", file)
				if _, err := os.Lstat(legacy); err == nil {
					return legacy, nil
				}
			}
		}
		return filepath.Join(data, "settings", file), nil
	case "pi", "omp":
		if dir, _ := s.configDir(target); dir != "" {
			if !filepath.IsAbs(dir) {
				return "", fmt.Errorf("PI_CODING_AGENT_DIR must be absolute")
			}
			return filepath.Join(dir, "mcp.json"), nil
		}
	case "copilot":
		if dir := s.ConfigDirs["copilot"]; dir != "" {
			if !filepath.IsAbs(dir) {
				return "", fmt.Errorf("copilot config directory must be absolute")
			}
			return filepath.Join(dir, "mcp-config.json"), nil
		}
	case "amp":
		return filepath.Join(xdg, "amp", "settings.json"), nil
	case "goose":
		if platform == "windows" {
			return filepath.Join(appdata, "Block", "goose", "config", "config.yaml"), nil
		}
		return filepath.Join(xdg, "goose", "config.yaml"), nil
	}
	return filepath.Join(home, filepath.FromSlash(format.globalPath)), nil
}

func renderAdditionalClient(target string, format clientFormat, s Server) (map[string]any, error) {
	if format.stdioOnly && s.URL != "" {
		return nil, fmt.Errorf("%s supports stdio in its config file; configure remote connectors in the client", target)
	}
	if format.refPrefix == "" {
		if s.BearerToken != nil {
			return nil, fmt.Errorf("%s: fromEnv is not supported because native environment interpolation is not documented", target)
		}
		for _, values := range []map[string]Value{s.Env, s.Headers} {
			for _, v := range values {
				if v.FromEnv != "" {
					return nil, fmt.Errorf("%s: fromEnv is not supported because native environment interpolation is not documented", target)
				}
			}
		}
	}
	resolve := func(v Value) string {
		if v.FromEnv != "" {
			return format.refPrefix + v.FromEnv + "}"
		}
		return v.Literal
	}
	values := func(src map[string]Value) map[string]string {
		out := map[string]string{}
		for k, v := range src {
			out[k] = resolve(v)
		}
		return out
	}
	out := map[string]any{}
	if s.Command != "" {
		out["command"] = s.Command
		if format.localType != "" {
			out["type"] = format.localType
		}
		if len(s.Args) > 0 {
			out["args"] = s.Args
		} else if target == "copilot" || target == "warp" || target == "goose" {
			out["args"] = []string{}
		}
		if len(s.Env) > 0 {
			out["env"] = values(s.Env)
		}
	} else {
		out[format.urlKey] = s.URL
		if format.remoteType != "" {
			out["type"] = format.remoteType
		}
		headers := values(s.Headers)
		if s.BearerToken != nil {
			headers["Authorization"] = "Bearer " + resolve(*s.BearerToken)
		}
		if len(headers) > 0 {
			out["headers"] = headers
		}
	}
	if target == "omp" {
		for _, key := range []string{"env", "headers"} {
			values, _ := out[key].(map[string]string)
			for _, value := range values {
				if strings.HasPrefix(value, "!") {
					return nil, fmt.Errorf("OMP: a literal beginning with ! would run a command; use fromEnv instead")
				}
			}
		}
	}
	if target == "copilot" {
		// Copilot lists the tools it enables, by exact name, or "*" for all of them.
		out["tools"] = []string{"*"}
		if allow, _, _ := namedTools(s.Tools, false); allow != nil {
			out["tools"] = allow
		}
	}
	if target == "goose" {
		out["enabled"] = true
		out["name"] = "server" // The plan supplies the actual portable server name.
		if command, ok := out["command"]; ok {
			out["cmd"] = command
			delete(out, "command")
		}
		if env, ok := out["env"]; ok {
			out["envs"] = env
			delete(out, "env")
		}
	}
	return out, nil
}

func additionalManagedFields(target string) []string {
	if target == "pi" {
		return []string{"type", "command", "args", "env", "url", "headers", "transport", "disabled", "enabled"}
	}
	format, ok := clientFormats[target]
	if !ok {
		return managedFields[target]
	}
	if target == "goose" {
		return []string{"type", "name", "cmd", "args", "envs", "env_keys", "uri", "headers", "enabled", "disabled"}
	}
	return []string{"type", "command", "args", "env", "url", format.urlKey, "headers", "enabled", "disabled"}
}
