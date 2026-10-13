package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// A live probe pins each tool's definition the first time a server answers and compares every
// later probe with it, so a tool whose description or input schema changes after the user
// reviewed it (a "rug pull") is named. The pins live in their own file beside state.json:
// the ownership ledger is rewritten wholesale and would drop them.

// toolPinFile is the mcp/tool-pins.json content.
type toolPinFile struct {
	Version int `json:"version"`
	// Servers is, by mcp.projects root ("" for the global list) and then server name, the
	// definition hash of each tool by name.
	Servers map[string]map[string]map[string]string `json:"servers"`
}

func (s *Service) toolPinsPath() string { return filepath.Join(s.StateDir, "mcp", "tool-pins.json") }

// readToolPins reads the pin file. An unreadable one is an error even for --accept-tools: it may
// hold other servers' pins, or a newer version's, and rewriting it would drop them unseen.
func (s *Service) readToolPins() (toolPinFile, error) {
	pins := toolPinFile{Version: 1, Servers: map[string]map[string]map[string]string{}}
	if s.StateDir == "" {
		return pins, fmt.Errorf("MCP state directory is required")
	}
	data, exists, _, err := safeRead(s.toolPinsPath())
	if err != nil || !exists {
		return pins, err
	}
	var read toolPinFile
	if json.Unmarshal(data, &read) != nil || read.Version != 1 || read.Servers == nil {
		return toolPinFile{}, fmt.Errorf("%s is invalid", s.toolPinsPath())
	}
	return read, nil
}

// liveTool is the part of a tools/list entry a pin covers: the text and schema a model reads.
type liveTool struct {
	Name        string          `json:"name"`
	Description json.RawMessage `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// hash is the SHA-256 of the tool's canonical JSON: encoding sorts object keys, so only a change
// in content, not in key order or whitespace, changes it. Numbers keep their literal text, so a
// bound past 2^53 is not rounded into another one.
func (t liveTool) hash() string {
	canonical := func(raw json.RawMessage) any {
		var v any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		_ = d.Decode(&v) // a missing field stays nil
		return v
	}
	data, _ := json.Marshal(map[string]any{"name": t.Name, "description": canonical(t.Description), "inputSchema": canonical(t.InputSchema)})
	return digest(data)
}

// toolBaseline is the pin file read once for a live check: servers are compared with it while
// they are probed, and the pins to record are written once afterwards.
type toolBaseline struct {
	pins   toolPinFile
	err    error
	accept bool
	// configRoot is the project a project-mode config belongs to, "" otherwise: its own servers
	// carry no root of their own, and the state directory is shared with the global scope.
	configRoot string
	// updates holds, by report.Servers index, the pins to record for a probed server.
	updates []map[string]string
}

func (s *Service) loadToolBaseline(servers int, accept bool) *toolBaseline {
	pins, err := s.readToolPins()
	return &toolBaseline{pins: pins, err: err, accept: accept, configRoot: s.configProjectRoot(), updates: make([]map[string]string, servers)}
}

// scope is the pin file key for a server whose report entry names project as its root.
func (b *toolBaseline) scope(project string) string {
	if project == "" {
		return b.configRoot
	}
	return project
}

// compare reports how a server's tools differ from its pins, and queues the pins to record.
// Each call touches only index i of updates, so probes may run it concurrently.
func (b *toolBaseline) compare(i int, root, name string, now map[string]string) []CheckFinding {
	if b.err != nil {
		return []CheckFinding{{Level: "warning", Check: "tool-change", Message: fmt.Sprintf("tool definitions were not compared: %v; move that file away to pin them again", b.err)}}
	}
	pinned, ok := b.pins.Servers[b.scope(root)][name]
	if !ok {
		b.updates[i] = now
		return []CheckFinding{{Level: "info", Check: "tool-change", Message: fmt.Sprintf("pinned %d tool definition(s) as the baseline", len(now))}}
	}
	var changed, added, removed []string
	for _, tool := range sortedKeys(now) {
		switch old, known := pinned[tool]; {
		case !known:
			added = append(added, tool)
		case old != now[tool]:
			changed = append(changed, tool)
		}
	}
	for _, tool := range sortedKeys(pinned) {
		if _, still := now[tool]; !still {
			removed = append(removed, tool)
		}
	}
	if len(changed)+len(added)+len(removed) == 0 {
		return nil
	}
	if b.accept {
		b.updates[i] = now
		var parts []string
		for label, names := range map[string][]string{"changed": changed, "added": added, "removed": removed} {
			if len(names) > 0 {
				parts = append(parts, label+": "+strings.Join(names, ", "))
			}
		}
		slices.Sort(parts)
		return []CheckFinding{{Level: "info", Check: "tool-change", Message: "accepted the current tool definitions as the baseline (" + strings.Join(parts, "; ") + ")"}}
	}
	// Name the scope, so the command accepts this baseline even when run from another directory.
	scope := "-g"
	if b.configRoot != "" {
		scope = "-p"
	}
	review := fmt.Sprintf("; review it, then run skillshare mcp check %s --live --accept-tools %s to accept", name, scope)
	var out []CheckFinding
	if len(changed) > 0 {
		out = append(out, CheckFinding{Level: "warning", Check: "tool-change", Message: "description or input schema changed since it was pinned: " + strings.Join(changed, ", ") + review})
	}
	if len(added) > 0 {
		out = append(out, CheckFinding{Level: "warning", Check: "tool-change", Message: "new since it was pinned: " + strings.Join(added, ", ") + review})
	}
	if len(removed) > 0 {
		out = append(out, CheckFinding{Level: "info", Check: "tool-change", Message: "no longer listed: " + strings.Join(removed, ", ")})
	}
	return out
}

// save records the queued pins. A failure is a finding on the servers it affects: the check
// itself still has its answer.
func (b *toolBaseline) save(s *Service, report *CheckReport) {
	recorded := false
	for _, tools := range b.updates {
		recorded = recorded || tools != nil
	}
	if !recorded {
		return
	}
	err := func() error {
		lock, err := s.lock()
		if err != nil {
			return err
		}
		defer lock.Unlock()
		// Read again under the lock, so a concurrent check's pins are kept.
		pins, err := s.readToolPins()
		if err != nil {
			return err
		}
		for i, tools := range b.updates {
			if tools == nil {
				continue
			}
			result := report.Servers[i]
			scope := b.scope(result.Project)
			if pins.Servers[scope] == nil {
				pins.Servers[scope] = map[string]map[string]string{}
			}
			// Without --accept-tools only an unpinned server queues a pin, so one that another
			// check pinned in the meantime keeps its baseline.
			if _, pinned := pins.Servers[scope][result.Name]; pinned && !b.accept {
				continue
			}
			pins.Servers[scope][result.Name] = tools
		}
		return writeJSONFile(s.toolPinsPath(), pins)
	}()
	if err == nil {
		return
	}
	for i, tools := range b.updates {
		if tools != nil {
			report.Servers[i].Findings = append(report.Servers[i].Findings, CheckFinding{Level: "warning", Check: "tool-change", Message: fmt.Sprintf("could not save the tool baseline: %v", err)})
		}
	}
}
