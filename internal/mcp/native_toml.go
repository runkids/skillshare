package mcp

import (
	"bytes"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

type tableSpan struct {
	start, end int
	name       string
}

func (n *Native) editTOML(changes map[string]map[string]any) ([]byte, error) {
	var parser unstable.Parser
	parser.Reset(n.data)
	var spans []tableSpan
	var table []string
	for parser.NextExpression() {
		expr := parser.Expression()
		if expr.Kind != unstable.Table && expr.Kind != unstable.ArrayTable && expr.Kind != unstable.KeyValue {
			continue
		}
		var keys []string
		first := -1
		it := expr.Key()
		for it.Next() {
			key := it.Node()
			if first == -1 {
				first = int(key.Raw.Offset)
			}
			keys = append(keys, string(key.Data))
		}
		if expr.Kind == unstable.KeyValue {
			// Inline/dotted definitions have no independent table boundary. Refuse
			// rather than accidentally rewrite an unrelated parent expression.
			if len(keys) > 0 && ((len(table) == 0 && keys[0] == "mcp_servers") || (len(table) == 1 && table[0] == "mcp_servers")) {
				return nil, fmt.Errorf("%s MCP uses inline/dotted entries; convert them to [mcp_servers.NAME] tables before syncing", n.Target)
			}
			continue
		}
		start := bytes.LastIndexByte(n.data[:first], '\n') + 1
		if len(spans) > 0 {
			spans[len(spans)-1].end = start
		}
		span := tableSpan{start: start, end: len(n.data)}
		if len(keys) >= 2 && keys[0] == "mcp_servers" {
			if expr.Kind == unstable.ArrayTable {
				return nil, fmt.Errorf("%s MCP cannot be an array of tables", n.Target)
			}
			span.name = keys[1]
		}
		spans = append(spans, span)
		table = keys
	}
	if parser.Error() != nil {
		return nil, fmt.Errorf("cannot safely locate %s MCP tables", n.Target)
	}
	newline := []byte("\n")
	if bytes.Contains(n.data, []byte("\r\n")) {
		newline = []byte("\r\n")
	}
	encode := func(name string) ([]byte, error) {
		encoded, err := toml.Marshal(map[string]any{"mcp_servers": map[string]any{name: changes[name]}})
		if err != nil {
			return nil, fmt.Errorf("cannot render %s MCP entry", n.Target)
		}
		// The parent table may already exist in the original file.
		encoded = bytes.TrimPrefix(encoded, []byte("[mcp_servers]\n"))
		return bytes.ReplaceAll(encoded, []byte("\n"), newline), nil
	}
	var out bytes.Buffer
	pos := 0
	written := map[string]bool{}
	for _, span := range spans {
		if _, changed := changes[span.name]; span.name == "" || !changed {
			continue
		}
		out.Write(n.data[pos:span.start])
		pos = trimTrailingComments(n.data, span.start, span.end)
		// An update keeps the entry's position; its sub-tables are rendered with it.
		if changes[span.name] != nil && !written[span.name] {
			encoded, err := encode(span.name)
			if err != nil {
				return nil, err
			}
			out.Write(encoded)
			written[span.name] = true
		}
	}
	out.Write(n.data[pos:])
	var added []string
	for _, name := range sortedKeys(changes) {
		if changes[name] != nil && !written[name] {
			added = append(added, name)
		}
	}
	// Removing or appending the last table must not accumulate blank lines.
	if pos == len(n.data) || len(added) > 0 {
		trimmed := bytes.TrimRight(out.Bytes(), "\r\n")
		out.Truncate(len(trimmed))
		if out.Len() > 0 {
			out.Write(newline)
		}
	}
	for _, name := range added {
		encoded, err := encode(name)
		if err != nil {
			return nil, err
		}
		if out.Len() > 0 {
			out.Write(newline)
		}
		out.Write(encoded)
	}
	result := out.Bytes()
	parsed, err := ParseNative(n.Target, result)
	if err != nil {
		return nil, err
	}
	for name, entry := range changes {
		if !sameEntry(parsed.Entries[name], entry) {
			return nil, fmt.Errorf("%s edit failed verification", n.Target)
		}
	}
	return result, nil
}
