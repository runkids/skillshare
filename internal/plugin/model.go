// Package plugin manages complete plugins through their native host CLIs.
// Plugin components are never published as standalone skills or MCP entries.
package plugin

import (
	"context"
	"errors"
)

var Targets = func() []string {
	var targets []string
	for _, d := range TargetDefinitions() {
		targets = append(targets, d.Target)
	}
	return targets
}()

type Binding struct {
	Entry      string   `json:"entry,omitempty" yaml:"entry,omitempty"`
	SourceRef  string   `json:"sourceRef,omitempty" yaml:"source_ref,omitempty"`
	Commit     string   `json:"commit,omitempty" yaml:"commit,omitempty"`
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
	Entry     string   `json:"entry,omitempty"`
	SourceRef string   `json:"sourceRef,omitempty"`
	Action    string   `json:"action"`
	Name      string   `json:"name,omitempty"`
	Source    string   `json:"source,omitempty"`
	Plugin    string   `json:"plugin,omitempty"`
	Targets   []string `json:"targets,omitempty"`
	From      string   `json:"from,omitempty"`
}

type TargetPackage struct {
	Manifest   string   `json:"manifest"`
	Version    string   `json:"version,omitempty"`
	Entry      string   `json:"entry,omitempty"`
	Components []string `json:"components"`
	Problem    string   `json:"problem,omitempty"`
}

type Candidate struct {
	TargetInfo  map[string]TargetPackage `json:"targetInfo,omitempty"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Version     string                   `json:"version"`
	Marketplace string                   `json:"marketplace,omitempty"`
	Path        string                   `json:"path"`
	Entry       string                   `json:"entry,omitempty"`
	Targets     []string                 `json:"targets"`
	Components  []string                 `json:"components"`
	Problem     string                   `json:"problem,omitempty"`
}

type Discovery struct {
	Warnings          []string           `json:"warnings,omitempty"`
	TargetDefinitions []TargetDefinition `json:"targetDefinitions"`
	SourceRef         string             `json:"sourceRef,omitempty"`
	Commit            string             `json:"commit,omitempty"`
	Source            string             `json:"source"`
	Digest            string             `json:"digest"`
	Candidates        []Candidate        `json:"candidates"`
}

type Installed struct {
	ID           string `json:"id,omitempty"`
	PluginID     string `json:"pluginId,omitempty"`
	Name         string `json:"name,omitempty"`
	Marketplace  string `json:"marketplaceName,omitempty"`
	Version      string `json:"version,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ProjectPath  string `json:"projectPath,omitempty"`
	EnabledKnown bool   `json:"enabledKnown"`
	Enabled      bool   `json:"enabled"`
	Installed    bool   `json:"installed"`
	Filtered     bool   `json:"filtered,omitempty"`
}

// What a caller would do about a host, so the dashboard can group Agents by the
// remedy instead of reading the English message back out of Error.
const (
	HostReady   = "ready"   // usable
	HostMissing = "missing" // its CLI is not installed or not on PATH here
	HostBlocked = "blocked" // reachable, but the Agent itself has to be dealt with
)

// NoteKey and ErrorKey name the fixed sentences the dashboard can translate. Note and
// Error stay the English the CLI prints, and are the fallback when a locale lacks the key.
// A key is absent whenever the message is assembled at runtime.
type Host struct {
	Target      string      `json:"target"`
	Fingerprint string      `json:"fingerprint,omitempty"`
	Note        string      `json:"note,omitempty"`
	NoteKey     string      `json:"noteKey,omitempty"`
	Version     string      `json:"version"`
	Status      string      `json:"status"`
	Error       string      `json:"error,omitempty"`
	ErrorKey    string      `json:"errorKey,omitempty"`
	Installed   []Installed `json:"installed"`
}

// fail records a failure and buckets it by its cause.
func (h *Host) fail(err error) {
	h.Error = err.Error()
	h.ErrorKey = ""
	h.Status = HostBlocked
	var keyed agentError
	if errors.As(err, &keyed) {
		h.ErrorKey = keyed.key
	}
	if errors.Is(err, ErrCLIMissing) {
		h.Status = HostMissing
	}
}

// block records a failure the caller resolves in the Agent, not by installing anything.
// key names the sentence for the dashboard; message is the English the CLI prints.
func (h *Host) block(key, message string) {
	h.Error = message
	h.ErrorKey = key
	h.Status = HostBlocked
}

type Inventory struct {
	TargetDefinitions []TargetDefinition `json:"targetDefinitions"`
	Packages          map[string]Package `json:"packages"`
	Hosts             []Host             `json:"hosts"`
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
