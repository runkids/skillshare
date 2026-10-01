package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Native state backups include directory modes, empty directories and Git
// metadata. Reject links/special files rather than silently losing recovery data.
func codexStateDigest(root string) (string, error) {
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return hash([]byte("absent")), nil
	} else if err != nil {
		return "", err
	}
	h := sha256.New()
	var size int64
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), info.Mode())
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("native Codex state contains links or special files; resolve them in Codex before updating")
		}
		count++
		size += info.Size()
		if count > 20000 || size > 100*1024*1024 {
			return fmt.Errorf("native Codex recovery state exceeds 20,000 files or 100 MiB")
		}
		fmt.Fprintf(h, "%d\x00", info.Size())
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(h, f)
		closeErr := f.Close()
		return errors.Join(err, closeErr)
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func copyCodexState(root, dest string) error {
	before, err := codexStateDigest(root)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return nil
	}
	type directory struct {
		path string
		mode fs.FileMode
	}
	var dirs []directory
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		to := filepath.Join(dest, rel)
		if info.IsDir() {
			dirs = append(dirs, directory{to, info.Mode().Perm()})
			return os.MkdirAll(to, 0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("native Codex state changed while backing up")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, 0600); err != nil {
			return err
		}
		return os.Chmod(to, info.Mode().Perm())
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i].path, dirs[i].mode); err != nil {
			return err
		}
	}
	after, err := codexStateDigest(root)
	if err != nil {
		return err
	}
	copied, err := codexStateDigest(dest)
	if err != nil {
		return err
	}
	if before != after || before != copied {
		return fmt.Errorf("native Codex state changed while backing up; preview again")
	}
	return nil
}

// Replace only a still-expected tree. Keep the backup until verification finishes.
func restoreCodexState(home, path, backup, expected string) error {
	if err := codexPlainPath(home, path); err != nil {
		return err
	}
	current, err := codexStateDigest(path)
	if err != nil {
		return err
	}
	if current != expected {
		return fmt.Errorf("native Codex state changed during recovery")
	}
	desired, err := codexStateDigest(backup)
	if err != nil {
		return err
	}
	if desired == current {
		return nil
	}
	parent := filepath.Dir(path)
	stage, err := os.MkdirTemp(parent, ".skillshare-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := copyCodexState(backup, filepath.Join(stage, "tree")); err != nil {
		return err
	}
	current, err = codexStateDigest(path)
	if err != nil || current != expected {
		return fmt.Errorf("native Codex state changed during recovery")
	}
	hadOld := false
	if _, err := os.Lstat(path); err == nil {
		if err := os.Rename(path, filepath.Join(stage, "old")); err != nil {
			return err
		}
		hadOld = true
	}
	if _, err := os.Lstat(filepath.Join(stage, "tree")); err == nil {
		if err := os.Rename(filepath.Join(stage, "tree"), path); err != nil {
			if hadOld {
				_ = os.Rename(filepath.Join(stage, "old"), path)
			}
			return err
		}
	}
	actual, err := codexStateDigest(path)
	if err != nil {
		return err
	}
	if actual != desired {
		return fmt.Errorf("native Codex state recovery verification failed")
	}
	return nil
}

// Predict only Codex's selected enabled-token write. The syntax tree gives the
// original byte range, including dotted keys, without rewriting
// comments, whitespace, unrelated entries or private native configuration.
func codexEnabledConfig(data []byte, id string) ([]byte, error) {
	var cfg codexNativeConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid native Codex configuration")
	}
	setting, ok := cfg.Plugins[id]
	if !ok || setting.Enabled == nil {
		return nil, fmt.Errorf("native Codex enabled state is not explicit")
	}
	var parser unstable.Parser
	parser.Reset(data)
	var table []string
	var token unstable.Range
	found := false
	inlineSelected := false
	var visit func(*unstable.Node, []string, bool)
	visit = func(n *unstable.Node, prefix []string, inline bool) {
		key := slices.Clone(prefix)
		it := n.Key()
		for it.Next() {
			key = append(key, string(it.Node().Data))
		}
		v := n.Value()
		if slices.Equal(key, []string{"plugins", id, "enabled"}) && v.Kind == unstable.Bool {
			token = parser.Range(v.Data)
			found = true
			inlineSelected = inline
		}
		if v.Kind == unstable.InlineTable {
			children := v.Children()
			for children.Next() {
				visit(children.Node(), key, true)
			}
		}
	}
	for parser.NextExpression() {
		n := parser.Expression()
		switch n.Kind {
		case unstable.Table, unstable.ArrayTable:
			table = nil
			it := n.Key()
			for it.Next() {
				table = append(table, string(it.Node().Data))
			}
		case unstable.KeyValue:
			visit(n, table, false)
		}
	}
	if inlineSelected {
		return nil, fmt.Errorf("Codex rewrites inline plugin tables; use ordinary or dotted plugin tables in native config before updating")
	}
	if parser.Error() != nil || !found {
		return nil, fmt.Errorf("native Codex enabled token cannot be safely preserved")
	}
	if *setting.Enabled {
		return bytes.Clone(data), nil
	}
	if string(data[token.Offset:token.Offset+token.Length]) != "false" {
		return nil, fmt.Errorf("native Codex enabled token cannot be safely preserved")
	}
	result := append(bytes.Clone(data[:token.Offset]), []byte("true")...)
	return append(result, data[token.Offset+token.Length:]...), nil
}

