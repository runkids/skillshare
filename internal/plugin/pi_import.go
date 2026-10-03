package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tailscale/hujson"
)

// The native entry is private state, not a config value or an API payload. Entry
// is a string so JSON serialization preserves its original bytes and escapes.
type piRegistration struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	ID     string `json:"id"`
	Entry  string `json:"entry"`
}

func piRegistrationDigestOK(value string) bool {
	return len(value) == 64 && strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' && r < 'a' || r > 'f' }) < 0
}

func piRegistrationID(source, file string) string {
	if !strings.HasPrefix(source, "npm:") && !strings.HasPrefix(source, "git:") && !strings.Contains(source, "://") && !strings.HasPrefix(source, "git@") {
		if strings.HasPrefix(source, "~/") {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, source[2:])
		}
		if !filepath.IsAbs(source) {
			return filepath.Join(filepath.Dir(file), source)
		}
	}
	return source
}

func (s *Service) piRegistrationEntry(target, id string) (*piSettings, piEntry, []byte, error) {
	file, err := s.piSettingsPath(target)
	if err != nil {
		return nil, piEntry{}, nil, err
	}
	st := readPiSettings(s.piSettingsRoot(file), file)
	if st.problem != "" {
		return nil, piEntry{}, nil, fmt.Errorf("Pi settings cannot be preserved safely: %s", st.problem)
	}
	agentDir, err := s.piAgentDir(target)
	if err != nil {
		return nil, piEntry{}, nil, err
	}
	scope, base := "user", agentDir
	if s.ProjectRoot != "" {
		scope, base = "project", filepath.Join(s.ProjectRoot, ".pi")
	}
	index := -1
	for _, e := range st.entries {
		if !e.badSource && piRegistrationID(e.source, file) == id {
			if index != -1 {
				return nil, piEntry{}, nil, errors.New("Pi registration is ambiguous; resolve duplicate entries in Pi first")
			}
			index = e.index
		}
	}
	if index == -1 {
		return nil, piEntry{}, nil, errors.New("Pi registration is missing; preview again")
	}
	e := st.entries[index]
	identity := resolvePiSource(e.source, agentDir, base, scope).identity
	if e.problem != "" || identity == "" || strings.TrimSpace(e.source) != e.source || redactSource(e.source) != e.source || strings.HasPrefix(e.source, "git@") {
		return nil, piEntry{}, nil, errors.New("Pi source or entry cannot be preserved safely; manage it in Pi")
	}
	// Do not adopt an ignored entry, or guess ownership around unresolved ones.
	for _, other := range st.entries {
		if (scope == "user" && other.index >= index) || (scope == "project" && other.index <= index) {
			continue
		}
		otherID := resolvePiSource(other.source, agentDir, base, scope).identity
		if other.badSource || otherID == "" || otherID == identity {
			return nil, piEntry{}, nil, errors.New("Pi registration precedence is unresolved; manage it in Pi")
		}
	}
	if resolved := resolvePiSource(e.source, agentDir, base, scope); resolved.kind == "local" {
		normalized, relErr := filepath.Rel(filepath.Dir(file), piRegistrationID(e.source, file))
		if relErr != nil || normalized != e.source {
			return nil, piEntry{}, nil, errors.New("Pi would normalize this local source during reinstall; import a native-normalized entry instead")
		}
	}
	var list []json.RawMessage
	if err := json.Unmarshal(st.body["packages"], &list); err != nil {
		return nil, piEntry{}, nil, err
	}
	raw := list[index]
	if !utf8.Valid(raw) {
		return nil, piEntry{}, nil, errors.New("Pi entry has unsupported encoding")
	}
	for _, key := range []string{"extensions", "skills", "prompts", "themes"} {
		if field, ok := e.fields[key]; ok {
			var values []string
			if json.Unmarshal(field, &values) != nil || values == nil || !piLossless(field) {
				return nil, piEntry{}, nil, fmt.Errorf("unsupported Pi %s options", key)
			}
		}
	}
	if field, ok := e.fields["autoload"]; ok {
		var autoload bool
		if string(field) == "null" || json.Unmarshal(field, &autoload) != nil {
			return nil, piEntry{}, nil, errors.New("unsupported Pi autoload option")
		}
	}
	return st, e, raw, nil
}

