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
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// CodexTrust is one Codex hook Skillshare owns, as sync --trust sees it.
type CodexTrust struct {
	Target string `json:"target"`
	Entry  string `json:"entry,omitempty"`
	Key    string `json:"key"`
	// Command is what the hook runs, shown for review before trust is recorded.
	Command string `json:"command,omitempty"`
	// Action is needed (not trusted yet, or changed since it was), trusted (recorded
	// now) or unchanged (already trusted).
	Action string `json:"action"`
	// Disabled reports a hook switched off in Codex, which sync --trust leaves off.
	Disabled bool `json:"disabled,omitempty"`
}

// CodexTrustPlan is what sync --trust looked at. Revision names the hooks that need
// trust and their hashes, so trust is recorded only for what was shown.
type CodexTrustPlan struct {
	Hooks    []CodexTrust `json:"hooks"`
	Revision string       `json:"revision"`
	// Failed are the homes Codex could not list. Their hooks are not in Hooks, so
	// they are never trusted; the other homes go on without them.
	Failed []CodexTrustFailure `json:"failed,omitempty"`
}

// CodexTrustFailure is one Codex home that could not be listed, and why.
type CodexTrustFailure struct {
	Target string `json:"target"`
	Error  string `json:"error"`
}

// Err reports the homes Codex could not list, or nil.
func (p *CodexTrustPlan) Err() error {
	var errs []error
	for _, f := range p.Failed {
		errs = append(errs, fmt.Errorf("%s: %s", f.Target, f.Error))
	}
	return errors.Join(errs...)
}

// Needed are the hooks that would be newly trusted.
func (p *CodexTrustPlan) Needed() []CodexTrust {
	var out []CodexTrust
	for _, h := range p.Hooks {
		if h.Action == "needed" {
			out = append(out, h)
		}
	}
	return out
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
	// SetTrust writes hooks.state.<key>.trusted_hash for each key.
	SetTrust(hashes map[string]string) error
	Close() error
}

// codexHome is one physical hooks.json: every record written to it, and the active
// config's own among them.
type codexHome struct {
	target, path   string
	records, owned map[string]record
}

// PreviewCodexTrust lists the Codex hooks Skillshare owns in each global Codex home
// and which of them need trust. It writes nothing and never runs a hook.
func (s *Service) PreviewCodexTrust() (*CodexTrustPlan, error) { return s.codexTrust("", false) }

// TrustCodex records Codex trust for the hooks a preview with this revision listed
// as needed. Only trusted_hash changes; enabled and the rest of config.toml stay as
// they are, and trust recorded earlier for a hook that moved or left stays too: it
// matches only a hook with the same event, matcher and handler at that position.
func (s *Service) TrustCodex(revision string) (*CodexTrustPlan, error) {
	return s.codexTrust(revision, true)
}

