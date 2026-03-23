package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateTargetJSON creates JSON bytes for a target's MCP config.
// Uses target.Key as top-level key, target.EffectiveURLKey() for remote URL field.
func GenerateTargetJSON(servers map[string]MCPServer, target MCPTargetSpec) ([]byte, error) {
	doc := map[string]any{
		target.Key: transformServers(servers, target),
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// transformServers converts all servers to a target-specific map.
func transformServers(servers map[string]MCPServer, target MCPTargetSpec) map[string]any {
	result := make(map[string]any, len(servers))
	for name, srv := range servers {
		result[name] = transformServer(srv, target)
	}
	return result
}

// transformServer converts MCPServer to target-specific map[string]any.
//   - stdio: {"command": ..., "args": [...], "env": {...}}
//   - remote: {urlKey: ..., "headers": {...}}  (urlKey from target.EffectiveURLKey())
//   - env included for both if non-empty
func transformServer(srv MCPServer, target MCPTargetSpec) map[string]any {
	m := make(map[string]any)
	if srv.IsRemote() {
		m[target.EffectiveURLKey()] = srv.URL
		if len(srv.Headers) > 0 {
			m["headers"] = copyStringMap(srv.Headers)
		}
	} else {
		m["command"] = srv.Command
		if len(srv.Args) > 0 {
			m["args"] = srv.Args
		}
	}
	if len(srv.Env) > 0 {
		m["env"] = copyStringMap(srv.Env)
	}
	return m
}

// copyStringMap returns a shallow copy of a map[string]string as map[string]any.
func copyStringMap(src map[string]string) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GeneratedDir returns the directory for generated MCP JSON files.
func GeneratedDir(configDir string) string {
	return filepath.Join(configDir, "mcp")
}

// GenerateAllTargetFiles generates a JSON file for each target that has at least one
// matching server. Returns map[targetName]filePath. Skips targets with 0 matching servers.
func GenerateAllTargetFiles(cfg *MCPConfig, targets []MCPTargetSpec, outDir string) (map[string]string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	result := make(map[string]string)
	for _, target := range targets {
		servers := cfg.ServersForTarget(target.Name)
		if len(servers) == 0 {
			continue
		}

		data, err := GenerateTargetJSON(servers, target)
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", target.Name, err)
		}
		path := filepath.Join(outDir, target.Name+".json")

		// Skip write if content unchanged
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
			result[target.Name] = path
			continue
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", target.Name, err)
		}
		result[target.Name] = path
	}
	return result, nil
}
