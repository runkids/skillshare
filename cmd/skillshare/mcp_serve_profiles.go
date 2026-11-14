package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/skillserve"
)

var mcpTokenEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseMCPServeProfileBindings(values []string, lookup func(string) (string, bool)) ([]skillserve.ProfileBinding, error) {
	var bindings []skillserve.ProfileBinding
	names, tokens := map[string]bool{}, map[[32]byte]bool{}
	for _, value := range values {
		name, env, ok := strings.Cut(value, "=")
		if !ok || profile.ValidateName(name) != nil || !mcpTokenEnvName.MatchString(env) {
			return nil, fmt.Errorf("--profile-token requires NAME=ENV_VAR with a valid profile name and environment variable name")
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate --profile-token for profile %q", name)
		}
		token, ok := lookup(env)
		if !ok || token == "" {
			return nil, fmt.Errorf("profile %q: environment variable %s is absent or empty", name, env)
		}
		hash := sha256.Sum256([]byte(token))
		if tokens[hash] {
			return nil, fmt.Errorf("profile %q: environment variable %s resolves to a duplicate credential", name, env)
		}
		names[name], tokens[hash] = true, true
		bindings = append(bindings, skillserve.ProfileBinding{Profile: name, Token: token})
	}
	return bindings, nil
}

// loadMCPServeProfiles reads shared definitions and resolves explicit names;
// local active state never participates and no runtime migration or sync runs.
func loadMCPServeProfiles(mode runMode, target string) (map[string]*skillserve.Builder, *skillserve.EditOptions, error) {
	b := skillserve.Builder{Target: target}
	var catalog profile.Catalog
	var targets map[string]config.TargetConfig
	var edit *skillserve.EditOptions
	var path string
	if mode == modeProject {
		root, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		cfg, err := config.LoadProjectReadOnly(root)
		if err != nil {
			return nil, nil, err
		}
		targets, err = config.ResolveProjectTargets(root, cfg)
		if err != nil {
			return nil, nil, err
		}
		b.Source = cfg.EffectiveSkillsSource(root)
		b.Walk = config.SkillsWalk(cfg.FollowSourceLinks, b.Source, targets)
		path = config.ProjectConfigPath(root)
		catalog = profile.Parse(&cfg.Profiles)
		edit, err = mcpServeEdit(cfg.MCP, cfg.Audit.BlockThreshold, path)
		if err != nil {
			return nil, nil, err
		}
		if edit != nil {
			edit.ProjectRoot = root
		}
	} else {
		cfg, err := config.LoadReadOnly()
		if err != nil {
			return nil, nil, err
		}
		b.Source, b.Walk, targets = cfg.EffectiveSkillsSource(), cfg.SkillsWalk(), cfg.Targets
		path = config.ReadOnlyConfigPath()
		catalog = profile.Parse(&cfg.Profiles)
		edit, err = mcpServeEdit(cfg.MCP, cfg.Audit.BlockThreshold, path)
		if err != nil {
			return nil, nil, err
		}
	}
	if catalog.Reason != "" {
		return nil, nil, fmt.Errorf("profiles: %s", catalog.Reason)
	}
	if target != "" {
		tc, ok := targets[target]
		if !ok {
			return nil, nil, fmt.Errorf("unknown target %q", target)
		}
		sc := tc.SkillsConfig()
		if !sc.IsEnabled() {
			return nil, nil, fmt.Errorf("target %q has skills disabled", target)
		}
		b.Skills = &sc
	}
	builders := make(map[string]*skillserve.Builder, len(catalog.Definitions))
	for name := range catalog.Definitions {
		selector := profile.Resolve(catalog, profile.State{Version: 1, Active: name}, path, nil)
		if _, err := selector.Selects(""); err != nil {
			return nil, nil, err
		}
		copy := b
		copy.Profile = &selector
		builders[name] = &copy
	}
	return builders, edit, nil
}

func printMCPServeProfilesCheck(builders map[string]*skillserve.Builder, edit *skillserve.EditOptions) error {
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "Profile %s:\n", name)
		if err := printMCPServeCheck(builders[name], edit); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
	}
	return nil
}