func (s *Service) codexTrust(revision string, apply bool) (*CodexTrustPlan, error) {
	if s.ProjectRoot != "" {
		return nil, fmt.Errorf("--trust records trust in global Codex homes only; run it with -g")
	}
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	// A preview lets go before it asks Codex, which can take a while: a hooks.json that
	// changes meanwhile fails the listing or the revision, so nothing is trusted unseen.
	state, _, err := s.loadLedger()
	if !apply || err != nil {
		lock.Unlock()
	} else {
		defer lock.Unlock()
	}
	if err != nil {
		return nil, err
	}
	owner, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("resolve hooks config owner: %w", err)
	}
	// Two targets that name one hooks.json, by a symlink or not, share one Codex run.
	homes := map[string]*codexHome{}
	for key, r := range state.Records {
		if r.Root != "" || s.agentOf(r.Target) != "codex" {
			continue
		}
		file := canonicalCodexHooksPath(r.Path)
		h := homes[file]
		if h == nil {
			h = &codexHome{records: map[string]record{}, owned: map[string]record{}}
			homes[file] = h
		}
		h.records[key] = r
		if r.Owner != owner {
			continue
		}
		sc, _ := s.forTarget(r.Target)
		path, err := sc.nativePath("codex")
		if err != nil || canonicalCodexHooksPath(path) != file {
			continue // The record's home moved or is missing; there is nothing to trust there.
		}
		if h.target == "" || r.Target < h.target {
			h.target, h.path = r.Target, path
		}
		h.owned[key] = r
	}
	plan := &CodexTrustPlan{Hooks: []CodexTrust{}}
	type write struct {
		target string
		server CodexAppServer
		edits  map[string]string
	}
	var writes []write
	var servers []CodexAppServer
	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()
	var needed []string
	listed := 0
	for _, file := range sortedKeys(homes) {
		h := homes[file]
		if info, err := os.Stat(filepath.Dir(h.path)); len(h.owned) == 0 || err != nil || !info.IsDir() {
			continue // A missing account home is skipped, as sync skips it.
		}
		server, hooks, edits, err := s.listCodexHome(h)
		if server != nil {
			servers = append(servers, server)
		}
		if err != nil {
			// One home Codex cannot list does not stop the others.
			plan.Failed = append(plan.Failed, CodexTrustFailure{Target: h.target, Error: err.Error()})
			continue
		}
		listed++
		writes = append(writes, write{h.target, server, edits})
		plan.Hooks = append(plan.Hooks, hooks...)
		for key, hash := range edits {
			needed = append(needed, key+"\x00"+hash)
		}
	}
	if listed == 0 && len(plan.Failed) > 0 {
		return plan, plan.Err()
	}
	slices.Sort(needed)
	plan.Revision = digest([]byte(strings.Join(needed, "\n")))
	if !apply {
		return plan, nil
	}
	if revision != plan.Revision {
		return plan, fmt.Errorf("Codex hooks changed since they were listed: %w", ErrStaleRevision)
	}
	// One home that fails does not stop the others, and the error says which is which.
	var done []string
	var failed []error
	if err := plan.Err(); err != nil {
		failed = append(failed, err)
	}
	for _, w := range writes {
		if len(w.edits) == 0 {
			continue
		}
		if err := w.server.SetTrust(w.edits); err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", w.target, err))
			continue
		}
		done = append(done, w.target)
		for i, h := range plan.Hooks {
			if _, ok := w.edits[h.Key]; ok {
				plan.Hooks[i].Action = "trusted"
			}
		}
	}
	if len(failed) > 0 && len(done) > 0 {
		return plan, fmt.Errorf("Codex trust was recorded for %s, but not for every home: %w", strings.Join(done, ", "), errors.Join(failed...))
	}
	return plan, errors.Join(failed...)
}

// canonicalCodexHooksPath resolves the home's symlinks even when hooks.json is absent.
func canonicalCodexHooksPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Join(canonicalPath(filepath.Dir(path)), filepath.Base(path))
}

// listCodexHome asks Codex about one hooks.json. It locates groups against every
// record for the file, then reports only the active owner's; edits are the hashes
// that would make the needed ones trusted. The server is returned open for the write.
func (s *Service) listCodexHome(h *codexHome) (CodexAppServer, []CodexTrust, map[string]string, error) {
	data, exists, _, err := safeRead(h.path)
	if err != nil || !exists {
		return nil, nil, nil, err
	}
	doc, err := parseNative("codex", data)
	if err != nil {
		return nil, nil, nil, err
	}
	type group struct {
		entry string
		item  any
	}
	groups := map[string]group{}
	for key, index := range locate(doc, h.records) {
		if r, ok := h.owned[key]; ok {
			groups[codexEventKey(r.Event)+":"+strconv.Itoa(index)] = group{r.Entry, doc.events[r.Event][index]}
		}
	}
	if len(groups) == 0 {
		return nil, nil, nil, nil
	}
	start := s.CodexAppServer
	if start == nil {
		start = startCodexAppServer
	}
	dir := filepath.Dir(h.path)
	server, err := start(dir, s.codexDefaultHome(dir))
	if err != nil {
		return nil, nil, nil, err
	}
	listed, err := server.ListHooks()
	if err != nil {
		return server, nil, nil, err
	}
	current, currentExists, _, err := safeRead(h.path)
	if err != nil {
		return server, nil, nil, err
	}
	if !currentExists || !bytes.Equal(data, current) {
		return server, nil, nil, fmt.Errorf("%s changed while Codex hooks were being listed; run hooks sync --trust again", h.path)
	}
	var results []CodexTrust
	edits := map[string]string{}
	for _, hook := range listed {
		if !samePath(hook.SourcePath, h.path) {
			continue
		}
		position, handler := codexPosition(hook.Key)
		g, ok := groups[position]
		if !ok {
			continue
		}
		result := CodexTrust{Target: h.target, Entry: g.entry, Key: hook.Key, Command: codexHandlerCommand(g.item, handler), Action: "unchanged", Disabled: !hook.Enabled}
		switch hook.TrustStatus {
		case "untrusted", "modified":
			result.Action, edits[hook.Key] = "needed", hook.CurrentHash
		case "trusted":
		default:
			continue // Managed hooks are trusted by policy, not config.toml.
		}
		results = append(results, result)
	}
	return server, results, edits, nil
}