func piPreservedKeys(raw string) []string {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func (s *Service) preparePiRegistration(ctx context.Context, c *Change) error {
	if _, why := s.piGate(ctx, c.Target); why != "" {
		return fmt.Errorf("Pi filtered registrations require a supported native Pi version: %s", why)
	}
	if !filepath.IsAbs(s.StateDir) {
		return errors.New("Pi preservation requires an absolute private state directory")
	}
	file, err := s.piSettingsPath(c.Target)
	if err != nil {
		return err
	}
	if c.Action == "install" {
		record, err := s.readPiRegistration(c.Binding.PiRegistration, c.Target, c.ID)
		if err != nil {
			return err
		}
		c.PreservedKeys = piPreservedKeys(record.Entry)
		st := readPiSettings(s.piSettingsRoot(file), file)
		if st.problem != "" {
			return errors.New("Pi settings cannot be restored safely")
		}
		if err := s.checkPiRestoreOwnership(st, record, c.Binding.Pending == "install"); err != nil {
			return err
		}
		c.piSettingsHash = hash(st.raw)
		return nil
	}
	if c.Action != "import" && c.Action != "remove" && c.Action != "uninstall" && c.Action != "update" {
		return nil
	}
	st, _, raw, err := s.piRegistrationEntry(c.Target, c.ID)
	if err != nil {
		return err
	}
	record := piRegistration{Target: c.Target, Path: file, ID: c.ID, Entry: string(raw)}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	c.Binding.PiRegistration, c.piRecord, c.piSettingsHash = hash(data), data, hash(st.raw)
	c.PreservedKeys = piPreservedKeys(record.Entry)
	return nil
}

func (s *Service) piRegistrationPath(digest string) (string, error) {
	if !filepath.IsAbs(s.StateDir) || !piRegistrationDigestOK(digest) {
		return "", errors.New("invalid preserved Pi registration reference")
	}
	file := filepath.Join(s.StateDir, "pi-registrations", digest+".json")
	if err := noSymlink(s.StateDir, file); err != nil {
		return "", err
	}
	return file, nil
}

func (s *Service) readPiRegistration(digest, target, id string) (*piRegistration, error) {
	file, err := s.piRegistrationPath(digest)
	if err != nil {
		return nil, err
	}
	if err := checkPiRegistrationPrivate(filepath.Dir(file)); err != nil {
		return nil, err
	}
	if err := checkPiRegistrationPrivate(file); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("preserved Pi registration unavailable; restore private state or re-import: %w", err)
	}
	var record piRegistration
	path, pathErr := s.piSettingsPath(target)
	if hash(data) != digest || json.Unmarshal(data, &record) != nil || pathErr != nil || record.Target != target || record.Path != path || record.ID != id || !json.Valid([]byte(record.Entry)) {
		return nil, errors.New("preserved Pi registration changed or belongs to another target")
	}
	return &record, nil
}

func (s *Service) savePiRegistration(c Change) error {
	if len(c.piRecord) == 0 {
		return nil
	}
	path, err := s.piSettingsPath(c.Target)
	if err != nil {
		return err
	}
	st := readPiSettings(s.piSettingsRoot(path), path)
	if st.problem != "" || hash(st.raw) != c.piSettingsHash {
		return errors.New("Pi settings changed since preview; preview again")
	}
	file, err := s.piRegistrationPath(c.Binding.PiRegistration)
	if err != nil {
		return err
	}
	if hash(c.piRecord) != c.Binding.PiRegistration {
		return errors.New("invalid preserved Pi registration")
	}
	if err := makePiRegistrationDir(filepath.Dir(file)); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(file))
	if err != nil {
		return err
	}
	defer root.Close()
	// On Windows the opened directory blocks replacement while its ACL is checked.
	if err := checkPiRegistrationPrivate(filepath.Dir(file)); err != nil {
		return err
	}
	if err := rootAtomicWrite(root, filepath.Base(file), c.piRecord, 0o600, true); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := checkPiRegistrationPrivate(file); err != nil {
			return err
		}
		existing, readErr := root.ReadFile(filepath.Base(file))
		if readErr != nil || !bytes.Equal(existing, c.piRecord) {
			return errors.New("preserved Pi registration was modified; restore it before retrying")
		}
	}
	return checkPiRegistrationPrivate(file)
}

