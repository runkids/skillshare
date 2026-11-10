package hooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

// CodexTrust is one Codex hook position that sync --trust looked at.
type CodexTrust struct {
	Target string `json:"target"`
	Entry  string `json:"entry,omitempty"`
	Key    string `json:"key"`
	// Action is trusted (recorded now), unchanged (already trusted) or cleared (trust
	// Skillshare recorded for a position its hook no longer holds).
	Action string `json:"action"`
	// Disabled reports a hook switched off in Codex, which sync --trust leaves off.
	Disabled bool `json:"disabled,omitempty"`
}

// CodexHook is the part of a Codex app-server hooks/list entry trust needs.
type CodexHook struct {
	Key         string `json:"key"`
	SourcePath  string `json:"sourcePath"`
	Enabled     bool   `json:"enabled"`
	CurrentHash string `json:"currentHash"`
	TrustStatus string `json:"trustStatus"`
}

// CodexAppServer is a running `codex app-server` for one Codex home. Codex computes
// each hook's hash and writes config.toml itself, exactly as its /hooks review does.
type CodexAppServer interface {
	ListHooks() ([]CodexHook, error)
	// SetTrust writes hooks.state.<key>.trusted_hash for each key; "" clears it.
	SetTrust(hashes map[string]string) error
	Close() error
}

// codexTrustState remembers the trust sync --trust recorded, by hooks.json path, so
// a later run clears only what it wrote.
type codexTrustState struct {
	Version int                       `json:"version"`
	Homes   map[string]codexTrustHome `json:"homes"`
}

type codexTrustHome struct {
	Owner  string            `json:"owner"`
	Path   string            `json:"path"`
	Target string            `json:"target"`
	Keys   map[string]string `json:"keys"`
}

func (s *Service) codexTrustPath() string { return filepath.Join(s.stateDir(), "codex-trust.json") }

// TrustCodex records Codex trust for the hooks Skillshare owns in each global Codex
// home, after a sync. Only trusted_hash changes; enabled and the rest of config.toml
// stay as they are. It never runs a hook.
func (s *Service) TrustCodex() ([]CodexTrust, error) {
	if s.ProjectRoot != "" {
		return nil, fmt.Errorf("--trust records trust in global Codex homes only; run it with -g")
	}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	state, _, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	owner, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("resolve hooks config owner: %w", err)
	}
	saved := codexTrustState{Version: 2, Homes: map[string]codexTrustHome{}}
	if data, exists, _, err := safeRead(s.codexTrustPath()); err != nil {
		return nil, err
	} else if exists && (json.Unmarshal(data, &saved) != nil || saved.Homes == nil) {
		return nil, fmt.Errorf("Codex trust state is invalid: %s", s.codexTrustPath())
	}
	saved.Version = 2
	homes := map[string]string{}
	perFile := map[string]map[string]record{}
	owned := map[string]map[string]record{}
	parked := map[string]map[string]record{}
	for key, r := range state.Records {
		if r.Root != "" || s.agentOf(r.Target) != "codex" {
			continue
		}
		file := canonicalPath(r.Path)
		if perFile[file] == nil {
			perFile[file] = map[string]record{}
		}
		perFile[file][key] = r
		if r.Owner != owner {
			continue
		}
		sc, _ := s.forTarget(r.Target)
		if path, err := sc.nativePath("codex"); err != nil || !samePath(path, r.Path) {
			if parked[file] == nil {
				parked[file] = map[string]record{}
			}
			parked[file][key] = r
			continue // A parked record's home moved or is missing.
		}
		homes[r.Path] = r.Target
		if owned[file] == nil {
			owned[file] = map[string]record{}
		}
		owned[file][key] = r
	}
	for _, home := range saved.Homes {
		if home.Owner == owner {
			if _, ok := homes[home.Path]; !ok {
				homes[home.Path] = home.Target
			}
		}
	}
	out := []CodexTrust{}
	for _, path := range sortedKeys(homes) {
		if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
			continue // A missing account home is skipped, as sync skips it.
		}
		homeKey := codexTrustHomeKey(owner, path)
		file := canonicalPath(path)
		results, keys, err := s.trustCodexHome(homes[path], path, perFile[file], owned[file], parked[file], saved.Homes[homeKey].Keys)
		out = append(out, results...)
		if err != nil {
			return out, fmt.Errorf("%s: %w", homes[path], err)
		}
		if len(keys) == 0 {
			delete(saved.Homes, homeKey)
		} else {
			saved.Homes[homeKey] = codexTrustHome{Owner: owner, Path: path, Target: homes[path], Keys: keys}
		}
		if err := writeJSONFile(s.codexTrustPath(), saved); err != nil {
			return out, err
		}
	}
	// Version 1 did not store config ownership. Reconcile those entries only as
	// stale trust to clear; never use them to select a hook that should be trusted.
	legacyChanged := false
	for _, key := range sortedKeys(saved.Homes) {
		home := saved.Homes[key]
		if home.Owner != "" {
			continue
		}
		path := key
		if home.Path != "" {
			path = home.Path
		}
		if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
			continue
		}
		results, _, err := s.trustCodexHome(home.Target, path, nil, nil, nil, home.Keys)
		out = append(out, results...)
		if err != nil {
			return out, fmt.Errorf("%s: %w", home.Target, err)
		}
		current, err := codexTrustedHashes(filepath.Join(filepath.Dir(path), "config.toml"))
		if err != nil {
			return out, err
		}
		for hookKey, hash := range home.Keys {
			if current[hookKey] != hash {
				delete(home.Keys, hookKey)
			}
		}
		if len(home.Keys) == 0 {
			delete(saved.Homes, key)
		} else {
			saved.Homes[key] = home
		}
		legacyChanged = true
	}
	if legacyChanged {
		if err := writeJSONFile(s.codexTrustPath(), saved); err != nil {
			return out, err
		}
	}
	return out, nil
}