func restoreCodexConfig(home string, original, expected []byte) error {
	path := filepath.Join(home, "config.toml")
	if err := codexPlainPath(home, path); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Equal(current, original) {
		return nil
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("native Codex configuration changed concurrently; preserve the external edit and inspect recovery")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(home, ".skillshare-config-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		_ = f.Close()
		return err
	}
	_, writeErr := f.Write(original)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	current, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(current, expected) {
		return fmt.Errorf("native Codex configuration changed during recovery")
	}
	return os.Rename(f.Name(), path)
}

func verifyCodexSource(root string, u *CodexUpdate) error {
	content, err := codexContentDigest(filepath.Join(root, filepath.FromSlash(u.PluginPath)))
	if err != nil {
		return err
	}
	if content != u.ContentDigest {
		return fmt.Errorf("Codex plugin source changed since preview; preview again")
	}
	if u.CatalogDigest != "" {
		catalog, err := codexCatalogDigest(root)
		if err != nil {
			return err
		}
		if catalog != u.CatalogDigest {
			return fmt.Errorf("Codex marketplace catalog changed since preview; preview again")
		}
	}
	return nil
}

func verifyCodexMarketplaceCommit(ctx context.Context, root, expected string) error {
	commit, err := runCommand(ctx, root, []string{"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}, "git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(commit)) != expected {
		return fmt.Errorf("Codex marketplace ref advanced since preview; preview again")
	}
	return nil
}

func (s *Service) verifyCodexInstalled(ctx context.Context, target, id, version string, enabled bool) error {
	h := s.host(ctx, target)
	if h.Error != "" {
		return fmt.Errorf("Codex update verification failed: %s", h.Error)
	}
	for _, item := range h.Installed {
		if item.ID == id && item.Installed && item.EnabledKnown && item.Version == version && item.Enabled == enabled {
			return nil
		}
	}
	return fmt.Errorf("Codex installed version or enabled state did not match the expected result")
}

func (s *Service) applyCodexUpdate(ctx context.Context, c Change, b Binding) (resultErr error) {
	if c.CodexUpdate == nil {
		return fmt.Errorf("preview the Codex update before applying")
	}
	plan := *c.CodexUpdate
	u := &plan
	home, err := s.codexHome(c.Target)
	if err != nil {
		return err
	}
	lockPath := filepath.Join(home, ".skillshare-plugin-update.lock")
	if err := codexPlainPath(home, lockPath); err != nil {
		return err
	}
	lock := flock.New(lockPath)
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil || !locked {
		return fmt.Errorf("cannot lock the selected Codex configuration: %w", err)
	}
	defer lock.Unlock()
	original, _, err := readCodexConfig(home)
	if err != nil {
		return err
	}
	if hash(original) != u.ConfigDigest {
		return fmt.Errorf("Codex configuration changed since preview")
	}
	expected, err := codexEnabledConfig(original, c.ID)
	if err != nil {
		return err
	}
	name, market, _ := strings.Cut(c.ID, "@")
	cache := filepath.Join(home, "plugins/cache", market)
	if err := codexPlainPath(home, cache); err != nil {
		return err
	}
	digest, err := codexStateDigest(cache)
	if err != nil {
		return err
	}
	if digest != u.CacheDigest {
		return fmt.Errorf("Codex installed cache changed since preview")
	}
	root, err := s.codexMarketplace(ctx, c.Target, c.ID)
	if err != nil {
		return err
	}
	if root != u.MarketplaceRoot {
		return fmt.Errorf("Codex marketplace registration changed since preview")
	}
	if u.Refresh {
		digest, err := codexStateDigest(root)
		if err != nil {
			return err
		}
		if digest != u.MarketplaceDigest {
			return fmt.Errorf("Codex marketplace snapshot changed since preview")
		}
	}
	if err := s.verifyCodexInstalled(ctx, c.Target, c.ID, u.OldVersion, u.Enabled); err != nil {
		return err
	}
	if b.Source != "" {
		root, err = s.materialize(ctx, b, c.Target)
		if err != nil {
			return err
		}
		u.PluginPath = filepath.ToSlash(filepath.Join("content", u.PluginPath))
	}
	source := root
	cleanup := func() {}
	if b.Source == "" {
		source, _, cleanup, err = acquireCodexSource(ctx, u.Source, u.Commit, u.Refresh)
		if err != nil {
			return err
		}
	}
	defer cleanup()
	if err := verifyCodexSource(source, u); err != nil {
		return err
	}
	// Private catalogue retains installation/authentication policies for the native
	// client. No force, trust or authentication bypass flags are passed.
	backup, err := os.MkdirTemp(home, ".skillshare-plugin-recovery-")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(backup)
		}
	}()
	if err := copyTree(source, filepath.Join(backup, "reviewed")); err != nil {
		return err
	}
	if err := verifyCodexSource(filepath.Join(backup, "reviewed"), u); err != nil {
		return err
	}
	if err := copyCodexState(cache, filepath.Join(backup, "cache")); err != nil {
		return err
	}
	if u.Refresh {
		if err := copyCodexState(root, filepath.Join(backup, "marketplace")); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(backup, "config.toml"), original, 0600); err != nil {
		return err
	}
	// Verification/source acquisition can take time. Backups must still match
	// the reviewed state, and the live files must match again before mutation.
	for _, check := range []struct{ path, digest string }{{filepath.Join(backup, "cache"), u.CacheDigest}, {cache, u.CacheDigest}} {
		actual, err := codexStateDigest(check.path)
		if err != nil {
			return err
		}
		if actual != check.digest {
			return fmt.Errorf("Codex cache changed during preflight; preview again")
		}
	}
	if u.Refresh {
		for _, path := range []string{filepath.Join(backup, "marketplace"), root} {
			actual, err := codexStateDigest(path)
			if err != nil {
				return err
			}
			if actual != u.MarketplaceDigest {
				return fmt.Errorf("Codex marketplace changed during preflight; preview again")
			}
		}
	}
	currentConfig, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil || !bytes.Equal(currentConfig, original) {
		return fmt.Errorf("Codex configuration changed during preflight; preview again")
	}
	mutated := false
	var cacheAfter, rootAfter string
	statesAfter := map[string]string{}
	capture := func() error {
		var err error
		if err := codexPlainPath(home, cache); err != nil {
			return err
		}
		cacheAfter, err = codexStateDigest(cache)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(cache)
		if err != nil {
			return err
		}
		old, err := os.ReadDir(filepath.Join(backup, "cache"))
		if err != nil {
			return err
		}
		names := map[string]bool{}
		for _, e := range entries {
			names[e.Name()] = true
		}
		for _, e := range old {
			names[e.Name()] = true
		}
		statesAfter = map[string]string{}
		for name := range names {
			statesAfter[name], err = codexStateDigest(filepath.Join(cache, name))
			if err != nil {
				return err
			}
		}
		stable, err := codexStateDigest(cache)
		if err != nil {
			return err
		}
		if stable != cacheAfter {
			return fmt.Errorf("native Codex cache changed while capturing the result")
		}
		if u.Refresh {
			if err := codexPlainPath(home, root); err != nil {
				return err
			}
			rootAfter, err = codexStateDigest(root)
		}
		return err
	}
	defer func() {
		if !mutated || resultErr == nil {
			return
		}
		recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
		defer cancel()
		var recovery []error
		if u.Refresh {
			recovery = append(recovery, restoreCodexState(home, root, filepath.Join(backup, "marketplace"), rootAfter))
		}
		cacheErr := restoreCodexState(home, cache, filepath.Join(backup, "cache"), cacheAfter)
		if cacheErr != nil && statesAfter[name] != "" {
			// A concurrent sibling edit must survive. Recover the selected working
			// install separately when its own captured state is still unchanged.
			recovery = append(recovery, restoreCodexState(home, filepath.Join(cache, name), filepath.Join(backup, "cache", name), statesAfter[name]))
		}
		recovery = append(recovery, cacheErr, restoreCodexConfig(home, original, expected))
		recovery = append(recovery, s.verifyCodexInstalled(recoveryCtx, c.Target, c.ID, u.OldVersion, u.Enabled))
		if err := errors.Join(recovery...); err != nil {
			keep = true
			resultErr = fmt.Errorf("%w; recovery incomplete: %v; preserved recovery files at %s", resultErr, err, backup)
		}
	}()
	if u.Refresh {
		mutated = true
		data, nativeErr := s.run(ctx, c.Target, "plugin", "marketplace", "upgrade", market, "--json")
		if err := capture(); err != nil {
			keep = true
			return fmt.Errorf("cannot fingerprint native result: %w; recovery files at %s", err, backup)
		}
		if nativeErr != nil {
			return nativeErr
		}
		var response struct {
			Selected []string          `json:"selectedMarketplaces"`
			Roots    []string          `json:"upgradedRoots"`
			Errors   []json.RawMessage `json:"errors"`
		}
		if json.Unmarshal(data, &response) != nil || len(response.Selected) != 1 || response.Selected[0] != market || response.Roots == nil || response.Errors == nil || len(response.Errors) != 0 {
			return fmt.Errorf("native Codex marketplace upgrade was unsuccessful or returned an unrecognized result; inspect native trust/authentication requirements")
		}
		if err := verifyCodexSource(root, u); err != nil {
			return err
		}
		if err := verifyCodexMarketplaceCommit(ctx, root, u.Commit); err != nil {
			return err
		}
	}
	marketKey, _ := json.Marshal(market)
	reviewedPath, _ := json.Marshal(filepath.Join(backup, "reviewed"))
	mutated = true
	_, nativeErr := s.run(ctx, c.Target, "plugin", "add", c.ID, "--json", "-c", "marketplaces."+string(marketKey)+".source_type=\"local\"", "-c", "marketplaces."+string(marketKey)+".source="+string(reviewedPath))
	if err := capture(); err != nil {
		keep = true
		return fmt.Errorf("cannot fingerprint native result: %w; recovery files at %s", err, backup)
	}
	if nativeErr != nil {
		return nativeErr
	}
	if err := verifyCodexSource(root, u); err != nil {
		return err
	}
	if u.Refresh {
		if err := verifyCodexMarketplaceCommit(ctx, root, u.Commit); err != nil {
			return err
		}
	}
	if err := verifyCodexSource(filepath.Join(backup, "reviewed"), u); err != nil {
		return err
	}
	if err := s.verifyCodexInstalled(ctx, c.Target, c.ID, b.Version, true); err != nil {
		return err
	}
	currentCache, err := codexStateDigest(cache)
	if err != nil {
		return err
	}
	if currentCache != cacheAfter {
		return fmt.Errorf("native Codex cache changed before restoration")
	}
	if err := restoreCodexConfig(home, original, expected); err != nil {
		return err
	}
	// Refresh can reinstall every configured sibling, including uncached plugins.
	// Restore every unselected directory and its original absence, bytes and modes.
	if u.Refresh {
		entries, err := os.ReadDir(cache)
		if err != nil {
			return err
		}
		old, err := os.ReadDir(filepath.Join(backup, "cache"))
		if err != nil {
			return err
		}
		names := map[string]bool{}
		for _, e := range entries {
			names[e.Name()] = true
		}
		for _, e := range old {
			names[e.Name()] = true
		}
		for sibling := range names {
			if sibling == name {
				continue
			}
			path := filepath.Join(cache, sibling)
			if err := restoreCodexState(home, path, filepath.Join(backup, "cache", sibling), statesAfter[sibling]); err != nil {
				return err
			}
		}
		if err := capture(); err != nil {
			return err
		}
	}
	if err := s.verifyCodexInstalled(ctx, c.Target, c.ID, b.Version, u.Enabled); err != nil {
		return err
	}
	// Inventory is independent of the sibling fingerprint: do not accept a same
	// version as proof that unrelated cached contents survived the native refresh.
	if err := verifyCodexSiblings(cache, filepath.Join(backup, "cache"), name); err != nil {
		return err
	}
	current, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil || !bytes.Equal(current, original) {
		return fmt.Errorf("native Codex configuration changed during verification")
	}
	return nil
}

func verifyCodexSiblings(cache, backup, selected string) error {
	entries, err := os.ReadDir(cache)
	if err != nil {
		return err
	}
	old, err := os.ReadDir(backup)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
	}
	for _, e := range old {
		names[e.Name()] = true
	}
	for name := range names {
		if name == selected {
			continue
		}
		a, err := codexStateDigest(filepath.Join(cache, name))
		if err != nil {
			return err
		}
		b, err := codexStateDigest(filepath.Join(backup, name))
		if err != nil {
			return err
		}
		if a != b {
			return fmt.Errorf("unselected Codex plugin changed during update")
		}
	}
	return nil
}