func (s *Service) checkPiRestoreOwnership(st *piSettings, record *piRegistration, retry bool) error {
	agentDir, err := s.piAgentDir(record.Target)
	if err != nil {
		return err
	}
	scope, base := "user", agentDir
	if s.ProjectRoot != "" {
		scope, base = "project", filepath.Join(s.ProjectRoot, ".pi")
	}
	identity := resolvePiSource(parsePiEntry(0, []byte(record.Entry)).source, agentDir, base, scope).identity
	if identity == "" {
		return errors.New("preserved Pi identity cannot be verified")
	}
	matched := false
	for _, e := range st.entries {
		otherID := resolvePiSource(e.source, agentDir, base, scope).identity
		if retry && otherID == identity && piRegistrationID(e.source, st.path) == record.ID {
			var entries []json.RawMessage
			if json.Unmarshal(st.body["packages"], &entries) == nil && string(entries[e.index]) == record.Entry && !matched {
				matched = true
				continue
			}
		}
		if e.badSource || otherID == "" || otherID == identity {
			return errors.New("Pi registration appeared or is ambiguous; preview again")
		}
	}
	return nil
}

var piBeforeRegistrationWrite = func(string) {}

// A batch may advance a reviewed file only to bytes this Apply itself wrote.
// Native/external writes are not adopted by reading the file after a command.
type piRestoreReceipt struct {
	reviewed string
	written  string
}

// Restore the reviewed object before calling pi install: installing a string and
// patching filters afterwards would briefly default-enable other resources.
func (s *Service) restorePiRegistration(c Change, b Binding) error {
	record, err := s.readPiRegistration(b.PiRegistration, c.Target, b.ID)
	if err != nil {
		return err
	}
	file := record.Path
	if err := noSymlink(s.piSettingsRoot(file), file); err != nil {
		return err
	}
	// A preserved import necessarily had settings here. Never recreate a removed
	// project or settings directory while trying to reinstall a package.
	lock, err := acquirePiNativeLock(file + ".lock")
	if err != nil {
		return err
	}
	defer lock.release()
	expected := c.piSettingsHash
	if receipt, ok := c.piRestores[file]; ok {
		if receipt.reviewed != expected {
			return errors.New("Pi settings changed since preview; preview again")
		}
		expected = receipt.written
	}
	st := readPiSettings(s.piSettingsRoot(file), file)
	if st.problem != "" || hash(st.raw) != expected {
		return errors.New("Pi settings changed since preview; preview again")
	}
	if err := s.checkPiRestoreOwnership(st, record, b.Pending == "install"); err != nil {
		return err
	}
	if b.Pending == "install" {
		for _, e := range st.entries {
			if !e.badSource && piRegistrationID(e.source, file) == b.ID {
				return nil
			}
		}
	}
	value, err := hujson.Parse(st.raw)
	if err != nil {
		return err
	}
	member := value.Find("/packages")
	if member == nil {
		return errors.New("Pi package list disappeared; restore it in Pi before retrying")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(st.body["packages"], &entries); err != nil {
		return err
	}
	entries = append(entries, json.RawMessage(record.Entry))
	parts := make([]string, len(entries))
	for i, entry := range entries {
		parts[i] = string(entry)
	}
	member.Value = hujson.Literal("[" + strings.Join(parts, ",") + "]")
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	piBeforeRegistrationWrite(file)
	latest := readPiSettings(s.piSettingsRoot(file), file)
	if latest.problem != "" || hash(latest.raw) != expected {
		return errors.New("Pi settings changed at the write boundary; preview again")
	}
	if err := lock.verify(); err != nil {
		return err
	}
	out := value.Pack()
	if s.ProjectRoot != "" {
		root, openErr := os.OpenRoot(s.ProjectRoot)
		if openErr != nil {
			return openErr
		}
		defer root.Close()
		err = rootAtomicWrite(root, ".pi/settings.json", out, info.Mode().Perm(), false)
	} else {
		err = atomicNativeWrite(s.piSettingsRoot(file), file, out, info.Mode().Perm())
	}
	if err == nil && c.piRestores != nil {
		c.piRestores[file] = piRestoreReceipt{reviewed: c.piSettingsHash, written: hash(out)}
	}
	return err
}
