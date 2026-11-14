package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/profile"
	"skillshare/internal/skillserve"
	"skillshare/internal/ui"
)

const mcpServeUsage = "skillshare mcp serve [--target <name>] [--profile-token NAME=ENV_VAR ...] [--http <addr> [--tls-cert <file> --tls-key <file>] [--http-allow-insecure] | --check] [-g|-p]"

// mcpServeTokenEnv holds the bearer token HTTP clients must send.
const mcpServeTokenEnv = "SKILLSHARE_MCP_TOKEN"

// cmdMCPServe serves the managed skills over MCP (SEP-2640). It writes only
// skill files that mcp.serve.edit makes editable. stdout carries the
// protocol, so diagnostics go to stderr.
func cmdMCPServe(args []string) error {
	var target, addr, certFile, keyFile string
	var profileTokens []string
	valueFlags := map[string]*string{"--target": &target, "--http": &addr, "--tls-cert": &certFile, "--tls-key": &keyFile}
	mode, rest, err := parseModeArgs(args, append(slices.Collect(maps.Keys(valueFlags)), "--profile-token")...)
	if err != nil {
		return err
	}
	check, allowInsecure := false, false
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if a == "--http-allow-insecure" {
			allowInsecure = true
			continue
		}
		if a == "--check" {
			check = true
			continue
		}
		if a == "--profile-token" {
			if i+1 == len(rest) {
				return fmt.Errorf("--profile-token requires NAME=ENV_VAR")
			}
			i++
			profileTokens = append(profileTokens, rest[i])
			continue
		}
		v, ok := valueFlags[a]
		if !ok {
			return fmt.Errorf("unknown mcp serve argument %q; usage: %s", a, mcpServeUsage)
		}
		if i+1 == len(rest) {
			return fmt.Errorf("%s requires a value", a)
		}
		i++
		*v = rest[i]
	}
	if check && addr != "" {
		return fmt.Errorf("--check starts no server; drop --http")
	}
	if (certFile == "") != (keyFile == "") {
		return fmt.Errorf("--tls-cert and --tls-key go together")
	}
	if certFile != "" && addr == "" {
		return fmt.Errorf("--tls-cert and --tls-key need --http")
	}

	if allowInsecure && addr == "" {
		return fmt.Errorf("--http-allow-insecure need --http")
	}

	// A gateway chooses the working directory, so only -p selects project mode.
	var handler http.Handler
	if len(profileTokens) > 0 {
		if addr == "" && !check {
			return fmt.Errorf("--profile-token requires --http or --check")
		}
		if os.Getenv(mcpServeTokenEnv) != "" {
			return fmt.Errorf("--profile-token cannot be combined with %s", mcpServeTokenEnv)
		}
		bindings, err := parseMCPServeProfileBindings(profileTokens, os.LookupEnv)
		if err != nil {
			return err
		}
		builders, edit, err := loadMCPServeProfiles(mode, target)
		if err != nil {
			return err
		}
		if check {
			if err := skillserve.ValidateProfileBindings(builders, bindings); err != nil {
				return err
			}
			return printMCPServeProfilesCheck(builders, edit)
		}
		if !isLoopbackAddr(addr) && certFile == "" && !allowInsecure {
			return fmt.Errorf("refusing hosted credentials over plain HTTP on %s: pass --tls-cert and --tls-key, bind loopback behind a TLS proxy, or pass --http-allow-insecure for a trusted private network", addr)
		}
		handler, err = skillserve.NewHostedHandler(builders, bindings, skillserve.Options{Version: version, Warn: os.Stderr, Refresh: 5 * time.Second, Edit: edit})
		if err != nil {
			return err
		}
		return serveMCPHTTP(handler, addr, certFile, keyFile, allowInsecure)
	}
	b := &skillserve.Builder{Target: target}
	var targets map[string]config.TargetConfig
	var edit *skillserve.EditOptions
	if mode == modeProject {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		rt, err := loadProjectRuntime(cwd)
		if err != nil {
			return err
		}
		b.Source, b.Walk, targets = rt.sourcePath, rt.skillsWalk(), rt.targets
		b.ResolveProfile = func() (profile.Selector, error) {
			cfg, err := config.LoadProjectReadOnly(cwd)
			if err != nil {
				return profile.Selector{}, err
			}
			return cfg.ProfileSelector(), nil
		}
		if edit, err = mcpServeEdit(rt.config.MCP, rt.config.Audit.BlockThreshold, config.ProjectConfigPath(cwd)); err != nil {
			return err
		}
		if edit != nil {
			edit.ProjectRoot = cwd
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		b.Source, b.Walk, targets = cfg.EffectiveSkillsSource(), cfg.SkillsWalk(), cfg.Targets
		path := config.ReadOnlyConfigPath()
		b.ResolveProfile = func() (profile.Selector, error) {
			current, err := config.LoadReadOnlyPath(path)
			if err != nil {
				return profile.Selector{}, err
			}
			return current.ProfileSelector(), nil
		}
		if edit, err = mcpServeEdit(cfg.MCP, cfg.Audit.BlockThreshold, config.ConfigPath()); err != nil {
			return err
		}
	}
	if target != "" {
		tc, ok := targets[target]
		if !ok {
			return fmt.Errorf("unknown target %q", target)
		}
		sc := tc.SkillsConfig()
		if !sc.IsEnabled() {
			return fmt.Errorf("target %q has skills disabled", target)
		}
		b.Skills = &sc
		if tc.ProjectRoot() != "" && mode != modeProject {
			b.ResolveProfile = nil
		}
	}
	if check {
		return printMCPServeCheck(b, edit)
	}

	// Off loopback, require authentication and TLS unless explicitly opted out.
	token := os.Getenv(mcpServeTokenEnv)
	if addr != "" && !isLoopbackAddr(addr) {
		if token == "" {
			return fmt.Errorf("refusing to serve on %s without authentication: set %s, or bind a loopback address such as 127.0.0.1:8765", addr, mcpServeTokenEnv)
		}
		if certFile == "" && !allowInsecure {
			return fmt.Errorf("refusing to send %s over plain HTTP on %s: pass --tls-cert and --tls-key, bind a loopback address behind a TLS proxy, or pass --http-allow-insecure for a trusted private network", mcpServeTokenEnv, addr)
		}
	}

	srv, err := skillserve.NewServer(b, skillserve.Options{Version: version, Warn: os.Stderr, Refresh: 5 * time.Second, Edit: edit})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if addr == "" {
		// A client stopping the server with a signal is a normal exit, as for --http.
		if err := srv.Run(ctx, &mcp.StdioTransport{}); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	}
	if idx, err := srv.Index(); err == nil {
		for _, s := range idx.Skipped {
			fmt.Fprintf(os.Stderr, "warning: %s left out of the Executor index: %s\n", s.Path, s.Reason)
		}
		if idx.Err != nil {
			fmt.Fprintln(os.Stderr, "warning: "+idx.Err.Error())
		}
	}
	return serveMCPHTTP(skillserve.HTTPHandler(srv, token), addr, certFile, keyFile, allowInsecure)
}

