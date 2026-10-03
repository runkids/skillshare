package mcp

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var grokServerName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// Change deliberately contains no native values, which may include credentials.
type Change struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	// Root is the mcp.projects folder this change belongs to, empty for a global one. The
	// path usually says, but Claude Code's per-project off list lives in the global file.
	Root string `json:"root,omitempty"`
	// Switch marks an entry that only turns a global server off for one project. Adding it
	// turns the server off there and removing it turns it back on; no server comes or goes.
	Switch  bool          `json:"switch,omitempty"`
	Action  string        `json:"action"`
	Message string        `json:"message,omitempty"`
	Fields  *FieldChanges `json:"fields,omitempty"`
}

// switchOnly reports an entry holding nothing but the switch renderDisabled writes. It reads
// the entry rather than the source, which has nothing left to say once the switch is removed.
func switchOnly(target string, entry map[string]any) bool {
	if strings.HasPrefix(target, claudeOffPrefix) {
		return true
	}
	if target == "pi" {
		// Pi's switch carries the global server's command or url; see renderDisabled.
		return len(entry) == 2 && entry["enabled"] == false && (entry["command"] != nil || entry["url"] != nil)
	}
	return len(entry) == 1 && (entry["enabled"] == false || entry["disabled"] == true)
}

// changeRoot is the mcp.projects folder a file plan writes for. Claude Code's off list
// sits in ~/.claude.json, outside every root, so its native target carries the root;
// in project mode that root is current, the scope itself, which reports as "".
func changeRoot(target, path, current string, roots []string) string {
	if root, ok := strings.CutPrefix(target, claudeOffPrefix); ok {
		if root == current {
			return ""
		}
		return root
	}
	for _, root := range roots {
		if strings.HasPrefix(path, root+string(filepath.Separator)) {
			return root
		}
	}
	return ""
}

// Plan is a redacted, optimistic-concurrency-protected preview.
type Plan struct {
	Revision   string `json:"revision"`
	SourcePath string `json:"sourcePath"`
	Blocked    bool   `json:"blocked"`
	// Notices are the source's, about settings it still has that no longer apply, and name
	// each Agent that cannot hold a part of a server's tool policy.
	Notices []string `json:"notices,omitempty"`
	// Migrates is set when applying the plan also saves its source without the settings
	// 0.23.0 retired, even when no Agent file changes.
	Migrates   bool     `json:"migrates,omitempty"`
	Changes    []Change `json:"changes"`
	files      []*filePlan
	state      ledger
	stateBytes []byte
	source     *Source
}

