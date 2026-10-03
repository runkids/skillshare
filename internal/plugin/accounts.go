package plugin

import (
	"context"
	"slices"
	"sort"
)

// Account is a target that is another config directory of a built-in Agent, such as a
// second Claude account. Its plugins are installed by the Agent's own CLI, run against Dir.
type Account struct {
	Agent string
	Dir   string
	// CLI runs the plugin commands instead of the Agent's own executable, for a compatible
	// CLI such as omo for Pi. Empty means the Agent's.
	CLI string
}

// accountEnv names, for each Agent whose plugins follow its config directory, the
// environment variable that points its CLI at another one. Only these Agents have accounts.
var accountEnv = map[string]string{"claude": "CLAUDE_CONFIG_DIR", "codex": "CODEX_HOME", "pi": "PI_CODING_AGENT_DIR"}

// piForkEnv are the variables Pi forks read before PI_CODING_AGENT_DIR (senpi, and omo
// built on it). One left in the user's environment would win over the account's directory.
var piForkEnv = []string{"SENPI_CODING_AGENT_DIR", "OMO_CODING_AGENT_DIR"}

// env points an account's CLI at the account's directory.
func (a Account) env() []string {
	env := []string{accountEnv[a.Agent] + "=" + a.Dir}
	if a.Agent == "pi" {
		for _, name := range piForkEnv {
			env = append(env, name+"="+a.Dir)
		}
	}
	return env
}

// account resolves a target that is another config directory of an Agent. A project's
// plugins belong to the project rather than to one account, so accounts are global only.
func (s *Service) account(target string) (Account, bool) {
	a, ok := s.Accounts[target]
	if !ok || s.ProjectRoot != "" || a.Dir == "" {
		return Account{}, false
	}
	_, supported := accountEnv[a.Agent]
	return a, supported
}

// agentOf is the Agent that runs a target: an account's Agent, or the target itself.
func (s *Service) agentOf(target string) string {
	if a, ok := s.account(target); ok {
		return a.Agent
	}
	return target
}

// accountNames lists this configuration's accounts, in the order they are shown.
func (s *Service) accountNames() []string {
	names := []string{}
	for name := range s.Accounts {
		if _, ok := s.account(name); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// accountAgents maps each account to its Agent, for the checks that read config alone.
func (s *Service) accountAgents() map[string]string {
	agents := map[string]string{}
	for _, name := range s.accountNames() {
		agents[name] = s.Accounts[name].Agent
	}
	return agents
}

// targets are the plugin targets of this configuration: the built-in Agents and its accounts.
func (s *Service) targets() []string {
	return append(slices.Clone(Targets), s.accountNames()...)
}

// TargetDefinitions adds this configuration's accounts to the built-in targets. An account
// takes the operations of the Agent it is another config directory of.
func (s *Service) TargetDefinitions() []TargetDefinition {
	definitions := TargetDefinitions()
	for i := range definitions {
		definitions[i].Npm = s.npmTarget(definitions[i].Target)
	}
	accounts := []TargetDefinition{}
	for _, name := range s.accountNames() {
		for _, d := range definitions {
			if d.Target == s.Accounts[name].Agent {
				d.Target, d.Label, d.Project = name, name, false
				d.Npm = s.npmTarget(name)
				accounts = append(accounts, d)
				break
			}
		}
	}
	return append(definitions, accounts...)
}

// Discover resolves a source for this configuration. An account takes the entries of the
// Agent it is another config directory of, so it is offered wherever that Agent is.
func (s *Service) Discover(ctx context.Context, source, ref, entry string) (*Discovery, error) {
	d, err := DiscoverOptions(ctx, source, ref, entry)
	if err != nil {
		return nil, err
	}
	return s.withAccounts(d), nil
}

// DiscoverManaged is Discover for the package name already manages. Its reviewed snapshot
// answers when it is of the same source, so opening the plugin does not clone it again.
func (s *Service) DiscoverManaged(ctx context.Context, name, source, ref, entry string) (*Discovery, error) {
	if d := s.discoverSnapshot(name, source, ref, entry); d != nil {
		return s.withAccounts(d), nil
	}
	return s.Discover(ctx, source, ref, entry)
}

func (s *Service) withAccounts(d *Discovery) *Discovery {
	d.TargetDefinitions = s.TargetDefinitions()
	for i := range d.Candidates {
		c := &d.Candidates[i]
		for _, name := range s.accountNames() {
			info, ok := c.TargetInfo[s.Accounts[name].Agent]
			if !ok {
				continue
			}
			c.TargetInfo[name] = info
			if info.Problem == "" && !slices.Contains(c.Targets, name) {
				c.Targets = append(c.Targets, name)
			}
		}
		slices.Sort(c.Targets)
	}
	return d
}
