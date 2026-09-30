package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ToolPolicy says which of a server's tools reach the model. It is written once and
// translated per Agent; the plan and check name each Agent that cannot hold a part of it.
type ToolPolicy struct {
	// Expose is how the model reaches the tools: direct, deferred (loaded through tool
	// search) or hidden. Empty leaves each Agent's default.
	Expose string `yaml:"expose,omitempty" json:"expose,omitempty"`
	// Allow, when set, keeps only the tools that match. Deny removes the tools that match and
	// beats Allow. Entries are tool names in which * matches any characters.
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"`
	Deny  []string `yaml:"deny,omitempty" json:"deny,omitempty"`
}

// ToolExposures are the values of ToolPolicy.Expose.
var ToolExposures = []string{"direct", "deferred", "hidden"}

// IsZero makes an empty policy mean no policy, and keeps it out of YAML and JSON.
func (t ToolPolicy) IsZero() bool { return t.Expose == "" && len(t.Allow) == 0 && len(t.Deny) == 0 }

// toolPattern leaves out the wildcards other than * and the comma the CLI splits lists on.
var toolPattern = regexp.MustCompile(`^[^\s,?\[\]{}]+$`)

func (t ToolPolicy) validate(name string) error {
	if t.Expose != "" && !slices.Contains(ToolExposures, t.Expose) {
		return fmt.Errorf("MCP %s: tools.expose must be direct, deferred or hidden", name)
	}
	for part, patterns := range map[string][]string{"allow": t.Allow, "deny": t.Deny} {
		for i, pattern := range patterns {
			if !toolPattern.MatchString(pattern) {
				return fmt.Errorf("MCP %s: tools.%s: %q is not a tool name; use letters, digits and punctuation, with * matching any characters", name, part, pattern)
			}
			if slices.Contains(patterns[:i], pattern) {
				return fmt.Errorf("MCP %s: tools.%s lists %q twice", name, part, pattern)
			}
		}
	}
	if len(t.Allow) > 0 && !slices.ContainsFunc(t.Allow, func(tool string) bool { return isToolPattern(tool) || !t.denied(tool) }) {
		return fmt.Errorf("MCP %s: tools.deny removes every tool tools.allow keeps", name)
	}
	return nil
}

func isToolPattern(tool string) bool { return strings.Contains(tool, "*") }

func matchTool(pattern, tool string) bool {
	return regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$").MatchString(tool)
}

// denied reports a tool name that a deny entry matches.
func (t ToolPolicy) denied(tool string) bool {
	return slices.ContainsFunc(t.Deny, func(pattern string) bool { return matchTool(pattern, tool) })
}

// piToolPolicy writes a policy into a Pi entry. Pi takes an exact tool name over any
// pattern, and among patterns the first that matches, so the denied tools come first,
// then the allowed ones, then "*": "hidden" to close the list. An allowed name that a
// denied pattern matches is left out, or it would win over that pattern. Allowed tools
// get the policy's exposure, or Pi's default, codemode, when that is unset or hidden.
func piToolPolicy(entry map[string]any, t ToolPolicy) {
	if t.Expose != "" {
		entry["exposure"] = t.Expose
	}
	allowed := "codemode"
	if t.Expose == "direct" || t.Expose == "deferred" {
		allowed = t.Expose
	}
	var exposure piToolExposure
	add := func(tool, value string) {
		if !slices.ContainsFunc(exposure, func(e piExposureEntry) bool { return e.Key == tool }) {
			exposure = append(exposure, piExposureEntry{tool, value})
		}
	}
	for _, tool := range t.Deny {
		add(tool, "hidden")
	}
	for _, tool := range t.Allow {
		if isToolPattern(tool) || !t.denied(tool) {
			add(tool, allowed)
		}
	}
	if len(t.Allow) > 0 {
		add("*", "hidden")
	}
	if len(exposure) > 0 {
		entry["toolExposure"] = exposure
	}
}

// namedTools is a policy for an Agent that lists exact tool names, and the parts it cannot
// hold. allow is nil when every tool stays. A deny list (denyList) takes denied names as
// they are; otherwise, or for denied patterns, an allow list of names leaves them out.
func namedTools(t ToolPolicy, denyList bool) (allow, deny, gaps []string) {
	all := len(t.Allow) == 0 || slices.Contains(t.Allow, "*")
	names := !all && !slices.ContainsFunc(t.Allow, isToolPattern)
	if t.Expose != "" {
		gaps = append(gaps, "expose")
	}
	if !all && !names {
		gaps = append(gaps, "allow patterns")
	}
	switch {
	case len(t.Deny) == 0:
	case denyList && !slices.ContainsFunc(t.Deny, isToolPattern):
		deny = slices.Clone(t.Deny)
	case names:
	case !denyList:
		gaps = append(gaps, "deny")
	default:
		gaps = append(gaps, "deny patterns")
	}
	if names {
		allow = slices.Clone(t.Allow)
		if deny == nil {
			allow = slices.DeleteFunc(allow, t.denied)
		}
	}
	return allow, deny, gaps
}

// toolPolicyGaps names the parts of a policy an Agent cannot hold. Pi holds all of it;
// Codex and Copilot list exact tool names, and Copilot has no deny list. OpenCode and Kilo
// Code filter tools only in a permission section outside the server's entry.
func toolPolicyGaps(agent string, t ToolPolicy) []string {
	switch {
	case t.IsZero() || agent == "pi":
		return nil
	case agent == "codex" || agent == "copilot":
		_, _, gaps := namedTools(t, agent == "codex")
		return gaps
	}
	var gaps []string
	if t.Expose != "" {
		gaps = append(gaps, "expose")
	}
	if len(t.Allow) > 0 {
		gaps = append(gaps, "allow")
	}
	if len(t.Deny) > 0 {
		gaps = append(gaps, "deny")
	}
	return gaps
}

