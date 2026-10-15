package skillserve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/skillpkg"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/textdiff"
)

// EditOptions turns on editing existing skill files. A client previews a
// change, which writes nothing, and applies the preview by its id alone, so
// what is saved is exactly what was shown.
type EditOptions struct {
	// Skills are patterns matched like a target's include filter.
	Skills     []string
	PreviewTTL time.Duration
	// Threshold is audit.block_threshold; a new finding at or above it refuses the edit.
	Threshold string
	// ProjectRoot is set in project mode, for the project's audit rules.
	ProjectRoot string
	// Log records each save to oplog.OpsFile and each audit to
	// oplog.AuditFile; nil records nothing.
	Log func(file string, e oplog.Entry)
}

const notWritten = "Nothing was written."

// preview is a checked change waiting to be applied.
type preview struct {
	uri     string
	skill   ssync.DiscoveredSkill
	rel     string // slash path inside the skill
	base    string // revision the change was made against
	content []byte
	expires time.Time
}

type previewInput struct {
	URI          string `json:"uri" jsonschema:"The skill:// URI of an existing file, as read_skill lists it"`
	BaseRevision string `json:"base_revision" jsonschema:"The revision read_skill printed for the file, which is also its digest in skills/list"`
	Content      string `json:"content" jsonschema:"The complete new content of the file, without the footer read_skill adds"`
}

type applyInput struct {
	PreviewID string `json:"preview_id" jsonschema:"The id preview_skill_edit returned"`
}

func (s *service) addEditTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "preview_skill_edit",
		Description: "Check a change to one existing file of a skill and show the full diff and audit result. " +
			"Writes nothing. Pass the revision read_skill returned; show the diff to the user, then save it with apply_skill_edit.",
		Annotations: readOnly,
	}, s.previewTool)
	destructive := true
	mcp.AddTool(srv, &mcp.Tool{
		Name: "apply_skill_edit",
		Description: "Save a change checked by preview_skill_edit, by its preview id. " +
			"Fails with a conflict when the file changed since it was read; read it again and preview again.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, s.applyTool)
}

func (s *service) previewTool(_ context.Context, _ *mcp.CallToolRequest, in previewInput) (*mcp.CallToolResult, any, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	skill, rel, err := owner(c, in.URI)
	if err != nil {
		return nil, nil, err
	}
	if err := s.writable(skill); err != nil {
		return nil, nil, err
	}
	old, err := c.Read(in.URI)
	if err != nil {
		return nil, nil, fmt.Errorf("conflict: %v. %s", err, notWritten)
	}
	if rev := c.Revision(in.URI); in.BaseRevision != rev {
		return nil, nil, fmt.Errorf("conflict: %s is now at revision %s, not %s; read it again. %s", in.URI, rev, in.BaseRevision, notWritten)
	}
	if !utf8.Valid(old) || !utf8.ValidString(in.Content) {
		return nil, nil, fmt.Errorf("invalid: only text files can be edited, and the content must be UTF-8. %s", notWritten)
	}
	findings, err := s.check(skill, rel, []byte(in.Content))
	if err != nil {
		return nil, nil, err
	}

	id := newPreviewID()
	s.editMu.Lock()
	now := time.Now()
	for k, p := range s.previews {
		if now.After(p.expires) {
			delete(s.previews, k)
		}
	}
	s.previews[id] = &preview{uri: in.URI, skill: skill, rel: rel, base: in.BaseRevision, content: []byte(in.Content), expires: now.Add(s.edit.PreviewTTL)}
	s.editMu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "Preview %s of %s, against revision %s. %s\n", id, in.URI, in.BaseRevision, notWritten)
	fmt.Fprintf(&b, "Save it with apply_skill_edit within %s.\n\n", s.edit.PreviewTTL)
	writeFindings(&b, findings)
	b.WriteString("\nDiff (+ added, - removed):\n")
	b.WriteString(textdiff.Lines(string(old), in.Content))
	return textResult(b.String()), nil, nil
}