// mcpServeEdit reads mcp.serve.edit. It returns nil when editing is off.
func mcpServeEdit(m *config.MCPConfig, threshold, cfgPath string) (*skillserve.EditOptions, error) {
	skills, ttl, err := m.ServeEdit()
	if err != nil || skills == nil {
		return nil, err
	}
	return &skillserve.EditOptions{
		Skills: skills, PreviewTTL: ttl, Threshold: threshold,
		Log: func(file string, entry oplog.Entry) {
			oplog.WriteWithLimit(cfgPath, file, entry, logMaxEntries()) //nolint:errcheck
		},
	}, nil
}

// printMCPServeCheck builds the catalog the server would serve and lists the
// skills it would skip, those clients may edit, and those the Executor index
// leaves out, without starting a server.
func printMCPServeCheck(b *skillserve.Builder, edit *skillserve.EditOptions) error {
	start := time.Now()
	c, err := b.Build()
	if err != nil {
		return err
	}
	idx := c.Index()
	labels := []string{"Editable", "Executor"}
	for _, s := range slices.Concat(c.Skipped, idx.Skipped) {
		labels = append(labels, s.Path)
	}
	width := ui.RowWidth(labels...)
	for _, s := range c.Skipped {
		ui.Row(ui.MarkWarn, s.Path, s.Reason, width)
	}
	for _, s := range idx.Skipped {
		ui.Row(ui.MarkWarn, s.Path, "left out of the Executor index: "+s.Reason, width)
	}
	if idx.Err != nil {
		ui.Row(ui.MarkFail, "Executor", idx.Err.Error(), width)
	} else {
		ui.Row(ui.MarkNone, "Executor", fmt.Sprintf("%d of %d in %s (%s, %s)", idx.Skills, len(c.Skills), skillserve.IndexPath+"index.json", plural(idx.Files, "file"), formatBytes(idx.Bytes)), width)
	}
	if edit != nil {
		editable, err := b.Editable(c, edit)
		if err != nil {
			return err
		}
		ui.Row(ui.MarkNone, "Editable", fmt.Sprintf("%d of %d (mcp.serve.edit.skills)", len(editable), len(c.Skills)), width)
	}
	text, mark := plural(len(c.Skills), "skill")+" served", ui.MarkOK
	if len(c.Skipped) > 0 {
		text, mark = text+fmt.Sprintf(", %d skipped", len(c.Skipped)), ui.MarkWarn
	}
	ui.Done(mark, text, time.Since(start))
	return nil
}
