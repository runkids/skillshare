package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// CheckFinding is one result of a check. It never carries an environment value.
type CheckFinding struct {
	Level   string `json:"level"`
	Check   string `json:"check"`
	Target  string `json:"target"`
	Message string `json:"message"`
	// Subject is what the finding is about, for a client to phrase it: the variable name,
	// command, or host. Never a variable's value.
	Subject string `json:"subject,omitempty"`
}

// CheckServer holds the findings for one server; OK means none is an error.
type CheckServer struct {
	Name string `json:"name"`
	// Project is the mcp.projects root a server belongs to, as Source.Projects keys it;
	// empty for a global server.
	Project  string         `json:"project,omitempty"`
	OK       bool           `json:"ok"`
	Findings []CheckFinding `json:"findings"`
	// Live is what the server reported about itself; only CheckOptions.Live probes it.
	Live *CheckLive `json:"live,omitempty"`
}

type CheckSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// CheckReport answers, per server, whether it will work as synced.
type CheckReport struct {
	SourcePath string        `json:"-"`
	Servers    []CheckServer `json:"servers"`
	Summary    CheckSummary  `json:"summary"`
}

// CheckOptions selects servers and supplies the lookups, which tests replace.
type CheckOptions struct {
	Names      []string
	SkipDNS    bool
	LookupEnv  func(string) (string, bool)
	LookPath   func(string) (string, error)
	LookupHost func(context.Context, string) ([]string, error)
	// Live starts each stdio server and sends requests to each remote one, after the
	// static checks; nothing else in a check does either.
	Live bool
	// Timeout bounds each live probe; zero means DefaultLiveTimeout.
	Timeout time.Duration
	// ClientVersion is the clientInfo version a live probe sends.
	ClientVersion string
}

// dnsTimeout bounds each host lookup, the only network access a check makes.
const dnsTimeout = 3 * time.Second