// codexDefaultHome reports whether the user's Codex finds dir as ~/.codex rather than
// through CODEX_HOME. Codex resolves CODEX_HOME's symlinks but not ~/.codex's, and the
// trust key holds that path, so the app-server must find the home the same way.
func (s *Service) codexDefaultHome(dir string) bool {
	if env := s.ConfigDirs["codex"]; env != "" && samePath(env, dir) {
		return false
	}
	home, err := os.UserHomeDir()
	return err == nil && filepath.Clean(dir) == filepath.Join(home, ".codex")
}

// codexHandlerCommand is the command of one handler of a hooks.json group.
func codexHandlerCommand(item any, handler int) string {
	group, _ := item.(map[string]any)
	handlers, _ := group["hooks"].([]any)
	if handler < 0 || handler >= len(handlers) {
		return ""
	}
	h, _ := handlers[handler].(map[string]any)
	command, _ := h["command"].(string)
	return command
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

// codexPosition splits a hook key "<source>:<event>:<group>:<handler>" into
// "<event>:<group>" and the handler index; every handler of a group Skillshare owns
// is its own.
func codexPosition(key string) (string, int) {
	parts := strings.Split(key, ":")
	if len(parts) < 4 {
		return "", -1
	}
	handler, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", -1
	}
	return parts[len(parts)-3] + ":" + parts[len(parts)-2], handler
}

// codexRPCTimeout bounds each app-server request.
const codexRPCTimeout = 30 * time.Second

type codexProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan []byte
	next  int
}

// startCodexAppServer runs `codex app-server` for one Codex home: with CODEX_HOME set
// to it, or for the default home with CODEX_HOME unset so Codex finds ~/.codex itself.
func startCodexAppServer(home string, defaultHome bool) (CodexAppServer, error) {
	bin, err := exec.LookPath("codex")
	if err != nil {
		return nil, errors.New("codex is not on PATH")
	}
	cmd := exec.Command(bin, "app-server")
	cmd.Dir = home
	cmd.Env = slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, "CODEX_HOME=") })
	if !defaultHome {
		cmd.Env = append(cmd.Env, "CODEX_HOME="+home)
	}
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
	return parseCodexHooksList(raw)
}

// parseCodexHooksList reads a hooks/list result. Codex answers a home it cannot load,
// such as one with an invalid config.toml, with no hooks and the reason in errors;
// that is a failed listing, not a home without hooks.
func parseCodexHooksList(raw json.RawMessage) ([]CodexHook, error) {
	var res struct {
		Data []struct {
			Hooks  []CodexHook `json:"hooks"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("codex app-server hooks/list: %w", err)
	}
	var out []CodexHook
	var problems []string
	for _, entry := range res.Data {
		out = append(out, entry.Hooks...)
		for _, e := range entry.Errors {
			problems = append(problems, e.Message)
		}
	}
	if len(out) == 0 && len(problems) > 0 {
		return nil, fmt.Errorf("Codex could not load its hooks: %s", strings.Join(problems, "; "))
	}
	return out, nil
}

func (p *codexProcess) SetTrust(hashes map[string]string) error {
	edits := []map[string]any{}
	for _, key := range sortedKeys(hashes) {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(key)
		edits = append(edits, map[string]any{"keyPath": `hooks.state."` + escaped + `".trusted_hash`, "value": hashes[key], "mergeStrategy": "replace"})
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
