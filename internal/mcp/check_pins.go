package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxToolPinBytes = 16 << 20

// ToolsAcceptance is the persistence outcome, independent of report findings.
// Only save may report accepted, including an unchanged, CAS-verified no-op.
type ToolsAcceptance string

const (
	ToolsAcceptanceAccepted ToolsAcceptance = "accepted"
	ToolsAcceptanceStale    ToolsAcceptance = "stale"
	ToolsAcceptanceError    ToolsAcceptance = "error"
)

// Legacy hashes remain evidence only. Records bind complete explicitly approved
// contracts to one canonical source/configuration, effective root and server.
type toolPinFile struct {
	Version int                                     `json:"version"`
	Servers map[string]map[string]map[string]string `json:"servers"`
	Records map[string]toolPinRecord                `json:"records,omitempty"`
}

type toolPinRecord struct {
	Config   string                  `json:"config"`
	Root     string                  `json:"root"`
	Name     string                  `json:"name"`
	Policy   string                  `json:"policy"`
	Identity string                  `json:"identity"`
	Tools    map[string]ToolContract `json:"tools"`
}

func (s *Service) toolPinsPath() string { return filepath.Join(s.StateDir, "mcp", "tool-pins.json") }

var errToolPins = errors.New("MCP tool baseline is invalid or unreadable; restore a valid backup or inspect the state file")
var errStaleTools = errors.New("MCP review is stale; run a new live review before accepting")

// Keep the original bytes for compare-and-swap. Do not use safeRead here: its
// unbounded ReadFile would allocate before the security file limit was checked.
func readToolPinBytes(path string) ([]byte, bool, error) {
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Size() > maxToolPinBytes {
		return nil, false, errToolPins
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, errToolPins
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Size() > maxToolPinBytes {
		return nil, false, errToolPins
	}
	data, err := io.ReadAll(io.LimitReader(f, maxToolPinBytes+1))
	if err != nil || len(data) > maxToolPinBytes {
		return nil, false, errToolPins
	}
	after, err := os.Lstat(path)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		return nil, false, errToolPins
	}
	return data, true, nil
}

func emptyToolPins() toolPinFile {
	return toolPinFile{Version: 2, Servers: map[string]map[string]map[string]string{}, Records: map[string]toolPinRecord{}}
}

func parseToolPins(data []byte) (toolPinFile, error) {
	var pins toolPinFile
	// The store adds nesting around each descriptor; each descriptor is separately
	// recaptured at its own strict 64-level bound below.
	var shape map[string]json.RawMessage
	if decodeJSONBounded(data, &shape, 72) != nil || shape == nil {
		return pins, errToolPins
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&pins) != nil || pins.Servers == nil {
		return pins, errToolPins
	}
	if pins.Version != 1 && pins.Version != 2 {
		return pins, errToolPins
	}
	if pins.Version == 1 {
		if _, ok := shape["records"]; ok {
			return pins, errToolPins
		}
	} else {
		// Empty v2 records are emitted explicitly, rather than omitted by omitempty.
		if pins.Records == nil {
			return pins, errToolPins
		}
	}

	envelopeKeys := []string{"version", "servers"}
	if pins.Version == 2 {
		envelopeKeys = append(envelopeKeys, "records")
	}
	if !exactToolPinFields(shape, envelopeKeys...) {
		return pins, errToolPins
	}
	if pins.Version == 2 {
		var records map[string]json.RawMessage
		if json.Unmarshal(shape["records"], &records) != nil {
			return pins, errToolPins
		}
		for _, raw := range records {
			var record map[string]json.RawMessage
			if json.Unmarshal(raw, &record) != nil || !exactToolPinFields(record, "config", "root", "name", "policy", "identity", "tools") {
				return pins, errToolPins
			}
			for _, key := range []string{"config", "root", "name", "policy", "identity"} {
				if !bytes.HasPrefix(bytes.TrimSpace(record[key]), []byte{'"'}) {
					return pins, errToolPins
				}
			}
			var tools map[string]json.RawMessage
			if json.Unmarshal(record["tools"], &tools) != nil {
				return pins, errToolPins
			}
			for _, raw := range tools {
				var tool map[string]json.RawMessage
				if json.Unmarshal(raw, &tool) != nil || !exactToolPinFields(tool, "hash", "inputHash", "proseHash", "outputHash", "metadataHash", "definition") {
					return pins, errToolPins
				}
			}
		}
	}
	for _, servers := range pins.Servers {
		if servers == nil {
			return pins, errToolPins
		}
		for name, tools := range servers {
			if !serverName.MatchString(name) || tools == nil {
				return pins, errToolPins
			}
			for name, hash := range tools {
				if strings.TrimSpace(name) == "" || !toolHashValid(hash) {
					return pins, errToolPins
				}
			}
		}
	}
	for key, record := range pins.Records {
		if !toolHashValid(record.Config) || !toolHashValid(record.Identity) || !serverName.MatchString(record.Name) || record.Policy == "" || len(record.Policy) > 128 || key != toolRecordKey(record.Config, record.Root, record.Name) {
			return pins, errToolPins
		}
		if validateToolSnapshot(record.Tools) != nil {
			return pins, errToolPins
		}
	}
	return pins, nil
}