// Check verifies the source's servers, including those under each mcp.projects root, without
// starting them or sending requests: referenced variables are set, commands resolve, hosts
// resolve, and the plan has each entry in sync. Only opts.Live then probes the servers.
func (s *Service) Check(opts CheckOptions) (*CheckReport, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.LookupHost == nil {
		opts.LookupHost = net.DefaultResolver.LookupHost
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	// Every server the plan covers: the global list, then each mcp.projects root.
	var all []checkKey
	for _, name := range sortedKeys(source.Servers) {
		all = append(all, checkKey{name: name})
	}
	for _, root := range sortedKeys(source.Projects) {
		for _, name := range sortedKeys(source.Projects[root].Servers) {
			all = append(all, checkKey{root: root, name: name})
		}
	}
	selected := all
	if len(opts.Names) > 0 {
		selected = nil
		for _, name := range opts.Names {
			found := false
			for _, k := range all {
				if k.name == name {
					found = true
					if !slices.Contains(selected, k) {
						selected = append(selected, k)
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("unknown MCP server %q; known servers: %s", name, knownNames(all))
			}
		}
	}
	report := &CheckReport{SourcePath: source.Path, Servers: []CheckServer{}}
	// Every server's client rules count, selected or not: the plan below covers them all.
	preflight := map[checkKey][]CheckFinding{}
	// refused holds the targets whose rules refuse a server; an empty name means all of them.
	refused := map[checkKey][]string{}
	for _, k := range all {
		preflight[k] = s.checkClientRules(source, k.root, k.name, k.server(source))
		for _, f := range preflight[k] {
			if f.Level == "error" {
				refused[k] = append(refused[k], f.Target)
			}
		}
	}
	for _, k := range selected {
		server := k.server(source)
		result := CheckServer{Name: k.name, Project: k.root, Findings: []CheckFinding{}}
		result.Findings = append(result.Findings, checkEnv(server, opts.LookupEnv)...)
		if !server.Disabled {
			result.Findings = append(result.Findings, checkLaunch(server, opts)...)
		}
		result.Findings = append(result.Findings, preflight[k]...)
		result.Findings = append(result.Findings, checkToolPolicy(source, k.root, server)...)
		report.Servers = append(report.Servers, result)
	}
	byKey := map[checkKey]*CheckServer{}
	for i := range report.Servers {
		byKey[checkKey{report.Servers[i].Project, report.Servers[i].Name}] = &report.Servers[i]
	}
	// Plan only what the Agents accept, so one refused entry does not hide the rest.
	accept := func(root string, servers map[string]Server, defaults []string) map[string]Server {
		out := map[string]Server{}
		for name, server := range servers {
			k := checkKey{root, name}
			if len(refused[k]) > 0 {
				if slices.Contains(refused[k], "") {
					continue
				}
				server.Targets = slices.DeleteFunc(slices.Clone(server.Targets.orDefault(defaults)), func(target string) bool {
					return slices.Contains(refused[k], target)
				})
			}
			out[name] = server
		}
		return out
	}
	accepted := *source
	accepted.Servers = accept("", source.Servers, source.Targets)
	if source.Projects != nil {
		accepted.Projects = map[string]Project{}
		for root, project := range source.Projects {
			project.Servers = accept(root, project.Servers, project.defaults(source))
			accepted.Projects[root] = project
		}
	}
	p, err := s.previewSource(&accepted)
	if err != nil {
		return nil, err
	}
	var synced []Change
	for _, c := range p.Changes {
		k := checkKey{c.Root, c.Name}
		if byKey[k] != nil && !slices.Contains(refused[k], c.Target) {
			synced = append(synced, c)
		}
	}
	slices.SortStableFunc(synced, func(a, b Change) int { return strings.Compare(a.Target, b.Target) })
	for _, c := range synced {
		result := byKey[checkKey{c.Root, c.Name}]
		result.Findings = append(result.Findings, syncFinding(c))
	}
	if opts.Live {
		s.checkLive(source, report, opts)
	}
	for i := range report.Servers {
		result := &report.Servers[i]
		result.OK = true
		for _, f := range result.Findings {
			switch f.Level {
			case "error":
				result.OK = false
				report.Summary.Errors++
			case "warning":
				report.Summary.Warnings++
			}
		}
	}
	return report, nil
}

// checkEnv reports each fromEnv reference whose variable is unset or empty.
func checkEnv(server Server, lookup func(string) (string, bool)) []CheckFinding {
	var out []CheckFinding
	missing := func(field string, v Value) {
		if v.FromEnv == "" {
			return
		}
		if value, ok := lookup(v.FromEnv); !ok || value == "" {
			out = append(out, CheckFinding{Level: "error", Check: "env", Subject: v.FromEnv, Message: fmt.Sprintf("%s reads %s, which is not set", field, v.FromEnv)})
		}
	}
	for _, key := range sortedKeys(server.Env) {
		missing("env "+key, server.Env[key])
	}
	for _, key := range sortedKeys(server.Headers) {
		missing("header "+key, server.Headers[key])
	}
	if server.BearerToken != nil {
		missing("bearerToken", *server.BearerToken)
	}
	return out
}

// checkLaunch resolves a stdio command on PATH, or a remote host through DNS.
func checkLaunch(server Server, opts CheckOptions) []CheckFinding {
	if server.Command != "" {
		command, err := expandHome(server.Command)
		if err == nil {
			_, err = opts.LookPath(command)
		}
		if err != nil {
			return []CheckFinding{{Level: "error", Check: "command", Subject: server.Command, Message: fmt.Sprintf("command %s was not found on PATH", server.Command)}}
		}
		return nil
	}
	u, err := url.Parse(server.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return []CheckFinding{{Level: "error", Check: "url", Message: "url is not a valid HTTP(S) URL"}}
	}
	host := u.Hostname()
	if opts.SkipDNS || net.ParseIP(host) != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()
	if _, err := opts.LookupHost(ctx, host); err != nil {
		return []CheckFinding{{Level: "warning", Check: "dns", Subject: host, Message: fmt.Sprintf("host %s did not resolve", host)}}
	}
	return nil
}

// checkClientRules renders the server alone, once per target, so a rule an Agent enforces
// names that Agent. A project server is rendered in its root. A server that names no targets
// is kept in Skillshare only.
func (s *Service) checkClientRules(source *Source, root, name string, server Server) []CheckFinding {
	if server.Targets != nil && len(server.Targets) == 0 {
		return []CheckFinding{{Level: "info", Check: "targets", Message: "kept in Skillshare only; no Agent receives it"}}
	}
	defaults := source.Targets
	if root != "" {
		project := source.Projects[root]
		defaults = project.defaults(source)
		// Rendered alone, a switch-only entry could not see what decides its targets.
		server = followingSwitches(project.Servers, defaults, source)[name]
	}
	render := func(server Server) error {
		alone := *source
		alone.Servers = map[string]Server{}
		alone.Projects = nil
		if root == "" {
			alone.Servers[name] = server
		} else {
			project := source.Projects[root]
			project.Servers = map[string]Server{name: server}
			alone.Projects = map[string]Project{root: project}
		}
		_, err := s.render(&alone)
		if err != nil && root != "" {
			// The finding already names its project.
			return errors.New(strings.TrimPrefix(err.Error(), root+": "))
		}
		return err
	}
	selected := server.Targets.orDefault(defaults)
	// A switch-only entry works out its own targets, so it is rendered as written.
	if server.Disabled || len(selected) == 0 {
		if err := render(server); err != nil {
			return []CheckFinding{{Level: "error", Check: "client-rule", Message: err.Error()}}
		}
		return nil
	}
	var out []CheckFinding
	for _, target := range selected {
		one := server
		one.Targets = TargetList{target}
		if err := render(one); err != nil {
			out = append(out, CheckFinding{Level: "error", Check: "client-rule", Target: target, Message: err.Error()})
		}
	}
	return out
}

// checkToolPolicy names each selected Agent that cannot hold a part of the server's tool
// policy, the same gaps the plan's notices list.
func checkToolPolicy(source *Source, root string, server Server) []CheckFinding {
	defaults := source.Targets
	if root != "" {
		defaults = source.Projects[root].defaults(source)
	}
	var out []CheckFinding
	for _, target := range server.Targets.orDefault(defaults) {
		agent := target
		if account, ok := source.Accounts[target]; ok {
			agent = account.Agent
		}
		if gaps := toolPolicyGaps(agent, server.Tools); len(gaps) > 0 {
			out = append(out, CheckFinding{Level: "warning", Check: "tools", Target: target, Message: toolPolicyMessage(target, gaps)})
		}
	}
	return out
}

// syncFinding says whether an Agent's entry matches what the source asks for.
func syncFinding(c Change) CheckFinding {
	f := CheckFinding{Level: "info", Check: "sync", Target: c.Target, Message: "in sync"}
	switch c.Action {
	case "unchanged":
		// Claude's local scope server of the same name wins over the synced one.
		if c.Message != "" {
			f.Level, f.Message = "warning", c.Message
		}
	case "adopt":
		f.Message = "already present; the next sync takes it over"
	case "add", "update":
		f.Level, f.Message = "warning", "not synced yet; run skillshare sync mcp"
	case "remove":
		f.Level, f.Message = "warning", "not synced yet; the next sync removes it from this Agent"
	case "conflict":
		f.Level, f.Message = "error", c.Message
	default:
		f.Level, f.Message = "warning", c.Action
	}
	return f
}

// checkKey names a server in its scope: a root under mcp.projects, or empty for the global list.
type checkKey struct{ root, name string }

func (k checkKey) server(source *Source) Server {
	if k.root == "" {
		return source.Servers[k.name]
	}
	return source.Projects[k.root].Servers[k.name]
}

// knownNames lists each server name once, whichever scopes declare it.
func knownNames(keys []checkKey) string {
	var names []string
	for _, k := range keys {
		if !slices.Contains(names, k.name) {
			names = append(names, k.name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// defaults is the list a project's servers go to when they name none.
func (p Project) defaults(source *Source) []string {
	if p.Targets == nil {
		return source.Targets
	}
	return p.Targets
}

// orDefault is the list a server is written to: its own, or the config's default.
func (t TargetList) orDefault(defaults []string) TargetList {
	if t == nil {
		return defaults
	}
	return t
}
