package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"skillshare/internal/audit"
)

// Independently authored native profile inspired by RugSnare's split-contract
// model (db0bb6a7021c17f0dd9b26457f65b686a87d24db). This is not RFC8785:
// number lexemes and ALL array order are retained, including schema sets.
const contractPolicy = "skillshare-contract-v2"
const contractToolBytes = 256 * 1024
const contractJSONDepth = 64

// ToolContract retains the complete normalized descriptor for explicit review.
type ToolContract struct {
	Hash         string          `json:"hash"`
	InputHash    string          `json:"inputHash"`
	ProseHash    string          `json:"proseHash"`
	OutputHash   string          `json:"outputHash"`
	MetadataHash string          `json:"metadataHash"`
	Definition   json.RawMessage `json:"definition"`
}
type ContractChange struct {
	Tool    string `json:"tool"`
	Area    string `json:"area"`
	Path    string `json:"path"`
	Grade   string `json:"grade"`
	Message string `json:"message"`
}
type ContractAudit struct {
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
type ContractReview struct {
	Status          string                  `json:"status"`
	Revision        string                  `json:"revision,omitempty"`
	Policy          string                  `json:"policy,omitempty"`
	IdentityChanged bool                    `json:"identityChanged,omitempty"`
	Tools           map[string]ToolContract `json:"tools,omitempty"`
	Changes         []ContractChange        `json:"changes,omitempty"`
	Audit           []ContractAudit         `json:"audit,omitempty"`
	Omitted         int                     `json:"omitted,omitempty"`
}

var errContractJSON = errors.New("invalid MCP JSON")

// decodeLiveJSON validates before decoding into structs: encoding/json alone
// replaces invalid scalars and silently accepts duplicate decoded keys.
// DecodeContractJSON validates a complete JSON value with the native contract
// Unicode, duplicate-key and depth rules before decoding. API request handlers
// can then apply DisallowUnknownFields to enforce their narrower request shape.
func DecodeContractJSON(raw []byte, out any) error { return decodeLiveJSON(raw, out) }

func decodeLiveJSON(raw []byte, out any) error {
	return decodeJSONBounded(raw, out, contractJSONDepth)
}

// decodeJSONBounded permits envelope overhead for stored descriptors. Callers
// must still recapture each descriptor to enforce the profile depth of 64.
func decodeJSONBounded(raw []byte, out any, maxDepth int) error {
	if _, err := contractJSONBounded(raw, maxDepth); err != nil {
		return errContractJSON
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(out) != nil {
		return errContractJSON
	}
	return nil
}

func contractJSON(raw []byte) (any, error) { return contractJSONBounded(raw, contractJSONDepth) }

func contractJSONBounded(raw []byte, maxDepth int) (any, error) {
	if maxDepth < 1 || maxDepth > 128 {
		return nil, errContractJSON
	}
	if !utf8.Valid(raw) || !contractUnicode(raw) {
		return nil, errContractJSON
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := contractValue(d, 0, maxDepth)
	if err != nil {
		return nil, errContractJSON
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errContractJSON
	}
	return v, nil
}

// Validate escaped UTF-16 too; utf8.Valid cannot see unpaired JSON surrogates.
func contractUnicode(raw []byte) bool {
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			lo, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || lo < 0xdc00 || lo > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return !inString
}

func contractValue(d *json.Decoder, depth, maxDepth int) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	if depth >= maxDepth {
		return nil, errContractJSON
	}
	switch delim {
	case '{':
		obj := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			s, ok := key.(string)
			if !ok {
				return nil, errContractJSON
			}
			if _, exists := obj[s]; exists {
				return nil, errContractJSON
			}
			v, err := contractValue(d, depth+1, maxDepth)
			if err != nil {
				return nil, err
			}
			obj[s] = v
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errContractJSON
		}
		return obj, nil
	case '[':
		arr := []any{}
		for d.More() {
			v, err := contractValue(d, depth+1, maxDepth)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errContractJSON
		}
		return arr, nil
	}
	return nil, errContractJSON
}

func captureTool(raw json.RawMessage, secrets []string) (string, ToolContract, error) {
	fail := func() (string, ToolContract, error) {
		return "", ToolContract{}, errors.New("invalid MCP tool contract")
	}
	if len(raw) > contractToolBytes {
		return fail()
	}
	v, err := contractJSON(raw)
	if err != nil {
		return fail()
	}
	obj, ok := v.(map[string]any)
	if !ok || contractHasSecret(v, secrets) {
		return fail()
	}
	name, ok := obj["name"].(string)
	if !ok || strings.TrimSpace(name) == "" || len(name) > 128 {
		return fail()
	}
	for _, r := range name {
		if r < 0x21 || r == 0x7f {
			return fail()
		}
	}
	input, ok := obj["inputSchema"].(map[string]any)
	if !ok || !validContractSchema(input) {
		return fail()
	}
	if typ, exists := input["type"]; exists {
		if s, ok := typ.(string); !ok || s != "object" {
			return fail()
		}
	}
	if _, exists := obj["description"]; !exists {
		obj["description"] = ""
	}
	for _, key := range []string{"description", "title"} {
		if v, exists := obj[key]; exists {
			if _, ok := v.(string); !ok {
				return fail()
			}
		}
	}
	if out, exists := obj["outputSchema"]; exists {
		schema, ok := out.(map[string]any)
		if !ok || !validContractSchema(schema) {
			return fail()
		}
		if typ, exists := schema["type"]; exists {
			if s, ok := typ.(string); !ok || s != "object" {
				return fail()
			}
		}
	}
	annotations := map[string]any{}
	if v, exists := obj["annotations"]; exists {
		var ok bool
		annotations, ok = v.(map[string]any)
		if !ok {
			return fail()
		}
	}
	for key, def := range map[string]bool{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": true} {
		if v, exists := annotations[key]; exists {
			if _, ok := v.(bool); !ok {
				return fail()
			}
		} else {
			annotations[key] = def
		}
	}
	if v, exists := annotations["title"]; exists {
		if _, ok := v.(string); !ok {
			return fail()
		}
	}
	obj["annotations"] = annotations
	if v, exists := obj["_meta"]; exists {
		if _, ok := v.(map[string]any); !ok {
			return fail()
		}
	}
	if v, exists := obj["icons"]; exists {
		icons, ok := v.([]any)
		if !ok {
			return fail()
		}
		for _, v := range icons {
			icon, ok := v.(map[string]any)
			if !ok {
				return fail()
			}
			if _, ok := icon["src"].(string); !ok {
				return fail()
			}
			for _, key := range []string{"mimeType", "theme"} {
				if v, exists := icon[key]; exists {
					if _, ok := v.(string); !ok {
						return fail()
					}
				}
			}
			if sizes, exists := icon["sizes"]; exists {
				if !contractStringArray(sizes) {
					return fail()
				}
			}
		}
	}
	definition, err := json.Marshal(obj)
	if err != nil || len(definition) > contractToolBytes {
		return fail()
	}
	prose, metadata := map[string]any{}, map[string]any{}
	for key, v := range obj {
		switch key {
		case "description", "title":
			prose[key] = v
		case "inputSchema", "outputSchema":
		default:
			metadata[key] = v
		}
	}
	return name, ToolContract{Hash: digest(definition), InputHash: contractHash(input), ProseHash: contractHash(prose), OutputHash: contractHash(obj["outputSchema"]), MetadataHash: contractHash(metadata), Definition: definition}, nil
}

func contractHash(v any) string { b, _ := json.Marshal(v); return digest(b) }
func contractHasSecret(v any, secrets []string) bool {
	has := func(s string) bool {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(s, secret) {
				return true
			}
		}
		return false
	}
	switch v := v.(type) {
	case string:
		return has(v)
	case json.Number:
		return has(v.String())
	case map[string]any:
		for k, v := range v {
			if has(k) || contractHasSecret(v, secrets) {
				return true
			}
		}
	case []any:
		for _, v := range v {
			if contractHasSecret(v, secrets) {
				return true
			}
		}
	}
	return false
}
func contractStringArray(v any) bool {
	a, ok := v.([]any)
	if !ok {
		return false
	}
	for _, v := range a {
		if _, ok := v.(string); !ok {
			return false
		}
	}
	return true
}

