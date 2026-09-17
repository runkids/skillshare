package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	versioncheck "skillshare/internal/version"
)

type uiUpgradeResult struct {
	Updated       bool   `json:"updated"`
	DevMode       bool   `json:"devMode"`
	LatestVersion string `json:"latestVersion,omitempty"`
	Output        string `json:"output,omitempty"`
}

var (
	runUIUpgrade         = defaultRunUIUpgrade
	startUIRestartHelper = defaultStartUIRestartHelper
	processExit          = os.Exit
)

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result, err := runUIUpgrade()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"ok":            true,
		"updated":       result.Updated,
		"devMode":       result.DevMode,
		"latestVersion": result.LatestVersion,
		"output":        result.Output,
	})
}

func defaultRunUIUpgrade() (uiUpgradeResult, error) {
	if versioncheck.Version == "" || versioncheck.Version == "dev" {
		return uiUpgradeResult{Updated: true, DevMode: true, LatestVersion: "dev-ui-flow", Output: "DEV mode update simulated"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	exe, err := os.Executable()
	if err != nil {
		return uiUpgradeResult{}, err
	}
	cmd := exec.CommandContext(ctx, exe, "upgrade", "--force")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out != "" {
			return uiUpgradeResult{Output: out}, fmt.Errorf("upgrade failed: %w\n%s", err, out)
		}
		return uiUpgradeResult{}, fmt.Errorf("upgrade failed: %w", err)
	}
	return uiUpgradeResult{Updated: true, Output: strings.TrimSpace(buf.String())}, nil
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !restartRequestAllowed(r, s.addr) {
		writeError(w, http.StatusForbidden, "restart is only available from the local Skillshare UI")
		return
	}
	body := struct {
		ClearCache *bool `json:"clearCache"`
	}{}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}
	clearCache := true
	if body.ClearCache != nil {
		clearCache = *body.ClearCache
	}
	args, err := s.uiRestartHelperArgs(clearCache)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := startUIRestartHelper(args, s.projectRoot); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "restarting": true})
	go func() {
		time.Sleep(250 * time.Millisecond)
		processExit(0)
	}()
}

func (s *Server) uiRestartHelperArgs(clearCache bool) ([]string, error) {
	host, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		return nil, fmt.Errorf("restart unavailable for server address %q: %w", s.addr, err)
	}
	if host == "" {
		host = "0.0.0.0"
	}
	args := []string{"__ui-restart", "--no-open", "--host", host, "--port", port}
	if s.basePath != "" {
		args = append(args, "--base-path", s.basePath)
	}
	if clearCache {
		args = append(args, "--clear-cache")
	}
	if s.IsProjectMode() {
		args = append(args, "--project")
	} else {
		args = append(args, "--global")
	}
	return args, nil
}

func defaultStartUIRestartHelper(args []string, dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func restartRequestAllowed(r *http.Request, serverAddr string) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	originOK := false
	if origin := r.Header.Get("Origin"); origin != "" {
		originHost, err := hostFromURL(origin)
		if err != nil || !sameHost(originHost, r.Host) {
			return false
		}
		originOK = true
	}
	if requestFromLoopback(r) {
		return true
	}
	if !serverBindAcceptsRemote(serverAddr) {
		return false
	}
	fetchSite := r.Header.Get("Sec-Fetch-Site")
	return originOK || fetchSite == "same-origin" || fetchSite == "same-site"
}

// MCP settings can hold stdio commands an agent runs. DNS rebinding makes Origin
// match an attacker Host on any bind (0.0.0.0 in Docker included), but it always
// needs a domain name, so the Host must be localhost or an IP literal.
func mcpRequestAllowed(r *http.Request, serverAddr string) bool {
	if !restartRequestAllowed(r, serverAddr) {
		return false
	}
	host, _ := splitHostPortDefault(r.Host)
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil
}

func (s *Server) requireLocalMCP(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mcpRequestAllowed(r, s.addr) {
			writeError(w, http.StatusForbidden, "MCP settings are available only when the dashboard is opened by localhost or an IP address, not a domain name")
			return
		}
		next(w, r)
	}
}

func requestFromLoopback(r *http.Request) bool {
	if r.RemoteAddr == "" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serverBindAcceptsRemote(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

func hostFromURL(raw string) (string, error) {
	withoutScheme := raw
	if index := strings.Index(withoutScheme, "://"); index >= 0 {
		withoutScheme = withoutScheme[index+3:]
	}
	if slash := strings.Index(withoutScheme, "/"); slash >= 0 {
		withoutScheme = withoutScheme[:slash]
	}
	if withoutScheme == "" {
		return "", fmt.Errorf("empty origin host")
	}
	return withoutScheme, nil
}

func sameHost(a, b string) bool {
	ah, ap := splitHostPortDefault(a)
	bh, bp := splitHostPortDefault(b)
	return strings.EqualFold(ah, bh) && ap == bp
}

func splitHostPortDefault(value string) (string, string) {
	host, port, err := net.SplitHostPort(value)
	if err == nil {
		return host, port
	}
	if strings.Count(value, ":") == 1 {
		parts := strings.SplitN(value, ":", 2)
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return parts[0], parts[1]
		}
	}
	return value, ""
}