func codexTrustHomeKey(owner, path string) string { return owner + "\x00" + path }

// trustCodexHome locates groups against all records for one hooks.json, then trusts
// only the active owner's groups. It clears trust recorded earlier for positions they
// left and keeps receipts for still-owned registrations parked at this path.
func (s *Service) trustCodexHome(target, path string, records, owned, parked map[string]record, written map[string]string) ([]CodexTrust, map[string]string, error) {
	positions := map[string]string{}
	parkedPositions := map[string]bool{}
	data, exists, _, err := safeRead(path)
	if err != nil {
		return nil, nil, err
	}
	if exists && len(records) > 0 {
		doc, err := parseNative("codex", data)
		if err != nil {
			return nil, nil, err
		}
		for key, index := range locate(doc, records) {
			position := codexEventKey(records[key].Event) + ":" + strconv.Itoa(index)
			if _, ok := owned[key]; ok {
				positions[position] = records[key].Entry
			}
			if _, ok := parked[key]; ok {
				parkedPositions[position] = true
			}
		}
		// Duplicate counts can make locate reject an otherwise matching parked
		// registration. Keep its receipt only at positions with the same group hash.
		for _, r := range parked {
			for index, item := range doc.events[r.Event] {
				if elementHash(item) == r.Hash {
					parkedPositions[codexEventKey(r.Event)+":"+strconv.Itoa(index)] = true
				}
			}
		}
	}
	start := s.CodexAppServer
	if start == nil {
		start = startCodexAppServer
	}
	server, err := start(filepath.Dir(path))
	if err != nil {
		return nil, nil, err
	}
	defer server.Close()
	listed, err := server.ListHooks()
	if err != nil {
		return nil, nil, err
	}
	current, currentExists, _, err := safeRead(path)
	if err != nil {
		return nil, nil, err
	}
	if exists != currentExists || !bytes.Equal(data, current) {
		return nil, nil, fmt.Errorf("%s changed while Codex hooks were being listed; run hooks sync --trust again", path)
	}
	results := []CodexTrust{}
	edits, keep, here := map[string]string{}, map[string]string{}, map[string]CodexHook{}
	for _, h := range listed {
		if !samePath(h.SourcePath, path) {
			continue
		}
		here[h.Key] = h
		position := codexPosition(h.Key)
		entry, ok := positions[position]
		if !ok {
			if parkedPositions[position] && written[h.Key] == h.CurrentHash {
				keep[h.Key] = h.CurrentHash
			}
			continue
		}
		result := CodexTrust{Target: target, Entry: entry, Key: h.Key, Action: "unchanged", Disabled: !h.Enabled}
		switch {
		case h.TrustStatus == "untrusted" || h.TrustStatus == "modified":
			edits[h.Key], keep[h.Key], result.Action = h.CurrentHash, h.CurrentHash, "trusted"
		case h.TrustStatus != "trusted":
			continue // Managed hooks are trusted by policy, not config.toml.
		case written[h.Key] == h.CurrentHash:
			keep[h.Key] = h.CurrentHash
		}
		results = append(results, result)
	}
	if stale := staleKeys(written, keep); len(stale) > 0 {
		current, err := codexTrustedHashes(filepath.Join(filepath.Dir(path), "config.toml"))
		if err != nil {
			return results, written, err
		}
		for _, key := range stale {
			// Trust someone changed since, or that an identical hook now uses, stays.
			if current[key] == written[key] && here[key].CurrentHash != written[key] {
				edits[key] = ""
				results = append(results, CodexTrust{Target: target, Key: key, Action: "cleared"})
			}
		}
	}
	if len(edits) > 0 {
		if err := server.SetTrust(edits); err != nil {
			unchanged := results[:0]
			for _, result := range results {
				if result.Action == "unchanged" {
					unchanged = append(unchanged, result)
				}
			}
			return unchanged, written, err
		}
	}
	return results, keep, nil
}