// Only genuine schema locations are traversed. Defaults, enum members and
// extension objects are literal JSON and must not acquire schema semantics.
func validContractSchema(v any) bool {
	if _, ok := v.(bool); ok {
		return true
	}
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for k, v := range m {
		switch k {
		case "type":
			valid := func(v any) bool {
				s, ok := v.(string)
				return ok && (s == "object" || s == "array" || s == "string" || s == "number" || s == "integer" || s == "boolean" || s == "null")
			}
			if a, ok := v.([]any); ok {
				if len(a) == 0 {
					return false
				}
				for _, x := range a {
					if !valid(x) {
						return false
					}
				}
			} else if !valid(v) {
				return false
			}
		case "required":
			if !contractStringArray(v) {
				return false
			}
		case "title", "description", "$comment", "$ref", "$schema", "$id", "$anchor", "pattern", "format", "contentEncoding", "contentMediaType":
			if _, ok := v.(string); !ok {
				return false
			}
		case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf":
			if _, ok := v.(json.Number); !ok {
				return false
			}
		case "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties", "minContains", "maxContains":
			n, ok := v.(json.Number)
			if !ok || !contractNonnegativeInteger(n) {
				return false
			}
		case "uniqueItems", "readOnly", "writeOnly", "deprecated":
			if _, ok := v.(bool); !ok {
				return false
			}
		case "enum", "examples":
			if _, ok := v.([]any); !ok {
				return false
			}
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
			children, ok := v.(map[string]any)
			if !ok {
				return false
			}
			for _, child := range children {
				if !validContractSchema(child) {
					return false
				}
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			children, ok := v.([]any)
			if !ok {
				return false
			}
			for _, child := range children {
				if !validContractSchema(child) {
					return false
				}
			}
		case "items", "additionalProperties", "unevaluatedProperties", "additionalItems", "unevaluatedItems", "contains", "not", "if", "then", "else", "propertyNames":
			if !validContractSchema(v) {
				return false
			}
		case "dependentRequired":
			children, ok := v.(map[string]any)
			if !ok {
				return false
			}
			for _, child := range children {
				if !contractStringArray(child) {
					return false
				}
			}
		}
	}
	return true
}

// Full snapshots retain every byte. Diagnostics bound locations separately.
func contractLocation(path, message string) (string, string) {
	if len(path) <= 4096 {
		return path, message
	}
	end := 4093
	for !utf8.RuneStart(path[end]) {
		end--
	}
	return path[:end] + "…", message + "; location truncated, inspect full contract"
}

func contractPointer(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}
func contractKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// auditContracts never returns a partial scan as success. The shared scanner
// preserves whole-field context and caps findings during padding and rule scans.
func auditContracts(tools map[string]ToolContract) ([]ContractAudit, error) {
	rules, err := audit.Rules()
	if err != nil {
		return nil, errors.New("MCP contract audit rules unavailable")
	}
	findings := []ContractAudit{}
	fields, textBytes, lines := 0, 0, 0
	exhausted := errors.New("MCP contract audit incomplete: analysis limit reached")
	scan := func(tool, path, s string) error {
		fields++
		textBytes += len(s)
		if fields > 4096 || textBytes > 1024*1024 {
			return exhausted
		}

		lines += audit.VisualLineCount(s)
		if lines > 8192 {
			return exhausted
		}
		// A decoded string is content, never a file with an encoding mark.
		if strings.HasPrefix(s, "\ufeff") {
			s = " " + s
		}
		matches, incomplete := audit.ScanContentWithRulesBounded([]byte(s), "mcp-contract.txt", rules, 200-len(findings))
		for _, f := range matches {
			location, message := contractLocation(path, "contract text matched an audit rule; review required")
			findings = append(findings, ContractAudit{Tool: tool, Path: location, RuleID: f.RuleID, Severity: f.Severity, Message: message})
		}
		if incomplete {
			return exhausted
		}
		return nil
	}
	var walk func(string, string, any) error
	walk = func(tool, path string, v any) error {
		switch v := v.(type) {
		case string:
			return scan(tool, path, v)
		case map[string]any:
			for _, k := range contractKeys(v) {
				p := contractPointer(path, k)
				if err := scan(tool, p+"/@key", k); err != nil {
					return err
				}
				if err := walk(tool, p, v[k]); err != nil {
					return err
				}
			}
		case []any:
			for i, x := range v {
				if err := walk(tool, fmt.Sprintf("%s/%d", path, i), x); err != nil {
					return err
				}
			}
		}
		return nil
	}
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	inventoryBytes := 0
	if len(names) > 4096 {
		return nil, exhausted
	}
	for _, name := range names {
		raw := tools[name].Definition
		inventoryBytes += len(raw)
		if len(raw) > contractToolBytes || inventoryBytes > 4*1024*1024 {
			return findings, exhausted
		}
		v, err := contractJSON(raw)
		if err != nil {
			return findings, errors.New("MCP contract audit incomplete: invalid descriptor")
		}
		if err := walk(name, "", v); err != nil {
			return findings, err
		}
	}
	return findings, nil
}
