package mcp

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"skillshare/internal/tokens"
)

// A live probe also estimates what each server's tool definitions cost: an Agent puts every
// tool's name, description and input schema in the model's context before any tool is called.
// The counts live in their own file, so the pin file keeps the format older versions read.

// ToolCost is what the last live probe of a server counted.
type ToolCost struct {
	Tools   int       `json:"tools"`
	Tokens  int       `json:"tokens"`
	Checked time.Time `json:"checked"`
}

// toolCostFile is the mcp/tool-costs.json content.
type toolCostFile struct {
	Version int `json:"version"`
	// Servers is, by mcp.projects root ("" for the global list) and then server name, the
	// last count a live probe made.
	Servers map[string]map[string]ToolCost `json:"servers"`
}

func (s *Service) toolCostsPath() string { return filepath.Join(s.StateDir, "mcp", "tool-costs.json") }

// readToolCosts reads the counts. They are only a cache of what a probe can count again, so an
// unreadable file reads as empty and the next live check replaces it.
func (s *Service) readToolCosts() toolCostFile {
	costs := toolCostFile{Version: 1, Servers: map[string]map[string]ToolCost{}}
	if s.StateDir == "" {
		return costs
	}
	data, exists, _, err := safeRead(s.toolCostsPath())
	if err != nil || !exists {
		return costs
	}
	var read toolCostFile
	if json.Unmarshal(data, &read) != nil || read.Version != 1 || read.Servers == nil {
		return costs
	}
	return read
}

// costScope is the cost file key for a server whose report entry names project as its root:
// a project-mode config's own servers carry no root, and the state directory is shared.
func (s *Service) costScope(project string) string {
	if project == "" {
		return s.configProjectRoot()
	}
	return project
}

// saveToolCosts records the counts, by report.Servers index, of the servers this check probed.
// A failure is a finding on those servers: the check itself still has its answer.
func (s *Service) saveToolCosts(report *CheckReport, costs []*ToolCost) {
	if !slices.ContainsFunc(costs, func(c *ToolCost) bool { return c != nil }) {
		return
	}
	err := func() error {
		lock, err := s.lock()
		if err != nil {
			return err
		}
		defer lock.Unlock()
		// Read again under the lock, so a concurrent check's counts are kept.
		file := s.readToolCosts()
		for i, cost := range costs {
			if cost == nil {
				continue
			}
			scope := s.costScope(report.Servers[i].Project)
			if file.Servers[scope] == nil {
				file.Servers[scope] = map[string]ToolCost{}
			}
			file.Servers[scope][report.Servers[i].Name] = *cost
		}
		return writeJSONFile(s.toolCostsPath(), file)
	}()
	if err == nil {
		return
	}
	for i, cost := range costs {
		if cost != nil {
			report.Servers[i].Findings = append(report.Servers[i].Findings, CheckFinding{Level: "warning", Check: "tool-cost", Message: fmt.Sprintf("could not save the tool token count: %v", err)})
		}
	}
}

// tokens estimates the context the tool's definition costs. The description is read as the
// text it decodes to and the schema as compact JSON, so escapes and indentation add nothing.
func (t liveTool) tokens() int {
	var description string
	_ = json.Unmarshal(t.Description, &description)
	var schema bytes.Buffer
	if json.Compact(&schema, t.InputSchema) != nil {
		schema.Reset()
	}
	return tokens.Estimate(t.Name + "\n" + description + "\n" + schema.String())
}

// ToolCostServer is one server an Agent loads, with the last count a live probe made.
type ToolCostServer struct {
	Name string `json:"name"`
	// Project is the mcp.projects root the server is declared under, as CheckServer names it.
	Project string `json:"project,omitempty"`
	Tools   int    `json:"tools"`
	Tokens  int    `json:"tokens"`
	// Checked is when a live probe last counted the server; nil when none has.
	Checked *time.Time `json:"checked,omitempty"`
}

