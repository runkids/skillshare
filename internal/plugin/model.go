// Package plugin manages complete plugins through their native host CLIs.
// Plugin components are never published as standalone skills or MCP entries.
package plugin

import "context"

var Targets = []string{"claude", "codex", "cursor", "antigravity", "pi", "opencode"}

type Binding struct {
	Components []string `json:"components,omitempty" yaml:"components,omitempty"`
	Sync       *bool    `json:"sync,omitempty" yaml:"sync,omitempty"`
	ID         string   `json:"id" yaml:"id"`
	Source     string   `json:"source,omitempty" yaml:"source,omitempty"`
	Plugin     string   `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	Digest     string   `json:"digest,omitempty" yaml:"digest,omitempty"`
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	Pending    string   `json:"pending,omitempty" yaml:"pending,omitempty"`
}

type Package struct {
	Bindings map[string]Binding `json:"bindings" yaml:"bindings"`
}

type Request struct {
	Action  string   `json:"action"`
	Name    string   `json:"name,omitempty"`
	Source  string   `json:"source,omitempty"`
	Plugin  string   `json:"plugin,omitempty"`
	Targets []string `json:"targets,omitempty"`
	From    string   `json:"from,omitempty"`
}

type Candidate struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Marketplace string   `json:"marketplace,omitempty"`
	Path        string   `json:"path"`
	Entry       string   `json:"entry,omitempty"`
	Targets     []string `json:"targets"`
	Components  []string `json:"components"`
	Problem     string   `json:"problem,omitempty"`
}

type Discovery struct {
	Source     string      `json:"source"`
	Digest     string      `json:"digest"`
	Candidates []Candidate `json:"candidates"`
}

type Installed struct {
	ID          string `json:"id,omitempty"`
	PluginID    string `json:"pluginId,omitempty"`
	Name        string `json:"name,omitempty"`
	Marketplace string `json:"marketplaceName,omitempty"`
	Version     string `json:"version,omitempty"`
	Scope       string `json:"scope,omitempty"`
	ProjectPath string `json:"projectPath,omitempty"`
	Enabled     bool   `json:"enabled"`
	Installed   bool   `json:"installed"`
	Filtered    bool   `json:"filtered,omitempty"`
}

type Host struct {
	Target      string      `json:"target"`
	Fingerprint string      `json:"fingerprint,omitempty"`
	Note        string      `json:"note,omitempty"`
	Version     string      `json:"version"`
	Error       string      `json:"error,omitempty"`
	Installed   []Installed `json:"installed"`
}

type Inventory struct {
	Packages map[string]Package `json:"packages"`
	Hosts    []Host             `json:"hosts"`
}

type Change struct {
	Name       string   `json:"name"`
	Target     string   `json:"target"`
	ID         string   `json:"id"`
	Action     string   `json:"action"`
	Message    string   `json:"message,omitempty"`
	Binding    Binding  `json:"binding"`
	Components []string `json:"components,omitempty"`
}

type Plan struct {
	Revision string   `json:"revision"`
	Blocked  bool     `json:"blocked"`
	Changes  []Change `json:"changes"`
}

type Outcome struct {
	Name    string `json:"name"`
	Target  string `json:"target"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Result struct {
	Results []Outcome `json:"results"`
}

type Runner func(context.Context, string, string, ...string) ([]byte, error)

type Service struct {
	ConfigPath  string
	StateDir    string
	ProjectRoot string
	Run         Runner
}

func (b Binding) Selected() bool { return b.Sync == nil || *b.Sync }
