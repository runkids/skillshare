package mcp

import (
	"bytes"
	"fmt"
	"sort"

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
	var out bytes.Buffer
	pos := 0
	for _, span := range spans {
		if _, changed := changes[span.name]; span.name == "" || !changed {
			continue
		}
		out.Write(n.data[pos:span.start])
		pos = trimTrailingComments(n.data, span.start, span.end)
	}
	out.Write(n.data[pos:])
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if changes[name] == nil {
			continue
		}
		encoded, err := toml.Marshal(map[string]any{"mcp_servers": map[string]any{name: changes[name]}})
		if err != nil {
			return nil, fmt.Errorf("cannot render %s MCP entry", n.Target)
		}
		// The parent table may already exist in the original file.
		encoded = bytes.TrimPrefix(encoded, []byte("[mcp_servers]\n"))
		out.WriteByte('\n')
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