func (s *service) applyTool(_ context.Context, _ *mcp.CallToolRequest, in applyInput) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	s.editMu.Lock()
	defer s.editMu.Unlock()
	p := s.previews[in.PreviewID]
	delete(s.previews, in.PreviewID) // a preview is applied at most once
	if p == nil || time.Now().After(p.expires) {
		return nil, nil, fmt.Errorf("expired: no pending preview %q; it expired, was already applied, or the server restarted. Preview the change again. %s", in.PreviewID, notWritten)
	}
	if err := s.writable(p.skill); err != nil {
		return nil, nil, err
	}
	path := filepath.Join(p.skill.SourcePath, filepath.FromSlash(p.rel))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("conflict: %s is no longer a regular file; read it again. %s", p.uri, notWritten)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if rev := skillpkg.Digest(current); rev != p.base {
		return nil, nil, fmt.Errorf("conflict: %s changed after the preview (now %s); read it again and preview again. %s", p.uri, rev, notWritten)
	}
	// Other files may have changed since the preview: check the skill as it will be saved.
	if _, err := s.check(p.skill, p.rel, p.content); err != nil {
		return nil, nil, err
	}
	src, err := sourcefs.Open(s.b.Source, s.b.Walk.Follow)
	if err != nil {
		return nil, nil, err
	}
	defer src.Close()
	rel, err := src.Rel(path)
	if err == nil {
		err = src.WriteFileAtomic(rel, p.content, info.Mode().Perm())
	}
	if errors.Is(err, sourcefs.ErrLink) {
		return nil, nil, fmt.Errorf("refused: %v. %s", err, notWritten)
	}
	if err != nil {
		return nil, nil, err
	}
	// The cached catalog lists the old digest, which read_skill would refuse.
	s.mu.Lock()
	s.cat = nil
	s.mu.Unlock()

	rev := skillpkg.Digest(p.content)
	if s.edit.Log != nil {
		e := oplog.NewEntry("mcp serve", "ok", time.Since(start))
		e.Args = map[string]any{"action": "edit", "skill": p.skill.RelPath, "file": p.rel, "from": p.base, "to": rev}
		s.edit.Log(oplog.OpsFile, e)
	}
	return textResult(fmt.Sprintf("Saved %s at revision %s (was %s). Read it back with read_skill to verify. Targets get the change on the next sync.", p.uri, rev, p.base)), nil, nil
}

// owner returns the served skill whose manifest lists the file at uri, and
// the file's path inside it.
func owner(c *Catalog, uri string) (ssync.DiscoveredSkill, string, error) {
	key, ok := canonical(uri)
	sf, listed := c.byFile[key]
	if !ok || !listed {
		return ssync.DiscoveredSkill{}, "", fmt.Errorf("invalid: no skill file %s; only existing files can be edited, and list_skills gives the skill URIs. %s", uri, notWritten)
	}
	return sf.owner.src, sf.rel, nil
}

func (s *service) writable(skill ssync.DiscoveredSkill) error {
	store, err := readMetadata(s.b.Source)
	if err != nil {
		return err
	}
	return writable(s.edit, store, s.b.Source, skill)
}

// Editable returns the source paths of the served skills e lets clients edit.
func (b *Builder) Editable(c *Catalog, e *EditOptions) ([]string, error) {
	store, err := readMetadata(b.Source)
	if errors.Is(err, errLegacyMetadata) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, skill := range c.Skills {
		if writable(e, store, b.Source, skill.src) == nil {
			paths = append(paths, skill.src.RelPath)
		}
	}
	return paths, nil
}

var errLegacyMetadata = fmt.Errorf("not_writable: the source still has a legacy registry.yaml, so installed skills cannot be told apart; run any skillshare command, such as skillshare list, to migrate it. %s", notWritten)

// readMetadata reads the install records without migrating them, since a
// preview promises to write nothing.
func readMetadata(source string) (*install.MetadataStore, error) {
	store, legacy, err := install.ReadMetadata(source)
	if err == nil && legacy {
		err = errLegacyMetadata
	}
	return store, err
}

// writable refuses skills outside mcp.serve.edit.skills and skills that an
// update could overwrite: those in a tracked repo, and those installed from
// upstream or nested inside one.
func writable(e *EditOptions, store *install.MetadataStore, source string, skill ssync.DiscoveredSkill) error {
	if ok, err := ssync.ShouldSyncFlatName(skill.FlatName, e.Skills, nil); err != nil || !ok {
		return fmt.Errorf("not_writable: %s is not listed in mcp.serve.edit.skills. %s", skill.RelPath, notWritten)
	}
	if skill.IsInRepo {
		return fmt.Errorf("not_writable: %s is inside the tracked repo %s, which update would overwrite. %s", skill.RelPath, skill.RepoRelPath, notWritten)
	}
	for p := skill.RelPath; p != "." && p != "/"; p = path.Dir(p) {
		if installed(store, source, p) {
			return fmt.Errorf("not_writable: %s was installed from upstream, and update would overwrite it. %s", p, notWritten)
		}
	}
	return nil
}

// installed reports whether the store, or a legacy sidecar file, records the
// skill at rel. Unlike GetByPath it skips the basename-only fallback, which
// would mistake a local team/pdf for an installed top-level pdf; tracked
// repos are refused before.
func installed(store *install.MetadataStore, source, rel string) bool {
	if store.Entries[rel] != nil {
		return true
	}
	if e := store.Entries[path.Base(rel)]; e != nil && path.Dir(rel) != "." && e.Group == path.Dir(rel) {
		return true
	}
	_, err := os.Stat(filepath.Join(source, filepath.FromSlash(rel), install.MetaFileName))
	return err == nil
}