// encoding/json matches struct keys case-insensitively. The store grammar is
// exact: aliases such as Version or Hash must not shadow supported fields.
func exactToolPinFields(fields map[string]json.RawMessage, keys ...string) bool {
	if len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return false
		}
	}
	return true
}

func validateToolSnapshot(tools map[string]ToolContract) error {
	if tools == nil || len(tools) > 4096 {
		return errToolPins
	}
	total := 0
	for name, tool := range tools {
		total += len(tool.Definition)
		if total > 4<<20 || !toolHashValid(tool.Hash) || !toolHashValid(tool.InputHash) || !toolHashValid(tool.ProseHash) || !toolHashValid(tool.OutputHash) || !toolHashValid(tool.MetadataHash) {
			return errToolPins
		}
		capturedName, actual, err := captureTool(tool.Definition, nil)
		if err != nil || capturedName != name || actual.Hash != tool.Hash || actual.InputHash != tool.InputHash || actual.ProseHash != tool.ProseHash || actual.OutputHash != tool.OutputHash || actual.MetadataHash != tool.MetadataHash || !bytes.Equal(actual.Definition, tool.Definition) {
			return errToolPins
		}
	}
	return nil
}

// Inspect descriptor text, not stored fingerprint strings: hashes and resolved
// credentials must never become review content or credential-derived identities.
func toolSnapshotHasSecret(tools map[string]ToolContract, secrets []string) bool {
	if len(secrets) == 0 {
		return false
	}
	for name, tool := range tools {
		if contractHasSecret(name, secrets) {
			return true
		}
		decoded, err := contractJSON(tool.Definition)
		if err != nil || contractHasSecret(decoded, secrets) {
			return true
		}
	}
	return false
}