type ownership struct {
	Owner  string `json:"owner"`
	Target string `json:"target"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Hash   string `json:"hash"`
	// A nil Root is legacy ownership; an empty root records global scope.
	Root     *string           `json:"root,omitempty"`
	PiFields map[string]string `json:"piFields,omitempty"`
}

type ledger struct {
	Version int                  `json:"version"`
	Entries map[string]ownership `json:"entries"`
}

// PiBuiltinNotice is the plan's notice when it moves servers from pi-mcp-adapter or
// pi-mcp-extension to Pi's built-in MCP. Pi's MCP docs say an installed extension that
// registers /mcp replaces the built-in support; pi-mcp-extension, and pi-mcp-adapter
// before 3.0.0, read mcp.json too. The dashboard matches the text, so change both together.
const PiBuiltinNotice = "Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP"

// piExtensionEntry reports an entry Skillshare wrote for pi-mcp-adapter or pi-mcp-extension
// before 0.23.0: one in the adapter's own file, or one with the transport field only
// pi-mcp-extension read. The built-in entry never has it, and it is part of the hash, so an
// owned entry that still matches carries it only from Skillshare.
func piExtensionEntry(target, path string, entry map[string]any) bool {
	_, transport := entry["transport"]
	return target == "pi" && (path == piAdapterPath(path) || transport)
}

// orphanedMessage leads a conflict whose owning config is gone. The dashboard matches
// conflicts on how the message starts, so the path stays last and nothing else may.
const orphanedMessage = "left over from a Skillshare config that was removed; import it or explicitly replace this entry"

// ownerGone reports whether an owning config is missing, the one case where it can never
// release its entries itself and another config may take them over. Only a missing file
// counts: an unreadable one may sit on a drive that is not mounted, where the owner is
// still there. What the conflict says and what a resolution may do both rest on this, so
// they read it from here rather than each testing the path their own way.
func ownerGone(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

type filePlan struct {
	path, target  string
	before, after []byte
	exists        bool
	mode          os.FileMode
	section       string
	changes       map[string]map[string]any
}

// fileKey is one Agent file plus the native target that reads and edits it. Claude Code's
// per-project off list shares ~/.claude.json with the user-scope servers, so a path alone
// cannot say which of the two a plan means. Each key gets its own filePlan; apply rereads
// the file for every one of them, so two plans for one file fold in order.
type fileKey struct{ path, target string }

// ownershipKey is one entry in one file. Claude's off list shares ~/.claude.json with the
// user-scope servers a global config may own under the same name, so it keys apart.
func ownershipKey(target, path, name string) string {
	if strings.HasPrefix(target, claudeOffPrefix) {
		path = target + "\x00" + path
	}
	return digest([]byte(path + "\x00" + name))
}

func safeRead(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, 0600, nil
	}
	if err != nil {
		return nil, false, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, 0, fmt.Errorf("MCP file must be a regular file, not a symlink or directory: %s", path)
	}
	data, err := os.ReadFile(path)
	return data, true, info.Mode().Perm(), err
}

func (s *Service) loadLedger() (ledger, []byte, error) {
	state := ledger{Version: 1, Entries: map[string]ownership{}}
	// Only writes recover an interrupted write, under the lock. Until then, read
	// the ownership that recovery will record, byte for byte, so a preview's
	// revision stays valid across that recovery.
	pending, written, err := s.readPending()
	if err != nil {
		return state, nil, err
	}
	if written {
		data, err := json.MarshalIndent(pending.State, "", "  ")
		return pending.State, append(data, '\n'), err
	}
	data, exists, _, err := safeRead(s.statePath())
	if err != nil {
		return state, nil, err
	}
	if exists {
		if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Entries == nil {
			return state, nil, fmt.Errorf("MCP ownership state is invalid; restore a backup or explicitly import entries again")
		}
	}
	return state, data, nil
}

// forget drops this config's ownership of the servers a draft stops managing, in their own
// scope only: a global server's entries, or one project's. A plan then reads those Agent
// entries as the user's own and neither removes nor updates them.
func (s *Source) forget(state ledger, current string) {
	roots := sortedKeys(s.Projects)
	for key, owned := range state.Entries {
		if owned.Owner == s.ConfigPath && s.unmanaged[changeRoot(owned.Target, owned.Path, current, roots)+"\x00"+owned.Name] {
			delete(state.Entries, key)
		}
	}
}

// Preview reads source, ownership and native files without writing. Writes
// recover an interrupted operation first, while holding the lock.
func (s *Service) Preview() (*Plan, error) {
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	return s.previewSource(source)
}

func (s *Service) previewSource(source *Source) (*Plan, error) {
	return s.previewResolved(source, nil)
}

// render builds every native entry the source asks for, keyed by file and native target. A
// global source's projects land in the same map, so one plan, one revision and one ledger
// owner cover every root: separate plans would each read the others' entries as leftovers.
func (s *Service) render(source *Source) (map[fileKey]map[string]map[string]any, error) {
	desired, _, err := s.renderWithRoots(source)
	return desired, err
}

// renderWithRoots records each entry's actual scope, even when global and project
// outputs share a file. An empty root is global; older ledger records have no root.
func (s *Service) renderWithRoots(source *Source) (map[fileKey]map[string]map[string]any, map[string]string, error) {
	desired := map[fileKey]map[string]map[string]any{}
	roots := map[string]string{}
	if err := source.checkTargets(); err != nil {
		return nil, nil, err
	}
	s = s.withAccounts(source.Accounts)
	if s.ProjectRoot != "" && len(source.Projects) > 0 {
		return nil, nil, fmt.Errorf("mcp.projects belongs in the global config; this project already syncs its own mcp.servers")
	}
	servers := source.Servers
	if s.ProjectRoot != "" {
		// A project's own config cannot see the global one, so only the Agents' limits apply.
		servers = followingSwitches(servers, source.Targets, nil)
	}
	if err := s.renderScope(desired, roots, servers, source.Targets); err != nil {
		return nil, nil, err
	}
	for _, root := range sortedKeys(source.Projects) {
		project := source.Projects[root]
		scoped := *s
		scoped.ProjectRoot = root
		defaults := project.Targets
		if defaults == nil {
			defaults = source.Targets
		}
		if err := scoped.renderScope(desired, roots, followingSwitches(project.Servers, defaults, source), defaults); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", root, err)
		}
	}
	return desired, roots, nil
}

// SwitchTargets is where a switch-only entry that names no targets goes: the Agents among
// defaults that have a per-project switch and, when reached says where the global server of
// that name is written, that receive it. Never nil, so the result reads as a decision.
func SwitchTargets(server Server, defaults, reached []string) TargetList {
	targets := TargetList{}
	for _, target := range defaults {
		if _, err := renderDisabled(target, server); err == nil && (reached == nil || slices.Contains(reached, target)) {
			targets = append(targets, target)
		}
	}
	return targets
}

// followingSwitches gives each switch-only entry that names no targets the Agents where it has
// something to do: those the project uses, that have a per-project switch and, when global
// says where the server of that name goes, that receive it. The list is worked out on every
// plan rather than stored, so it cannot go stale when the project's targets change. An entry
// that names its targets is left alone, and an Agent without a switch there is still an error.
func followingSwitches(servers map[string]Server, defaults []string, global *Source) map[string]Server {
	out := maps.Clone(servers)
	for name, server := range servers {
		if !server.Disabled {
			continue
		}
		var reached []string
		if global != nil {
			if shared, ok := global.Servers[name]; ok {
				server.global = &shared
				if reached = shared.Targets; reached == nil {
					reached = global.Targets
				}
			}
		}
		if server.Targets != nil {
			out[name] = server
			continue
		}
		server.Targets = SwitchTargets(server, defaults, reached)
		// Claude Code's off list is per account, so an account that has the server gets the
		// switch too. Pi accounts all read the project's one .pi/mcp.json, which pi writes.
		if global != nil {
			for _, account := range sortedKeys(global.Accounts) {
				agent := global.Accounts[account].Agent
				if _, err := renderDisabled(agent, server); err == nil && agent == "claude" && slices.Contains(defaults, agent) && (reached == nil || slices.Contains(reached, account)) {
					server.Targets = append(server.Targets, account)
				}
			}
		}
		out[name] = server
	}
	return out
}

// renderScope adds one scope's servers: the global one, or a single project root.
func (s *Service) renderScope(desired map[fileKey]map[string]map[string]any, roots map[string]string, servers map[string]Server, defaults []string) error {
	for _, name := range sortedKeys(servers) {
		server := servers[name]
		// An explicit empty list is deliberate: the server stays in Skillshare and nothing is
		// written. An inherited one is more likely a forgotten default, so it is refused.
		selected := []string(server.Targets)
		if selected == nil {
			selected = defaults
			if len(selected) == 0 {
				return fmt.Errorf("MCP %s has no targets; select at least one Agent, or set targets to an empty list (--target none) to keep it in Skillshare only", name)
			}
		}
		for _, target := range selected {
			s, target := s.forTarget(target)
			if err := s.checkScope(name, target, server); err != nil {
				return err
			}
			path, native, err := s.destination(target, server)
			if err != nil {
				return err
			}
			entry, err := Render(target, server)
			if err != nil {
				return fmt.Errorf("%s / %s: %w", target, name, err)
			}
			if target == "goose" {
				entry["name"] = name
			}
			key := fileKey{path, native}
			if target == "pi" {
				for other := range desired[key] {
					if other != name && piServerKey(other) == piServerKey(name) {
						return fmt.Errorf("pi / %s: Pi reads %s and %s as one server and skips the second; rename one of them", name, other, name)
					}
				}
			}
			if desired[key] == nil {
				desired[key] = map[string]map[string]any{}
			}
			desired[key][name] = entry
			roots[ownershipKey(native, path, name)] = s.ProjectRoot
		}
	}
	if s.ProjectRoot != "" {
		if desired[fileKey{filepath.Join(s.ProjectRoot, ".mcp.json"), "claude"}] != nil && desired[fileKey{filepath.Join(s.ProjectRoot, ".github", "mcp.json"), "copilot"}] != nil {
			return fmt.Errorf("Claude and Copilot project MCP destinations overlap in precedence; use global mode for one client")
		}
	}
	return nil
}

// piServerKey is how Pi compares server names: since 0.99.2, - and _ are the same.
func piServerKey(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// forTarget resolves an account to its Agent, in a scope whose files are that account's.
func (s *Service) forTarget(target string) (*Service, string) {
	account, ok := s.accounts[target]
	if !ok {
		return s, target
	}
	scoped := *s
	scoped.ConfigDirs = maps.Clone(s.ConfigDirs)
	if scoped.ConfigDirs == nil {
		scoped.ConfigDirs = map[string]string{}
	}
	scoped.ConfigDirs[account.Agent] = account.Dir
	scoped.account = target
	return &scoped, account.Agent
}

// shownAs is the target a change reports: the account when the file is one's, so two
// accounts of one Agent stay apart in the plan and in conflict resolutions.
func (s *Service) shownAs(target, path string, accounts map[string]Account) string {
	shown := shownTarget(target)
	for _, name := range sortedKeys(accounts) {
		scoped := *s
		scoped.accounts, scoped.ProjectRoot = accounts, ""
		account, agent := scoped.forTarget(name)
		if file, err := account.nativePath(agent); err == nil && agent == shown && (file == path || agent == "pi" && piAdapterPath(file) == path) {
			return name
		}
	}
	return shown
}

// destination is the file one server lands in for one Agent, and the native target that
// reads and edits it. They differ from nativePath only for a switch-only Claude entry:
// Claude Code takes a whole entry from one scope, so a lone switch in .mcp.json would
// replace the server. Its per-project off list in the global file does the job instead.
func (s *Service) destination(target string, server Server) (string, string, error) {
	if target == "claude" && server.Disabled && s.ProjectRoot != "" {
		global := *s
		global.ProjectRoot = ""
		path, err := global.nativePath(target)
		return path, claudeOffPrefix + s.ProjectRoot, err
	}
	path, err := s.nativePath(target)
	return path, target, err
}

// ownedHomeResolves parks old homes, including removed accounts. Remembered
// project scopes still support cleanup after a project is removed from the source.
func (s *Service) ownedHomeResolves(owned ownership, source *Source) bool {
	agent := shownTarget(owned.Target)
	matches := func(scoped *Service) bool {
		path, target, err := scoped.destination(agent, Server{Disabled: strings.HasPrefix(owned.Target, claudeOffPrefix)})
		return err == nil && target == owned.Target && (path == owned.Path || agent == "pi" && piAdapterPath(path) == owned.Path)
	}
	roots := []string{s.ProjectRoot}
	if owned.Root != nil {
		roots = append(roots, *owned.Root)
	} else {
		roots = append(roots, sortedKeys(source.Projects)...)
		if root, ok := strings.CutPrefix(owned.Target, claudeOffPrefix); ok {
			roots = append(roots, root)
		}
	}
	for _, root := range roots {
		project := *s
		project.ProjectRoot = root
		if matches(&project) {
			return true
		}
		for _, name := range sortedKeys(source.Accounts) {
			account, target := project.forTarget(name)
			if target == agent && matches(account) {
				return true
			}
		}
	}
	return false
}

// claudeLocalServers names the servers of Claude Code's local scope for this project. They
// live in the global file and win, whole, over .mcp.json and the user scope.
func (s *Service) claudeLocalServers() map[string]any {
	if s.ProjectRoot == "" {
		return nil
	}
	global := *s
	global.ProjectRoot = ""
	path, err := global.nativePath("claude")
	if err != nil {
		return nil
	}
	data, _, _, err := safeRead(path)
	if err != nil {
		return nil
	}
	var document struct {
		Projects map[string]struct {
			McpServers map[string]any `json:"mcpServers"`
		} `json:"projects"`
	}
	_ = json.Unmarshal(data, &document)
	return document.Projects[s.ProjectRoot].McpServers
}

// From Claude Code's MCP docs. The withheld list is the names the docs give; they say "such as".
var (
	claudeReservedNames = []string{"workspace", "claude-in-chrome", "computer-use"}
	claudeWithheldEnv   = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "AWS_BEARER_TOKEN_BEDROCK", "HTTPS_PROXY", "NPM_TOKEN"}
)

// checkScope refuses what Render cannot see: limits that depend on the mode, the
// Agent's directory overrides or the server's name. The dashboard's preview runs it
// too, so it never shows a config that saving would then refuse.
func (s *Service) checkScope(name, target string, server Server) error {
	switch {
	case server.Disabled && s.ProjectRoot == "":
		return fmt.Errorf("MCP %s: disabled only applies in project mode, where it turns off a server from the Agent's global config; here, unselect the Agent instead", name)
	case target == "pi" && strings.Contains(name, "."):
		return fmt.Errorf("Pi built-in MCP %s: use letters, digits, underscores or hyphens", name)
	case target == "pi" && s.ProjectRoot != "" && server.PiOptions["auth"] != nil:
		return fmt.Errorf("Pi built-in MCP %s: Pi reads piOptions.auth only from its global mcp.json; define this server in global mode", name)
	case target == "grok" && (!grokServerName.MatchString(name) || strings.Contains(name, "__") || strings.HasSuffix(name, "_")):
		return fmt.Errorf("Grok MCP %s: use a name starting with a letter or underscore, containing only letters, digits, hyphens and single underscores, and not ending in underscore", name)
	case target == "claude" && slices.Contains(claudeReservedNames, name):
		return fmt.Errorf("Claude MCP %s: Claude Code reserves this name for a built-in server and skips the entry; choose another name", name)
	case target == "claude" && server.URL != "":
		// Claude Code reads its own and the cloud provider's credentials as empty in a remote
		// server's url and headers, so the server would get "Bearer " and answer 401.
		refs := []*Value{server.BearerToken}
		for _, v := range server.Headers {
			refs = append(refs, &v)
		}
		for _, v := range refs {
			if v != nil && slices.Contains(claudeWithheldEnv, v.FromEnv) {
				return fmt.Errorf("Claude MCP %s: Claude Code never sends %s to a remote server and reads it as empty; copy the value into a variable with a name of your own and reference that", name, v.FromEnv)
			}
		}
	case target == "kilocode" && s.ProjectRoot != "" && server.usesEnv():
		return fmt.Errorf("Kilo Code MCP %s: Kilo does not allow environment references in project config and ignores the whole file when it finds one; remove fromEnv here or define this server in global mode", name)
	}
	return nil
}

func (s *Service) previewResolved(source *Source, resolutions []Resolution) (*Plan, error) {
	s = s.withAccounts(source.Accounts)
	matched := map[string]bool{}
	for _, r := range resolutions {
		key := r.Target + "\x00" + r.Name
		_, exists := matched[key]
		_, account := source.Accounts[r.Target]
		if exists || (!validTarget(r.Target) && !account) || (r.Action != "replace" && r.Action != "adopt") {
			return nil, fmt.Errorf("invalid or duplicate MCP conflict resolution")
		}
		matched[key] = false
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.Path, Notices: slices.Concat(source.Notices, source.toolPolicyNotices()), Migrates: source.NeedsMigration(), Changes: []Change{}, source: source, state: state, stateBytes: stateBytes}
	for _, agent := range sortedKeys(accountEnv) {
		if _, account := s.configDir(agent); account != "" {
			global := *s
			global.ProjectRoot = ""
			path, err := global.nativePath(agent)
			if err != nil {
				return nil, err
			}
			p.Notices = append(p.Notices, fmt.Sprintf("%s points at the config_dir of target %s; %s syncs %s instead", accountEnv[agent], account, agent, path))
		}
	}
	desired, renderedRoots, err := s.renderWithRoots(source)
	if err != nil {
		return nil, err
	}
	// A file the source no longer writes still needs a plan, to remove what it left there.
	parked := map[string]ownership{}
	for _, key := range sortedKeys(state.Entries) {
		owned := state.Entries[key]
		if fk := (fileKey{owned.Path, owned.Target}); owned.Owner == source.ConfigPath && desired[fk][owned.Name] == nil {
			if !s.ownedHomeResolves(owned, source) {
				parked[key] = owned
				notice := fmt.Sprintf("%s: managed MCP entry %s in %s is left unchanged because the path no longer resolves to this target; sync where its home resolves to manage it", shownTarget(owned.Target), owned.Name, owned.Path)
				if owned.Root == nil && !strings.HasPrefix(owned.Target, claudeOffPrefix) {
					notice += "; this older ownership record has no saved scope; see the MCP reference for recovery"
				}
				p.Notices = append(p.Notices, notice)
				continue
			}
			if desired[fk] == nil {
				desired[fk] = map[string]map[string]any{}
			}
		}
	}
	// After the files are known, so stopping to manage a server keeps the revision of removing it.
	source.forget(state, s.ProjectRoot)
	maps.Copy(state.Entries, parked)
	proposal, _ := json.Marshal(struct {
		Servers     map[string]Server
		Targets     []string
		Resolutions []Resolution
		Projects    map[string]Project
	}{source.Servers, source.Targets, resolutions, source.Projects})
	revision := digest(source.configBytes) + digest(source.bytes) + digest(stateBytes) + digest(proposal)
	// Claude's local scope is per project, so look it up by the project file it shadows.
	local := map[string]map[string]any{}
	for _, root := range append(sortedKeys(source.Projects), s.ProjectRoot) {
		if root == "" {
			continue
		}
		scoped := *s
		scoped.ProjectRoot = root
		if path, err := scoped.nativePath("claude"); err == nil {
			local[path] = scoped.claudeLocalServers()
		}
	}
	projectRoots := sortedKeys(source.Projects)
	piMoves := source.piExtensionSettings
	keys := make([]fileKey, 0, len(desired))
	for key := range desired {
		keys = append(keys, key)
	}
	// Path first, then target, so ~/.claude.json writes its servers before the off list
	// that names them. Apply rereads the file per plan, so the later one sees the earlier.
	slices.SortFunc(keys, func(a, b fileKey) int {
		if a.path != b.path {
			return strings.Compare(a.path, b.path)
		}
		return strings.Compare(a.target, b.target)
	})
	for _, fk := range keys {
		path, target := fk.path, fk.target
		data, exists, mode, err := safeRead(path)
		if err != nil {
			return nil, err
		}
		native, err := ParseNative(target, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		shown := s.shownAs(target, path, source.Accounts)
		f := &filePlan{path: path, target: target, before: data, exists: exists, mode: mode, section: sectionDigest(native), changes: map[string]map[string]any{}}
		revision += path + target + f.section
		names := map[string]bool{}
		for name := range desired[fk] {
			names[name] = true
		}
		for key, owned := range state.Entries {
			if _, parked := parked[key]; !parked && owned.Owner == source.ConfigPath && owned.Path == path && owned.Target == target {
				names[owned.Name] = true
			}
		}
		for _, name := range sortedKeys(names) {
			key := ownershipKey(target, path, name)
			root := renderedRoots[key]
			owned, managed := state.Entries[key]
			current := native.Entries[name]
			currentHash := entryHash(managedEntry(target, current))
			want := desired[fk][name]
			wantHash := entryHash(managedEntry(target, want))
			prune := map[string]bool{}
			pruneConflict := false
			// Fields this config wrote earlier and no longer sets go, unless Pi changed them since.
			if target == "pi" && want != nil && managed && owned.Owner == source.ConfigPath {
				for field, hash := range owned.PiFields {
					if _, set := want[field]; set {
						continue
					}
					if value, exists := current[field]; exists {
						if piFieldHash(value) != hash {
							pruneConflict = true
						} else {
							prune[field] = true
						}
					}
				}
			}
			for _, resolution := range resolutions {
				if resolution.Target != shown || resolution.Name != name {
					continue
				}
				matched[resolution.Target+"\x00"+name] = true
				// An owner that still exists must release the entry itself. One that was
				// moved or deleted never can, so an explicit resolution may take over.
				if managed && owned.Owner != source.ConfigPath && !ownerGone(owned.Owner) {
					break
				}
				// adopt claims only an entry that already matches; anything else
				// stays a conflict for an explicit replace.
				if resolution.Action == "adopt" && currentHash != wantHash {
					continue
				}
				owned = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: currentHash}
				managed = true
				p.state.Entries[key] = owned
			}
			change := Change{Target: shown, Path: path, Name: name, Root: changeRoot(target, path, s.ProjectRoot, projectRoots)}
			if change.Switch = switchOnly(target, want); want == nil {
				change.Switch = switchOnly(target, current)
			}
			switch {
			case target == "pi" && current != nil && piOverride(current) && (change.Root != "" || s.ProjectRoot != "") &&
				(!managed || currentHash != owned.Hash) && !(managed && owned.Owner != source.ConfigPath && !ownerGone(owned.Owner)):
				// Skillshare never writes a connection-less Pi entry, so Pi's /mcp wrote this one,
				// possibly over an entry a config synced. It has no server for import to take, so
				// this comes before every conflict that offers import. A live owner keeps its own
				// message: only that config can release the entry. Replace records the entry as
				// owned at its current hash, which lets the cases below write over it.
				change.Action, change.Message = "conflict", "existing entry is a Pi project override of a global server; replace it, or remove the override with /mcp in Pi"
			case pruneConflict:
				change.Action, change.Message = "conflict", "Pi setting changed since sync; import it before removing the cleared setting"
			case managed && owned.Owner != source.ConfigPath:
				change.Action, change.Message = "conflict", "managed by another Skillshare config: "+owned.Owner
				// A config that is gone can never release the entry, so an explicit resolution
				// is the only way out and the message has to offer it. Saying it is managed
				// sends the user looking for a file that is not there. Refs: #288.
				if ownerGone(owned.Owner) {
					change.Message = orphanedMessage + ": " + owned.Owner
				}
			case currentHash == wantHash && managed && want != nil && native.cramped(name):
				// The content is right but it is all on one line. Sync owns this entry, so it
				// writes it again, laid out; the person pressing Sync expects a file they can read.
				change.Action, change.Message = "update", "same settings, laid out one field per line"
				f.changes[name] = withAgentFields(target, current, want)
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: wantHash}
			case currentHash == wantHash && managed && (agentFieldsChanged(target, current, want) || len(prune) > 0):
				change.Action = "update"
				f.changes[name] = withAgentFields(target, current, want)
			case currentHash == wantHash && !managed && want != nil && current != nil && !change.Switch:
				// The Agent already has this server, e.g. the one it was imported from, ticked
				// after the import. Sync takes it over without writing it, so the preview shows
				// the takeover and unticking the Agent later removes the entry. Refs: #303.
				// A switch the person turned off in the Agent stays theirs.
				change.Action = "adopt"
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: currentHash}
			case currentHash == wantHash:
				// Already as desired, e.g. after pulling a teammate's change or
				// moving a project. Refresh an owned baseline; an unmanaged entry
				// is only claimed by the adopt change above.
				if want == nil {
					delete(p.state.Entries, key)
					continue
				}
				change.Action = "unchanged"
				if managed {
					p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: currentHash}
				}
			case managed && currentHash != owned.Hash:
				change.Action, change.Message = "conflict", "Agent configuration changed; import it or explicitly replace this entry"
			case !managed && current != nil:
				change.Action, change.Message = "conflict", "existing entry is not managed; import it to explicitly adopt it"
			case want == nil:
				change.Action = "remove"
				f.changes[name] = nil
				delete(p.state.Entries, key)
			default:
				change.Action = "add"
				if managed {
					change.Action = "update"
				}
				f.changes[name] = withAgentFields(target, current, want)
				p.state.Entries[key] = ownership{Owner: source.ConfigPath, Target: target, Path: path, Name: name, Hash: wantHash}
			}
			if updated := f.changes[name]; updated != nil {
				for field := range prune {
					delete(updated, field)
				}
			}
			if want != nil && change.Action != "conflict" {
				if own, ok := p.state.Entries[key]; ok && own.Owner == source.ConfigPath {
					own.Root = &root
					if target == "pi" {
						own.PiFields = piOwnedFields(want)
					}
					p.state.Entries[key] = own
				}
			}
			if (change.Action == "remove" || change.Action == "update") && piExtensionEntry(target, path, current) {
				piMoves = true
			}
			change.Fields = changedFieldNames(current, f.changes[name], change.Action)
			if change.Action == "conflict" {
				p.Blocked = true
			} else if target == "claude" && want != nil && local[path][name] != nil {
				change.Message = "a local scope server of the same name in ~/.claude.json overrides this one in this project; remove it with: claude mcp remove " + name + " -s local"
			}
			p.Changes = append(p.Changes, change)
		}
		f.after, err = native.Edit(f.changes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		p.files = append(p.files, f)
	}
	p.Revision = digest([]byte(revision))
	if piMoves {
		p.Notices = append(p.Notices, PiBuiltinNotice)
	}
	for _, found := range matched {
		if !found {
			return nil, fmt.Errorf("conflict resolution does not refer to a selected MCP entry")
		}
	}
	return p, nil
}
