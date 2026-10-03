// Package plugin manages complete plugins through their native host CLIs.
// Plugin components are never published as standalone skills or MCP entries.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
)

var Targets = func() []string {
	var targets []string
	for _, d := range TargetDefinitions() {
		targets = append(targets, d.Target)
	}
	return targets
}()

type Binding struct {
	// PiRegistration references a private, content-addressed native entry. Values
	// never enter shared config or API responses; only its digest is recorded.
	PiRegistration string   `json:"piRegistration,omitempty" yaml:"pi_registration,omitempty"`
	Entry          string   `json:"entry,omitempty" yaml:"entry,omitempty"`
	SourceRef      string   `json:"sourceRef,omitempty" yaml:"source_ref,omitempty"`
	Commit         string   `json:"commit,omitempty" yaml:"commit,omitempty"`
	Components     []string `json:"components,omitempty" yaml:"components,omitempty"`
	Sync           *bool    `json:"sync,omitempty" yaml:"sync,omitempty"`
	ID             string   `json:"id" yaml:"id"`
	Source         string   `json:"source,omitempty" yaml:"source,omitempty"`
	Plugin         string   `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	Digest         string   `json:"digest,omitempty" yaml:"digest,omitempty"`
	Version        string   `json:"version,omitempty" yaml:"version,omitempty"`
	Pending        string   `json:"pending,omitempty" yaml:"pending,omitempty"`
}

// Source, SourceRef, Plugin, Entry and Version keep a package managed while no Agent is bound,
// so Agents can be bound later from the same source and the dashboard can still show its version.
type Package struct {
	Version   string             `json:"version,omitempty" yaml:"version,omitempty"`
	Source    string             `json:"source,omitempty" yaml:"source,omitempty"`
	SourceRef string             `json:"sourceRef,omitempty" yaml:"source_ref,omitempty"`
	Plugin    string             `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	Entry     string             `json:"entry,omitempty" yaml:"entry,omitempty"`
	Bindings  map[string]Binding `json:"bindings" yaml:"bindings"`
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

// ProblemKey names the sentence the dashboard can translate and ProblemArgs fill its
// {placeholders}. Problem stays the English the CLI prints, and is the fallback.
type TargetPackage struct {
	Manifest string `json:"manifest"`
	// Path is set when this Agent's catalog points at its own folder; empty means Candidate.Path.
	Path        string            `json:"path,omitempty"`
	Version     string            `json:"version,omitempty"`
	Logo        string            `json:"logo,omitempty"`
	Entry       string            `json:"entry,omitempty"`
	Components  []string          `json:"components"`
	Problem     string            `json:"problem,omitempty"`
	ProblemKey  string            `json:"problemKey,omitempty"`
	ProblemArgs map[string]string `json:"problemArgs,omitempty"`
}

// block records why this Agent cannot take the package.
func (p *TargetPackage) block(key, message string, args map[string]string) {
	p.Problem, p.ProblemKey, p.ProblemArgs = message, key, args
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
	ProblemKey  string                   `json:"problemKey,omitempty"`
	ProblemArgs map[string]string        `json:"problemArgs,omitempty"`
	// catalogEntry keeps the component fields a Claude catalog entry defines itself, for the
	// catalog Skillshare writes at install time (strict: false plugins have nothing else).
	catalogEntry map[string]json.RawMessage
}

// pathFor is the folder, relative to the source, that target installs from.
func (c Candidate) pathFor(target string) string {
	if p := c.TargetInfo[target].Path; p != "" {
		return p
	}
	return c.Path
}

// collectTargets derives Targets, Components and the OpenCode entry from TargetInfo.
func (c *Candidate) collectTargets() {
	c.Targets, c.Components = []string{}, []string{}
	for target, info := range c.TargetInfo {
		if info.Problem != "" {
			continue
		}
		c.Targets = append(c.Targets, target)
		for _, component := range info.Components {
			if !slices.Contains(c.Components, component) {
				c.Components = append(c.Components, component)
			}
		}
		if target == "opencode" {
			c.Entry = info.Entry
		}
	}
	slices.Sort(c.Targets)
	slices.Sort(c.Components)
}

// block records why this package cannot be installed from the source.
func (c *Candidate) block(key, message string, args map[string]string) {
	c.Problem, c.ProblemKey, c.ProblemArgs = message, key, args
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
	Importable   bool   `json:"importable,omitempty"`
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
	// Marketplaces maps each Claude/Codex marketplace name to its root; nil when unknown.
	Marketplaces map[string]string `json:"marketplaces,omitempty"`
	// ManagedMarketplaces names the ones Skillshare registered from its own state directory.
	ManagedMarketplaces []string `json:"managedMarketplaces,omitempty"`
}

// fail records a failure and buckets it by its cause. A failed inventory call leaves
// Installed nil, which encodes as null and breaks the dashboard's list.
func (h *Host) fail(err error) {
	if h.Installed == nil {
		h.Installed = []Installed{}
	}
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
	PreservedKeys  []string `json:"preservedKeys,omitempty"`
	piRecord       []byte
	piSettingsHash string
	piRestores     map[string]piRestoreReceipt
	// piRecordAfter records the entry Pi writes on install, which keeps another version's filters.
	piRecordAfter bool
	Name          string `json:"name"`
	Target        string `json:"target"`
	ID            string `json:"id"`
	Action        string `json:"action"`
	Message       string `json:"message,omitempty"`
	// MessageKey names Message for the dashboard to translate, with MessageArgs.
	MessageKey  string            `json:"messageKey,omitempty"`
	MessageArgs map[string]string `json:"messageArgs,omitempty"`
	Binding     Binding           `json:"binding"`
	Components  []string          `json:"components,omitempty"`
	// Logo is the package logo for a change with no Agent, as a data: URI.
	Logo string `json:"logo,omitempty"`
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
	// MessageKey names Message for the dashboard to translate, with MessageArgs.
	MessageKey  string            `json:"messageKey,omitempty"`
	MessageArgs map[string]string `json:"messageArgs,omitempty"`
}

type Result struct {
	Results []Outcome `json:"results"`
}

// Runner runs one native CLI: a working directory, environment entries on top of the
// process environment, the binary and its arguments.
type Runner func(context.Context, string, []string, string, ...string) ([]byte, error)

type Service struct {
	ConfigPath  string
	StateDir    string
	ProjectRoot string
	// Accounts are the targets that are another config directory of a built-in Agent,
	// keyed by the name the config gives them.
	Accounts map[string]Account
	// ExtrasSources maps each extra to its source folder, so a Pi extension that an
	// extra links into Pi's folder is shown as that extra's file.
	ExtrasSources map[string]string
	Run           Runner
}

func (b Binding) Selected() bool { return b.Sync == nil || *b.Sync }
