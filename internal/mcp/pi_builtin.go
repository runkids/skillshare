package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var PiExposures = []string{"codemode", "codemode-deferred", "deferred", "direct", "hidden"}

// Portable Pi options must not introduce commands, including through non-secret keys.
func piOptionCommand(path string, value any) string {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "!") {
			return path
		}
	case PiOptions:
		return piOptionCommand(path, map[string]any(v))
	case map[string]any:
		for _, key := range sortedKeys(v) {
			field := key
			if path != "" {
				field = path + "." + key
			}
			if field = piOptionCommand(field, v[key]); field != "" {
				return field
			}
		}
	case []any:
		for i, item := range v {
			if field := piOptionCommand(fmt.Sprintf("%s[%d]", path, i), item); field != "" {
				return field
			}
		}
	}
	return ""
}

func validPiExposure(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	for _, exposure := range PiExposures {
		if text == exposure {
			return true
		}
	}
	return false
}

// Known fields follow Pi 1.0.0; unknown per-server fields stay available to custom builds.
func validatePiBuiltinOptions(name string, options map[string]any) error {
	if value, ok := options["enabled"]; ok {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("MCP %s: invalid Pi built-in enabled", name)
		}
	}
	bad := func(field string) error { return fmt.Errorf("MCP %s: invalid Pi built-in %s", name, field) }
	if value, ok := options["exposure"]; ok && !validPiExposure(value) {
		return bad("exposure")
	}
	if value, ok := options["toolExposure"]; ok {
		data, _ := json.Marshal(value)
		var tools map[string]any
		if json.Unmarshal(data, &tools) != nil || tools == nil {
			return bad("toolExposure")
		}
		for _, exposure := range tools {
			if !validPiExposure(exposure) {
				return bad("toolExposure")
			}
		}
	}
	if value, ok := options["timeout"]; ok {
		data, _ := json.Marshal(value)
		var seconds float64
		if json.Unmarshal(data, &seconds) != nil || seconds <= 0 {
			return bad("timeout (positive seconds)")
		}
	}
	if value, ok := options["cwd"]; ok {
		if _, ok := value.(string); !ok {
			return bad("cwd")
		}
	}
	if value, ok := options["oauth"]; ok {
		data, _ := json.Marshal(value)
		var oauth map[string]any
		if json.Unmarshal(data, &oauth) != nil || oauth == nil {
			return bad("oauth")
		}
		for _, key := range []string{"clientId", "clientSecret", "callbackUrl", "scope", "clientName"} {
			if value, exists := oauth[key]; exists {
				if _, ok := value.(string); !ok {
					return bad("oauth." + key)
				}
			}
		}
		if value, exists := oauth["callbackPort"]; exists {
			var port int
			data, _ := json.Marshal(value)
			if json.Unmarshal(data, &port) != nil || port < 1 || port > 65535 {
				return bad("oauth.callbackPort")
			}
		}
		if text, exists := oauth["callbackUrl"].(string); exists {
			u, err := url.Parse(text)
			if err != nil || u.Scheme != "http" || !loopbackHost(u.Hostname()) || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				return bad("oauth.callbackUrl")
			}
			if port, exists := oauth["callbackPort"]; exists && u.Port() != "" && fmt.Sprint(port) != u.Port() {
				return bad("oauth.callbackPort")
			}
		}
		// Pi 1.0 trusts this document instead of discovery, so it requires https except on loopback.
		if value, exists := oauth["authServerMetadataUrl"]; exists {
			text, _ := value.(string)
			u, err := url.Parse(text)
			if err != nil || u.Host == "" || u.User != nil || u.Scheme != "https" && !(u.Scheme == "http" && loopbackHost(u.Hostname())) {
				return bad("oauth.authServerMetadataUrl (https, or http on localhost)")
			}
		}
	}
	if value, ok := options["auth"]; ok {
		data, _ := json.Marshal(value)
		var auth struct {
			Provider string `json:"provider"`
		}
		if json.Unmarshal(data, &auth) != nil || auth.Provider == "" {
			return bad("auth.provider")
		}
	}
	return nil
}

func loopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Import Pi options recursively without copying literal credentials into the source.
func importPiOption(name, key string, value any, warnings *[]string) any {
	if text, ok := value.(string); ok && (sensitiveKey.MatchString(key) || hasURLPassword(text)) {
		if strings.Contains(text, "${") {
			return text
		}
		ref := "MCP_" + strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(name, "-", "_"), ".", "_")) + "_" + strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(key, "_"))
		*warnings = append(*warnings, "Pi credential field "+key+" replaced with ${"+ref+"}; set that variable before connecting")
		return "${" + ref + "}"
	}
	if object, ok := value.(map[string]any); ok {
		out := map[string]any{}
		for key, item := range object {
			out[key] = importPiOption(name, key, item, warnings)
		}
		return out
	}
	if items, ok := value.([]any); ok {
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = importPiOption(name, key, item, warnings)
		}
		return out
	}
	return value
}

func piFieldHash(value any) string { data, _ := json.Marshal(value); return digest(data) }

func piOwnedFields(entry map[string]any) map[string]string {
	fields := map[string]string{}
	for key, value := range entry {
		managed := false
		for _, field := range additionalManagedFields("pi") {
			managed = managed || key == field && key != "enabled"
		}
		if !managed {
			fields[key] = piFieldHash(value)
		}
	}
	return fields
}

// FieldChanges contains names only: previews must not expose credentials from native files.
type FieldChanges struct {
	Added   []string `json:"added,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

func changedFieldNames(before, after map[string]any, action string) *FieldChanges {
	if action != "add" && action != "update" && action != "remove" {
		return nil
	}
	fields := &FieldChanges{}
	for _, key := range sortedKeys(after) {
		if _, exists := before[key]; !exists {
			fields.Added = append(fields.Added, key)
		} else if piFieldHash(before[key]) != piFieldHash(after[key]) {
			fields.Updated = append(fields.Updated, key)
		}
	}
	for _, key := range sortedKeys(before) {
		if _, exists := after[key]; !exists {
			fields.Removed = append(fields.Removed, key)
		}
	}
	return fields
}

// Keep ordered toolExposure patterns: Pi uses the first matching wildcard, so alphabetizing
// them while converting JSON/YAML would change which tools the model can reach.
type piExposureEntry struct {
	Key   string
	Value any
}
type piToolExposure []piExposureEntry

type PiOptions map[string]any

func orderedExposure(raw []byte) (piToolExposure, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("toolExposure must be an object")
	}
	var out piToolExposure
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value any
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		out = append(out, piExposureEntry{key.(string), value})
	}
	return out, nil
}

func (p *PiOptions) UnmarshalJSON(data []byte) error {
	var options map[string]any
	if err := json.Unmarshal(data, &options); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if value := raw["toolExposure"]; value != nil {
		if ordered, err := orderedExposure(value); err == nil {
			options["toolExposure"] = ordered
		}
	}
	*p = options
	return nil
}

func (p *PiOptions) UnmarshalYAML(n *yaml.Node) error {
	var options map[string]any
	if err := n.Decode(&options); err != nil {
		return err
	}
	if node := field(n, "toolExposure"); node != nil && node.Kind == yaml.MappingNode {
		var ordered piToolExposure
		for i := 0; i < len(node.Content); i += 2 {
			var value any
			if err := node.Content[i+1].Decode(&value); err != nil {
				return err
			}
			ordered = append(ordered, piExposureEntry{node.Content[i].Value, value})
		}
		options["toolExposure"] = ordered
	}
	*p = options
	return nil
}

func (p piToolExposure) MarshalJSON() ([]byte, error) {
	data := []byte("{")
	for i, entry := range p {
		if i > 0 {
			data = append(data, ',')
		}
		key, _ := json.Marshal(entry.Key)
		value, err := json.Marshal(entry.Value)
		if err != nil {
			return nil, err
		}
		data = append(data, key...)
		data = append(data, ':')
		data = append(data, value...)
	}
	return append(data, '}'), nil
}

func (p piToolExposure) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, entry := range p {
		value := &yaml.Node{}
		if err := value.Encode(entry.Value); err != nil {
			return nil, err
		}
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: entry.Key}, value)
	}
	return n, nil
}

// redactPiOption covers native-only values in configuration previews.
func redactPiOption(key string, value any) any {
	if text, ok := value.(string); ok && (sensitiveKey.MatchString(key) || hasURLPassword(text)) {
		return "<kept in Pi>"
	}
	if object, ok := value.(map[string]any); ok {
		out := map[string]any{}
		for key, item := range object {
			out[key] = redactPiOption(key, item)
		}
		return out
	}
	if items, ok := value.([]any); ok {
		out := make([]any, len(items))
		for i, item := range items {
			out[i] = redactPiOption(key, item)
		}
		return out
	}
	return value
}