func toolHashValid(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, c := range hash {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (s *Service) readToolPins() (toolPinFile, error) {
	if s.StateDir == "" {
		return toolPinFile{}, errToolPins
	}
	data, exists, err := readToolPinBytes(s.toolPinsPath())
	if err != nil {
		return toolPinFile{}, err
	}
	if !exists {
		return emptyToolPins(), nil
	}
	return parseToolPins(data)
}

type contractObservation struct {
	Tools        map[string]ToolContract
	Command, Dir string
	Live         *CheckLive
	// Secrets are the current configured values, used only to refuse unsafe review
	// material. They are never hashed, persisted, or included in report fields.
	Secrets []string
}

type toolBaseline struct {
	source             *Source
	pins               toolPinFile
	err                error
	opts               CheckOptions
	config, configRoot string
	before             []byte
	existed            bool
	updates            []*toolPinRecord
	stale              []bool
}

func (s *Service) loadToolBaseline(source *Source, servers int, opts CheckOptions) *toolBaseline {
	b := &toolBaseline{source: source, opts: opts, configRoot: s.configProjectRoot(), updates: make([]*toolPinRecord, servers), stale: make([]bool, servers)}
	if s.StateDir == "" {
		b.err = errToolPins
		return b
	}
	b.config, b.err = toolConfigIdentity(source, opts.knownSecrets)
	if b.err != nil {
		return b
	}
	b.before, b.existed, b.err = readToolPinBytes(s.toolPinsPath())
	if b.err == nil {
		if b.existed {
			b.pins, b.err = parseToolPins(b.before)
		} else {
			b.pins = emptyToolPins()
		}
	}
	return b
}

func (b *toolBaseline) scope(root string) string {
	if root == "" {
		return b.configRoot
	}
	return root
}

func (b *toolBaseline) compare(i int, root, name string, server Server, now contractObservation) (*ContractReview, []CheckFinding) {
	review := &ContractReview{Status: "error", Policy: contractPolicy}
	fail := func(message string) (*ContractReview, []CheckFinding) {
		review.Status = "error"
		review.Revision = ""
		return review, []CheckFinding{{Level: "error", Check: "tool-change", Message: message}}
	}
	if b.err != nil {
		return fail("tool contracts could not be compared: " + b.err.Error())
	}
	root = b.scope(root)
	if contractHasSecret(root, now.Secrets) {
		return fail("project scope contains a configured credential; no review is available")
	}
	if now.Live == nil || validateToolSnapshot(now.Tools) != nil {
		return fail("complete valid tool contracts are required before review")
	}
	if toolSnapshotHasSecret(now.Tools, now.Secrets) {
		return fail("tool contracts contain a configured credential; no review is available")
	}
	review.Tools = now.Tools
	audit, err := auditContracts(now.Tools)
	review.Audit = audit
	if err != nil {
		return fail("tool contract audit was incomplete; review cannot be accepted")
	}
	if !toolReferencesOnly(server) {
		if b.opts.AcceptTools || b.opts.StrictTools {
			return fail("baseline trust requires fromEnv references for every environment, header and bearer value")
		}
		review.Status = "untrusted"
		fs := []CheckFinding{{Level: "warning", Check: "tool-change", Message: "baseline trust requires fromEnv references for every environment, header and bearer value"}}
		if len(audit) > 0 {
			fs = append(fs, CheckFinding{Level: "warning", Check: "tool-audit", Message: "tool contract text has audit findings; inspect the complete review"})
		}
		return review, fs
	}
	identity, err := toolConnectionIdentity(server, now)
	if err != nil {
		return fail("connection identity is unavailable; run a new complete live review")
	}
	key := toolRecordKey(b.config, root, name)
	record := toolPinRecord{Config: b.config, Root: root, Name: name, Policy: contractPolicy, Identity: identity, Tools: now.Tools}
	var fs []CheckFinding
	if len(audit) > 0 {
		fs = append(fs, CheckFinding{Level: "warning", Check: "tool-audit", Message: "tool contract text has audit findings; inspect the complete review"})
	}
	old, pinned := b.pins.Records[key]
	// A reference's resolved value may have rotated since the baseline was saved.
	// Validate old decoded text before compareContracts can expose removed names
	// or schema keys and before issuing a revision bound to the old snapshot.
	if pinned && toolSnapshotHasSecret(old.Tools, now.Secrets) {
		return fail("stored tool contracts contain a configured credential; no review is available")
	}
	if !pinned {
		for tool := range b.pins.Servers[root][name] {
			if contractHasSecret(tool, now.Secrets) {
				return fail("stored tool contracts contain a configured credential; no review is available")
			}
		}
	}
	switch {
	case pinned:
		review.IdentityChanged = old.Identity != identity || old.Policy != contractPolicy
		review.Changes, review.Omitted = compareContracts(old.Tools, now.Tools)
		review.Status = "unchanged"
		if review.IdentityChanged || len(review.Changes) > 0 || review.Omitted > 0 {
			review.Status = "drifted"
		}
	case b.pins.Servers[root] != nil && b.pins.Servers[root][name] != nil:
		review.Status = "legacy"
	default:
		review.Status = "untrusted"
	}
	// Bind the complete record and the exact pre-probe store snapshot. Source
	// bytes stay in memory for CheckUnchanged; they never become a public hash.
	payload, _ := json.Marshal(struct {
		Record toolPinRecord `json:"record"`
		Store  string        `json:"store"`
		Exists bool          `json:"exists"`
	}{record, digest(b.before), b.existed})
	review.Revision = digest(payload)
	if b.opts.AcceptTools {
		if !b.opts.Live || len(b.opts.Names) != 1 || b.opts.Names[0] != name || !toolHashValid(b.opts.Revision) {
			return fail("accepting tool contracts requires a live review of one named server and its revision")
		}
		if b.opts.Revision != review.Revision {
			b.stale[i] = true
			return fail(errStaleTools.Error())
		}
		b.updates[i] = &record
		return review, fs
	}
	if review.Status != "unchanged" {
		level := "warning"
		if b.opts.StrictTools {
			level = "error"
		}
		message := "tool contracts have no approved baseline; inspect the complete inventory and revision before accepting"
		if review.Status == "legacy" {
			message = "legacy tool hashes require explicit review of complete contracts and connection identity"
		}
		if review.Status == "drifted" {
			message = "tool contracts or connection identity changed; inspect all changes before accepting"
		}
		fs = append(fs, CheckFinding{Level: level, Check: "tool-change", Message: message})
	}
	return review, fs
}

// save is the only acceptance finalizer. Even a no-op rechecks source and exact
// store bytes while holding the existing cross-process operation lock.
func (b *toolBaseline) save(s *Service, report *CheckReport) {
	if !b.opts.AcceptTools {
		return
	}
	report.ToolsAcceptance = ToolsAcceptanceError
	fail := func(status ToolsAcceptance, message string) {
		report.ToolsAcceptance = status
		for i := range report.Servers {
			result := &report.Servers[i]
			if result.Contract == nil {
				result.Contract = &ContractReview{Policy: contractPolicy}
			}
			result.Contract.Status = "error"
			result.Contract.Revision = ""
			result.Findings = append(result.Findings, CheckFinding{Level: "error", Check: "tool-change", Message: message})
		}
	}
	if len(report.Servers) != 1 || len(b.updates) != 1 || b.updates[0] == nil {
		status := ToolsAcceptanceError
		for _, stale := range b.stale {
			if stale {
				status = ToolsAcceptanceStale
			}
		}
		fail(status, "tool baseline was not accepted; a complete valid live observation and current review revision are required")
		return
	}
	err := func() error {
		lock, err := s.lock()
		if err != nil {
			return err
		}
		defer lock.Unlock()
		if b.source.CheckUnchanged() != nil {
			return errStaleTools
		}
		config, err := toolConfigIdentity(b.source, b.opts.knownSecrets)
		if err != nil || config != b.config {
			return errStaleTools
		}
		current, exists, err := readToolPinBytes(s.toolPinsPath())
		if err != nil {
			return err
		}
		if exists != b.existed || !bytes.Equal(current, b.before) {
			return errStaleTools
		}
		pins := emptyToolPins()
		if exists {
			pins, err = parseToolPins(current)
			if err != nil {
				return err
			}
		}
		if pins.Records == nil {
			pins.Records = map[string]toolPinRecord{}
		}
		record := *b.updates[0]
		key := toolRecordKey(record.Config, record.Root, record.Name)
		if old, ok := pins.Records[key]; ok {
			before, _ := json.Marshal(old)
			after, _ := json.Marshal(record)
			if bytes.Equal(before, after) {
				return nil
			}
		}
		pins.Version = 2
		pins.Records[key] = record
		data, err := marshalToolPins(pins)
		if err != nil {
			return err
		}
		if len(data) > maxToolPinBytes {
			return errToolPins
		}
		return atomicWrite(s.toolPinsPath(), data, 0600)
	}()
	if err != nil {
		if errors.Is(err, errStaleTools) {
			fail(ToolsAcceptanceStale, errStaleTools.Error())
		} else {
			fail(ToolsAcceptanceError, "tool baseline could not be persisted; no acceptance was recorded")
		}
		return
	}
	report.ToolsAcceptance = ToolsAcceptanceAccepted
	report.Servers[0].Contract.Status = "accepted"
	report.Servers[0].Findings = append(report.Servers[0].Findings, CheckFinding{Level: "info", Check: "tool-change", Message: "accepted the reviewed complete tool contracts and connection identity"})
}

func marshalToolPins(pins toolPinFile) ([]byte, error) {
	// Compact encoding preserves canonical RawMessage snapshots. Emit empty
	// records explicitly for a complete v2 envelope.
	type envelope struct {
		Version int                                     `json:"version"`
		Servers map[string]map[string]map[string]string `json:"servers"`
		Records map[string]toolPinRecord                `json:"records"`
	}
	data, err := json.Marshal(envelope{pins.Version, pins.Servers, pins.Records})
	if err != nil {
		return nil, fmt.Errorf("could not encode MCP tool baseline")
	}
	return append(data, '\n'), nil
}