func staleKeys(written, keep map[string]string) []string {
	var out []string
	for _, key := range sortedKeys(written) {
		if _, ok := keep[key]; !ok {
			out = append(out, key)
		}
	}
	return out
}

// codexEventKey is Codex's snake_case event label for a hooks.json event name.
func codexEventKey(event string) string {
	var b strings.Builder
	for i, r := range event {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// codexPosition is "<event>:<group>" from a hook key "<source>:<event>:<group>:<handler>";
// every handler of a group Skillshare owns is its own.
func codexPosition(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) < 4 {
		return ""
	}
	return parts[len(parts)-3] + ":" + parts[len(parts)-2]
}

// codexTrustedHashes reads hooks.state.<key>.trusted_hash from a Codex config.toml.
func codexTrustedHashes(path string) (map[string]string, error) {
	data, exists, _, err := safeRead(path)
	if err != nil || !exists {
		return map[string]string{}, err
	}
	var doc struct {
		Hooks struct {
			State map[string]struct {
				TrustedHash string `toml:"trusted_hash"`
			} `toml:"state"`
		} `toml:"hooks"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := map[string]string{}
	for key, state := range doc.Hooks.State {
		out[key] = state.TrustedHash
	}
	return out, nil
}

// codexRPCTimeout bounds each app-server request.
const codexRPCTimeout = 30 * time.Second

type codexProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan []byte
	next  int
}

// startCodexAppServer runs `codex app-server` with CODEX_HOME set to home.
func startCodexAppServer(home string) (CodexAppServer, error) {
	bin, err := exec.LookPath("codex")
	if err != nil {
		return nil, errors.New("codex is not on PATH")
	}
	cmd := exec.Command(bin, "app-server")
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "CODEX_HOME="+home)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &codexProcess{cmd: cmd, stdin: stdin, lines: make(chan []byte, 16)}
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for scanner.Scan() {
			p.lines <- append([]byte(nil), scanner.Bytes()...)
		}
		close(p.lines)
	}()
	if _, err := p.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "skillshare", "version": "0"}}); err != nil {
		p.Close()
		return nil, err
	}
	if err := p.send(map[string]any{"jsonrpc": "2.0", "method": "initialized"}); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *codexProcess) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = p.stdin.Write(append(data, '\n'))
	return err
}

func (p *codexProcess) call(method string, params any) (json.RawMessage, error) {
	p.next++
	if err := p.send(map[string]any{"jsonrpc": "2.0", "id": p.next, "method": method, "params": params}); err != nil {
		return nil, fmt.Errorf("codex app-server %s: %w", method, err)
	}
	timeout := time.After(codexRPCTimeout)
	for {
		select {
		case line, ok := <-p.lines:
			if !ok {
				return nil, fmt.Errorf("codex app-server exited during %s", method)
			}
			var msg struct {
				ID     *int            `json:"id"`
				Method string          `json:"method"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(line, &msg) != nil || msg.Method != "" || msg.ID == nil || *msg.ID != p.next {
				continue // Notifications and server requests carry a method; responses never do.
			}
			if msg.Error != nil {
				return nil, fmt.Errorf("codex app-server %s: %s", method, msg.Error.Message)
			}
			return msg.Result, nil
		case <-timeout:
			return nil, fmt.Errorf("codex app-server did not answer %s within %s", method, codexRPCTimeout)
		}
	}
}

func (p *codexProcess) ListHooks() ([]CodexHook, error) {
	raw, err := p.call("hooks/list", map[string]any{"cwds": []string{p.cmd.Dir}})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data []struct {
			Hooks []CodexHook `json:"hooks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("codex app-server hooks/list: %w", err)
	}
	var out []CodexHook
	for _, entry := range res.Data {
		out = append(out, entry.Hooks...)
	}
	return out, nil
}

func (p *codexProcess) SetTrust(hashes map[string]string) error {
	edits := []map[string]any{}
	for _, key := range sortedKeys(hashes) {
		var value any
		if hashes[key] != "" {
			value = hashes[key]
		}
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(key)
		edits = append(edits, map[string]any{"keyPath": `hooks.state."` + escaped + `".trusted_hash`, "value": value, "mergeStrategy": "replace"})
	}
	_, err := p.call("config/batchWrite", map[string]any{"edits": edits})
	return err
}

func (p *codexProcess) Close() error {
	p.stdin.Close()
	_ = p.cmd.Process.Kill()
	go func() {
		for range p.lines { // Unblock the reader so it sees EOF and exits.
		}
	}()
	_ = p.cmd.Wait()
	return nil
}
