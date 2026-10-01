package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// globalSpec finds a built-in target by name or alias.
func globalSpec(name string) (targetSpec, bool) {
	specs, _ := loadTargetSpecs()
	for _, spec := range specs {
		if spec.Name != "" && targetSpecMatchesName(spec, name) {
			return spec, true
		}
	}
	return targetSpec{}, false
}

// AgentConfigDirAgents lists the built-in Agents a target can be another config directory of.
func AgentConfigDirAgents() []string {
	specs, _ := loadTargetSpecs()
	var names []string
	for _, spec := range specs {
		if spec.ConfigDir != "" {
			names = append(names, spec.Name)
		}
	}
	return names
}

// AgentConfigDir is the default config directory of a built-in Agent, or "" when a
// target cannot be another config directory of it.
func AgentConfigDir(agent string) string {
	spec, ok := globalSpec(agent)
	if !ok || spec.ConfigDir == "" {
		return ""
	}
	return normalizeTargetPath(spec.ConfigDir)
}

// AgentConfigDirTargets names the targets that are another config directory of an Agent.
func (c *Config) AgentConfigDirTargets() []string {
	var names []string
	for name, target := range c.Targets {
		if target.Agent != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// AgentConfigDirTargetAgents maps each target that is another config directory of an Agent
// to that Agent.
func (c *Config) AgentConfigDirTargetAgents() map[string]string {
	agents := map[string]string{}
	for _, name := range c.AgentConfigDirTargets() {
		agents[name] = c.Targets[name].Agent
	}
	return agents
}

// AgentConfigDirPaths returns where an Agent keeps skills and agents when its config
// directory is dir instead of the default one. agents is empty for an Agent without any.
func AgentConfigDirPaths(agent, dir string) (skills, agents string, err error) {
	spec, ok := globalSpec(agent)
	if !ok {
		return "", "", fmt.Errorf("unknown Agent %q", agent)
	}
	if spec.ConfigDir == "" {
		return "", "", fmt.Errorf("%s has no config directory of its own to move; supported: %s", spec.Name, strings.Join(AgentConfigDirAgents(), ", "))
	}
	dir = expandPath(dir)
	if !filepath.IsAbs(dir) {
		return "", "", fmt.Errorf("config_dir %q must be an absolute path or start with ~", dir)
	}
	base := normalizeTargetPath(spec.ConfigDir)
	if filepath.Clean(dir) == filepath.Clean(base) {
		return "", "", fmt.Errorf("config_dir %s is where %s already keeps its files; use the %s target", dir, spec.Name, spec.Name)
	}
	move := func(path string) string {
		rel, err := filepath.Rel(base, normalizeTargetPath(path))
		if path == "" || err != nil || strings.HasPrefix(rel, "..") {
			return ""
		}
		return filepath.Join(dir, rel)
	}
	// An Agent may keep its primary skills directory outside its own config directory
	// (Codex reads the shared ~/.agents/skills), while also reading one inside it. An
	// account owns only what lies in its directory, so take the first path that does.
	skills = ""
	for _, path := range append([]string{spec.Skills.Global}, spec.AlsoScans.Global...) {
		if skills = move(path); skills != "" {
			break
		}
	}
	return skills, move(spec.Agents.Global), nil
}

// agentCLI checks the executable an account runs instead of its Agent's. A name is looked
// up on PATH when a plugin command runs; a path must be absolute, since a relative one would
// depend on where Skillshare runs. Nothing goes through a shell, so it is one executable.
func agentCLI(cli string) (string, error) {
	if !strings.ContainsAny(cli, `/\`) {
		if strings.ContainsAny(cli, " \t") {
			return "", fmt.Errorf("cli %q must be one executable, without arguments", cli)
		}
		return cli, nil
	}
	if path := expandPath(cli); filepath.IsAbs(path) {
		return path, nil
	}
	return "", fmt.Errorf("cli %q must be a name found on PATH or an absolute path, which may start with ~", cli)
}

// expandAgentConfigDirs fills in the paths of targets that are another config directory
// of a built-in Agent, such as a second account. A path written in the config wins.
func (c *Config) expandAgentConfigDirs() error {
	names := make([]string, 0, len(c.Targets))
	for name := range c.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	dirs := map[string]string{}
	for _, name := range names {
		target := c.Targets[name]
		if target.Agent == "" && target.ConfigDir == "" && target.CLI == "" {
			continue
		}
		if target.Agent == "" || target.ConfigDir == "" {
			return fmt.Errorf("targets: %s: agent and config_dir go together, and cli needs both", name)
		}
		if target.CLI != "" {
			cli, err := agentCLI(target.CLI)
			if err != nil {
				return fmt.Errorf("targets: %s: %w", name, err)
			}
			target.CLI = cli
		}
		if _, builtin := globalSpec(name); builtin || name == "none" {
			return fmt.Errorf("targets: %s: that name is taken by a built-in target; choose another, such as %s-work", name, target.Agent)
		}
		target.ConfigDir = expandPath(target.ConfigDir)
		skills, agents, err := AgentConfigDirPaths(target.Agent, target.ConfigDir)
		if err != nil {
			return fmt.Errorf("targets: %s: %w", name, err)
		}
		key := filepath.Clean(target.ConfigDir)
		if other, taken := dirs[key]; taken {
			return fmt.Errorf("targets: %s and %s share config_dir %s", other, name, target.ConfigDir)
		}
		dirs[key] = name
		if skills != "" && target.SkillsConfig().Path == "" {
			target.EnsureSkills().Path, target.derivedSkills = skills, skills
		}
		if agents != "" && target.AgentsConfig().Path == "" {
			target.EnsureAgents().Path, target.derivedAgents = agents, agents
		}
		c.Targets[name] = target
	}
	return nil
}

// AddAgentConfigDirTarget adds a target that is another config directory of a built-in
// Agent and returns the skills path it syncs to. cli is optional; see TargetConfig.CLI.
func (c *Config) AddAgentConfigDirTarget(name, agent, dir, cli string) (string, error) {
	if _, exists := c.Targets[name]; exists {
		return "", fmt.Errorf("target '%s' already exists", name)
	}
	if c.Targets == nil {
		c.Targets = map[string]TargetConfig{}
	}
	c.Targets[name] = TargetConfig{Agent: agent, ConfigDir: dir, CLI: cli, defaultTargetNaming: c.TargetNaming}
	if err := c.expandAgentConfigDirs(); err != nil {
		delete(c.Targets, name)
		return "", err
	}
	target := c.Targets[name]
	return target.SkillsConfig().Path, nil
}

// withoutDerivedPaths drops the paths expandAgentConfigDirs filled in, so they keep
// following config_dir instead of being saved as written ones.
func (tc TargetConfig) withoutDerivedPaths() TargetConfig {
	strip := func(rc *ResourceTargetConfig, derived string) *ResourceTargetConfig {
		if rc == nil || derived == "" || rc.Path != derived {
			return rc
		}
		out := *rc
		if out.Path = ""; out.IsEmpty() {
			return nil
		}
		return &out
	}
	tc.Skills, tc.Agents = strip(tc.Skills, tc.derivedSkills), strip(tc.Agents, tc.derivedAgents)
	return tc
}
