package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/skillserve"
	"skillshare/internal/ui"
)

const mcpServeUsage = "skillshare mcp serve [--target <name>] [--http <addr> [--tls-cert <file> --tls-key <file>] [--http-allow-insecure] | --check] [-g|-p]"

// mcpServeTokenEnv holds the bearer token HTTP clients must send.
const mcpServeTokenEnv = "SKILLSHARE_MCP_TOKEN"

// cmdMCPServe serves the managed skills over MCP (SEP-2640). It writes only
// skill files that mcp.serve.edit makes editable. stdout carries the
// protocol, so diagnostics go to stderr.
func cmdMCPServe(args []string) error {
	var target, addr, certFile, keyFile string
	valueFlags := map[string]*string{"--target": &target, "--http": &addr, "--tls-cert": &certFile, "--tls-key": &keyFile}
	mode, rest, err := parseModeArgs(args, slices.Collect(maps.Keys(valueFlags))...)
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
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Handler: skillserve.HTTPHandler(srv, token), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		httpSrv.Close()
	}()
	scheme := "https"
	if certFile == "" {
		scheme = "http"
	}
	if allowInsecure && certFile == "" && !isLoopbackAddr(addr) {
		fmt.Fprintln(os.Stderr, "warning: --http-allow-insecure sends the bearer token over plain HTTP; use only on a trusted private network")
	}
	fmt.Fprintf(os.Stderr, "Serving skills over MCP at %s://%s/\n", scheme, ln.Addr())
	fmt.Fprintf(os.Stderr, "Executor skill index at %s://%s%sindex.json\n", scheme, ln.Addr(), skillserve.IndexPath)
	if idx, err := srv.Index(); err == nil {
		for _, s := range idx.Skipped {
			fmt.Fprintf(os.Stderr, "warning: %s left out of the Executor index: %s\n", s.Path, s.Reason)
		}
		if idx.Err != nil {
			fmt.Fprintln(os.Stderr, "warning: "+idx.Err.Error())
		}
	}
	if certFile == "" {
		err = httpSrv.Serve(ln)
	} else {
		err = httpSrv.ServeTLS(ln, certFile, keyFile)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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

// isLoopbackAddr reports whether a listen address only accepts local
// connections. An empty host listens on every interface.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