func toolPolicyMessage(target string, gaps []string) string {
	return "tool policy not applied for " + target + ": " + strings.Join(gaps, ", ")
}

// toolPolicyNotices names, per selected target, the servers whose tool policy that Agent
// cannot hold, so no part of a policy is dropped without saying so.
func (s *Source) toolPolicyNotices() []string {
	found := map[string][]string{}
	add := func(servers map[string]Server, defaults []string, root string) {
		for name, server := range servers {
			if root != "" {
				name += " (" + root + ")"
			}
			for _, target := range server.Targets.orDefault(defaults) {
				agent := target
				if account, ok := s.Accounts[target]; ok {
					agent = account.Agent
				}
				if gaps := toolPolicyGaps(agent, server.Tools); len(gaps) > 0 {
					message := toolPolicyMessage(target, gaps)
					found[message] = append(found[message], name)
				}
			}
		}
	}
	add(s.Servers, s.Targets, "")
	for root, project := range s.Projects {
		add(project.Servers, project.defaults(s), root)
	}
	var notices []string
	for _, message := range sortedKeys(found) {
		slices.Sort(found[message])
		notices = append(notices, message+" ("+strings.Join(found[message], ", ")+")")
	}
	return notices
}

// directToolsExposure is what pi-mcp-adapter's directTools becomes in Pi's built-in MCP:
// true declares every tool to the model, like Pi's direct; "search" holds them until a
// search loads them, like Pi's deferred; a list declares those tools directly and leaves
// the rest at Pi's default. false, the adapter's default, and an invalid value give nothing.
func directToolsExposure(value any) (exposure string, direct []string) {
	switch v := value.(type) {
	case bool:
		if v {
			return "direct", nil
		}
	case string:
		if v == "search" {
			return "deferred", nil
		}
	case []string:
		return "", v
	case []any:
		for _, item := range v {
			if tool, ok := item.(string); ok && tool != "" {
				direct = append(direct, tool)
			}
		}
	}
	return "", direct
}

// adoptAdapterTools moves pi-mcp-adapter's tool settings into what Pi's built-in MCP reads.
// includeTools and excludeTools become tools.allow and tools.deny. directTools becomes the
// exposure: tools.expose next to them, otherwise piOptions.exposure, or for a list of names
// piOptions.toolExposure, which unlike tools.allow keeps the other tools. It returns each
// setting it dropped because the server already sets that part, or it does not convert.
func (s *Server) adoptAdapterTools(directTools any, include, exclude []any) (dropped []string) {
	exposure, direct := directToolsExposure(directTools)
	piSet := s.PiOptions["exposure"] != nil || s.PiOptions["toolExposure"] != nil
	if include != nil || exclude != nil {
		policy := ToolPolicy{Allow: toolNames(include), Deny: toolNames(exclude)}
		if s.Disabled || !s.Tools.IsZero() || piSet || len(policy.Allow) != len(include) || len(policy.Deny) != len(exclude) || policy.validate("") != nil {
			for key, list := range map[string][]any{"excludeTools": exclude, "includeTools": include} {
				if list != nil {
					dropped = append(dropped, key)
				}
			}
			slices.Sort(dropped)
		} else {
			policy.Expose = exposure
			s.Tools = policy
			if exposure != "" {
				return dropped
			}
		}
	}
	switch {
	case exposure == "" && direct == nil:
		if directTools != nil && directTools != false {
			dropped = append(dropped, "directTools")
		}
	case s.Disabled || !s.Tools.IsZero() || piSet:
		dropped = append(dropped, "directTools")
	default:
		if s.PiOptions == nil {
			s.PiOptions = PiOptions{}
		}
		if exposure != "" {
			s.PiOptions["exposure"] = exposure
		} else {
			var tools piToolExposure
			for _, tool := range direct {
				tools = append(tools, piExposureEntry{tool, "direct"})
			}
			s.PiOptions["toolExposure"] = tools
		}
	}
	return dropped
}

// toolNames keeps the strings of a decoded list.
func toolNames(list []any) []string {
	var names []string
	for _, item := range list {
		if name, ok := item.(string); ok {
			names = append(names, name)
		}
	}
	return names
}

// piImportPolicy reads Pi's exposure and toolExposure back into a tool policy when writing
// that policy gives Pi exactly the same settings, and otherwise reports false.
func piImportPolicy(options PiOptions) (ToolPolicy, bool) {
	var t ToolPolicy
	if value, set := options["exposure"]; set {
		text, _ := value.(string)
		if !slices.Contains(ToolExposures, text) {
			return t, false
		}
		t.Expose = text
	}
	data, _ := json.Marshal(options["toolExposure"])
	entries, err := orderedExposure(data)
	if options["toolExposure"] != nil && err != nil {
		return t, false
	}
	for _, e := range entries {
		switch {
		case e.Value == "hidden" && len(t.Allow) == 0 && e.Key != "*":
			t.Deny = append(t.Deny, e.Key)
		case e.Value != "hidden" && e.Key != "*":
			t.Allow = append(t.Allow, e.Key)
		}
	}
	if t.validate("") != nil {
		return t, false
	}
	want := map[string]any{}
	piToolPolicy(want, t)
	got := map[string]any{}
	for _, key := range []string{"exposure", "toolExposure"} {
		if value, set := options[key]; set {
			got[key] = value
		}
	}
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	return t, string(a) == string(b)
}
