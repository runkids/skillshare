package mcp

// Normalize native dialects before applying the common import validation.
func normalizeClientImport(target string, entry map[string]any, c *Candidate) {
	format, ok := clientFormats[target]
	if !ok {
		return
	}
	if target == "gemini" && entry["url"] != nil {
		c.Problems = append(c.Problems, "Gemini url uses SSE; only httpUrl (Streamable HTTP) is supported")
	}
	if format.urlKey != "url" {
		if value, exists := entry[format.urlKey]; exists {
			if entry["url"] != nil {
				c.Problems = append(c.Problems, "Multiple native URL fields are not supported")
			}
			entry["url"] = value
			delete(entry, format.urlKey)
		}
	}
	if target == "goose" {
		if value, ok := entry["cmd"]; ok {
			entry["command"] = value
			delete(entry, "cmd")
		}
		if value, ok := entry["envs"]; ok {
			entry["env"] = value
			delete(entry, "envs")
		}
		if keys, ok := entry["env_keys"]; ok {
			if list, ok := keys.([]any); !ok || len(list) > 0 {
				c.Problems = append(c.Problems, "Goose keychain env_keys cannot be imported as portable environment references")
			}
			delete(entry, "env_keys")
		}
		delete(entry, "name")
	}
	if kind, ok := entry["type"].(string); ok {
		if format.localType != "" && kind == format.localType {
			entry["type"] = "stdio"
		}
		if format.remoteType != "" && kind == format.remoteType {
			entry["type"] = "http"
		}
	}
	if format.stdioOnly && entry["url"] != nil {
		c.Problems = append(c.Problems, "Claude Desktop config files support only stdio servers")
	}
}