// check validates the skill with the new content in a copy and audits the
// copy the way install does: a finding at or above the threshold refuses the
// edit unless it was accepted with --force. Findings are marked new or
// existing against the skill as it is.
func (s *service) check(skill ssync.DiscoveredSkill, rel string, content []byte) (marked []markedFinding, err error) {
	start := time.Now()
	var after *audit.Result
	defer func() {
		if after != nil || errors.Is(err, errAuditFailed) {
			s.logAudit(skill, rel, after, err, time.Since(start))
		}
	}()
	pkg, err := skillpkg.Load(skill.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("invalid: %s is not a valid skill now: %v. %s", skill.RelPath, err, notWritten)
	}
	tmp, err := os.MkdirTemp("", "skillshare-edit-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// The copy keeps the skill's folder name, which its frontmatter name must match.
	newDir := filepath.Join(tmp, filepath.Base(skill.SourcePath))
	for _, f := range pkg.Files {
		data := content
		if f.Rel != rel {
			if data, err = os.ReadFile(f.Path); err != nil {
				return nil, err
			}
		}
		if err := writeCopy(newDir, f.Rel, data); err != nil {
			return nil, err
		}
	}
	if _, err := skillpkg.Load(newDir); err != nil {
		return nil, fmt.Errorf("invalid: with this content the skill %v. %s", err, notWritten)
	}
	before, err := s.scan(skill.SourcePath, s.b.Walk.Follow)
	if err != nil {
		return nil, err
	}
	scanned, err := s.scan(newDir, nil)
	if err != nil {
		return nil, err
	}
	after = scanned
	install.ApplyAcceptedFindings(s.b.Source, skill.SourcePath, after)
	marked = markNew(before.Findings, after.Findings)
	if after.HasSeverityAtOrAbove(s.edit.Threshold) {
		var blocking []markedFinding
		for _, f := range marked {
			if !f.Acknowledged && audit.SeverityRank(f.Severity) <= audit.SeverityRank(s.edit.Threshold) {
				blocking = append(blocking, f)
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "audit_blocked: with this change the skill has findings at or above audit.block_threshold (%s). %s\n", s.edit.Threshold, notWritten)
		writeFindings(&b, blocking)
		return nil, errors.New(strings.TrimSpace(b.String()))
	}
	return marked, nil
}

func writeCopy(dir, rel string, data []byte) error {
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// scan audits dir the way install audits a skill. It fails closed: an audit
// that cannot run refuses the edit.
func (s *service) scan(dir string, follow *sourcewalk.Follow) (*audit.Result, error) {
	var res *audit.Result
	var err error
	if s.edit.ProjectRoot != "" {
		res, err = audit.ScanSkillForProject(dir, s.edit.ProjectRoot, follow)
	} else {
		res, err = audit.ScanSkillWithFollow(dir, follow)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v. %s", errAuditFailed, err, notWritten)
	}
	return res, nil
}

var errAuditFailed = errors.New("audit_blocked: the audit could not run")

// logAudit records an edit's audit in audit.log, like install's audit gate.
func (s *service) logAudit(skill ssync.DiscoveredSkill, rel string, res *audit.Result, err error, d time.Duration) {
	if s.edit.Log == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "blocked"
		if errors.Is(err, errAuditFailed) {
			status = "error"
		}
	}
	e := oplog.NewEntry("audit", status, d)
	e.Args = map[string]any{"mode": "mcp serve edit", "name": skill.RelPath, "file": rel, "threshold": s.edit.Threshold}
	if res != nil {
		e.Args["max_severity"] = res.MaxSeverity()
		e.Args["findings"] = len(res.Findings)
	}
	if err != nil {
		e.Message, _, _ = strings.Cut(err.Error(), "\n")
	}
	s.edit.Log(oplog.AuditFile, e)
}

type markedFinding struct {
	audit.Finding
	isNew bool
}

// markNew marks the findings in after that before does not have. Findings
// match by audit.AcceptKey, which leaves out the line, so moving text is not new.
func markNew(before, after []audit.Finding) []markedFinding {
	seen := map[string]int{}
	for _, f := range before {
		seen[audit.AcceptKey(f)]++
	}
	marked := make([]markedFinding, len(after))
	for i, f := range after {
		k := audit.AcceptKey(f)
		marked[i] = markedFinding{Finding: f, isNew: seen[k] == 0}
		if seen[k] > 0 {
			seen[k]--
		}
	}
	slices.SortStableFunc(marked, func(x, y markedFinding) int {
		return audit.SeverityRank(x.Severity) - audit.SeverityRank(y.Severity)
	})
	return marked
}

func writeFindings(b *strings.Builder, findings []markedFinding) {
	if len(findings) == 0 {
		b.WriteString("Audit: no findings.\n")
		return
	}
	n := 0
	for _, f := range findings {
		if f.isNew {
			n++
		}
	}
	fmt.Fprintf(b, "Audit: %d findings, %d new with this change:\n", len(findings), n)
	for _, f := range findings {
		state := "existing"
		if f.isNew {
			state = "new"
		}
		if f.Acknowledged {
			state += ", accepted"
		}
		fmt.Fprintf(b, "- [%s] %s %s %s:%d %s\n", state, f.Severity, f.Pattern, f.File, f.Line, f.Message)
	}
}

func newPreviewID() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck // crypto/rand.Read never fails
	return hex.EncodeToString(b)
}
