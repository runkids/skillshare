package mcp

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"unicode/utf8"
)

// Canonical filesystem paths identify the configuration and its selected source;
// source bytes may contain credentials, so no source-content digest is exposed.
func toolConfigIdentity(source *Source, secrets []string) (string, error) {
	if source == nil {
		return "", errors.New("MCP source identity is unavailable")
	}
	resolve := func(path string) (string, error) {
		if path == "" {
			return "", errors.New("MCP source identity is unavailable")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(absolute)
	}
	config, err := resolve(source.ConfigPath)
	if err != nil {
		return "", errors.New("MCP source identity is unavailable")
	}
	path, err := resolve(source.Path)
	if err != nil {
		return "", errors.New("MCP source identity is unavailable")
	}
	// Resolve first and refuse recognized values before hashing. Sanitized display
	// paths are never identity inputs: replacing text could collapse distinct sources.
	// Reject invalid UTF-8 too: json.Marshal would replace distinct path bytes.
	// An actual, valid U+FFFD rune remains a supported path character.
	if !utf8.ValidString(config) || !utf8.ValidString(path) || contractHasSecret([]any{config, path}, secrets) {
		return "", errors.New("MCP source identity is unavailable")
	}
	data, _ := json.Marshal(struct{ Config, Source string }{config, path})
	return digest(data), nil
}

func toolRecordKey(config, root, name string) string {
	data, _ := json.Marshal(struct{ Config, Root, Name string }{config, root, name})
	return digest(data)
}

func toolReferencesOnly(server Server) bool {
	for _, v := range server.Env {
		if v.FromEnv == "" || v.Literal != "" {
			return false
		}
	}
	for _, v := range server.Headers {
		if v.FromEnv == "" || v.Literal != "" {
			return false
		}
	}
	return server.BearerToken == nil || server.BearerToken.FromEnv != "" && server.BearerToken.Literal == ""
}

// Hash declarations, never resolved credentials. Actual command/cwd and the
// endpoint's self-reported identity complement the complete Server declaration.
// This does not attest executable bytes, ambient env or referenced runtime values.
// The live probe rejects known credential echoes before making this observation.
func toolConnectionIdentity(server Server, now contractObservation) (string, error) {
	if !toolReferencesOnly(server) || now.Live == nil {
		return "", errors.New("MCP connection identity is unavailable")
	}
	command, dir := "", ""
	if server.Command != "" {
		if !filepath.IsAbs(now.Command) || !filepath.IsAbs(now.Dir) || !utf8.ValidString(now.Command) || !utf8.ValidString(now.Dir) {
			return "", errors.New("MCP launch identity is unavailable")
		}
		command, dir = now.Command, now.Dir
	}
	// Remote connections do not use the caller's cwd or launch an executable.
	// Including those unused observations would spuriously stale HTTP approvals.
	data, err := json.Marshal(struct {
		Policy   string         `json:"policy"`
		Server   Server         `json:"server"`
		Command  string         `json:"command,omitempty"`
		Dir      string         `json:"dir,omitempty"`
		Protocol string         `json:"protocol"`
		Info     LiveServerInfo `json:"serverInfo"`
	}{contractPolicy, server, command, dir, now.Live.ProtocolVersion, now.Live.ServerInfo})
	if err != nil {
		return "", errors.New("MCP connection identity is unavailable")
	}
	// Actual PATH resolution and cwd can contain values absent from declarations.
	// Refuse them before an externally visible credential-derived fingerprint.
	var material any
	if decodeLiveJSON(data, &material) != nil || containsLiveSecret(material, now.Secrets) {
		return "", errors.New("MCP connection identity is unavailable")
	}
	return digest(data), nil
}