// ToolCostGroup is what one target loads in one place: the global list, or one mcp.projects
// root. Tokens adds up only the servers a live probe has counted.
type ToolCostGroup struct {
	Root   string `json:"root,omitempty"`
	Target string `json:"target"`
	// Agent is the client the target writes for; an account target names its Agent here.
	Agent    string           `json:"agent"`
	Tokens   int              `json:"tokens"`
	Measured int              `json:"measured"`
	Servers  []ToolCostServer `json:"servers"`
}

// ToolCostReport is, per target and place, the tool definitions the synced servers cost.
type ToolCostReport struct {
	Groups []ToolCostGroup `json:"groups"`
}

// HasToolCosts reports whether a live check has counted any server yet, without planning.
func (s *Service) HasToolCosts() bool { return len(s.readToolCosts().Servers) > 0 }

// ToolCosts adds up, from the last live counts, the tool definitions each target loads once
// the source is synced. The plan decides which servers a target gets, so shared servers,
// overrides and turned-off entries count where sync writes them, and a project's group also
// holds the global servers it keeps on. A server no probe has counted is listed without a
// count instead of counting as zero. Tool policies are not applied: the totals are an upper
// bound.
func (s *Service) ToolCosts() (*ToolCostReport, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	plan, err := s.previewSource(source)
	if err != nil {
		return nil, err
	}
	costs := s.readToolCosts()
	type groupKey struct{ root, target string }
	groups := map[groupKey]*ToolCostGroup{}
	off := map[groupKey][]string{}
	add := func(g *ToolCostGroup, server ToolCostServer) {
		if server.Checked != nil {
			g.Tokens += server.Tokens
			g.Measured++
		}
		g.Servers = append(g.Servers, server)
	}
	for _, c := range plan.Changes {
		if c.Action == "remove" {
			continue
		}
		if c.Switch {
			off[groupKey{c.Root, c.Target}] = append(off[groupKey{c.Root, c.Target}], c.Name)
			continue
		}
		// A shared server is declared, and counted, once in the global list.
		project := c.Root
		if c.Root != "" && slices.Contains(source.Projects[c.Root].Use, c.Name) {
			project = ""
		}
		k := groupKey{c.Root, c.Target}
		g := groups[k]
		if g == nil {
			agent := c.Target
			if account, ok := source.Accounts[c.Target]; ok {
				agent = account.Agent
			}
			g = &ToolCostGroup{Root: c.Root, Target: c.Target, Agent: agent, Servers: []ToolCostServer{}}
			groups[k] = g
		}
		if slices.ContainsFunc(g.Servers, func(server ToolCostServer) bool { return server.Name == c.Name }) {
			continue
		}
		server := ToolCostServer{Name: c.Name, Project: project}
		if cost, ok := costs.Servers[s.costScope(project)][c.Name]; ok {
			checked := cost.Checked
			server.Tools, server.Tokens, server.Checked = cost.Tools, cost.Tokens, &checked
		}
		add(g, server)
	}
	// The Agent also loads its global servers in a project, except those the project turns off.
	for k, g := range groups {
		global := groups[groupKey{"", k.target}]
		if k.root == "" || global == nil {
			continue
		}
		for _, server := range global.Servers {
			if !slices.Contains(off[k], server.Name) && !slices.ContainsFunc(g.Servers, func(own ToolCostServer) bool { return own.Name == server.Name }) {
				add(g, server)
			}
		}
	}
	report := &ToolCostReport{Groups: []ToolCostGroup{}}
	for _, g := range groups {
		slices.SortFunc(g.Servers, func(a, b ToolCostServer) int {
			return cmp.Or(cmp.Compare(b.Tokens, a.Tokens), cmp.Compare(a.Name, b.Name))
		})
		report.Groups = append(report.Groups, *g)
	}
	slices.SortFunc(report.Groups, func(a, b ToolCostGroup) int {
		return cmp.Or(cmp.Compare(a.Root, b.Root), cmp.Compare(a.Target, b.Target))
	})
	return report, nil
}
