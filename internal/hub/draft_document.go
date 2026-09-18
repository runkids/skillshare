package hub

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"skillshare/internal/install"
)

// Draft keeps authoring metadata separate from the portable v1 index.
type Draft struct {
	ID          string                     `json:"id"`
	Revision    string                     `json:"revision"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	UpdatedAt   string                     `json:"updatedAt"`
	Entries     []DraftEntry               `json:"entries"`
	Fields      map[string]json.RawMessage `json:"fields"`
}

// DraftEntry has an identity independent of its editable display name.
type DraftEntry struct {
	ID   string                     `json:"id"`
	Data map[string]json.RawMessage `json:"data"`
}

type DraftProblem struct {
	EntryID string `json:"entryId"`
	Code    string `json:"code"`
}

func draftID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate draft ID: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func field(data map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(data[key], &s)
	return s
}

func setField(data map[string]json.RawMessage, key, value string) {
	data[key], _ = json.Marshal(value)
}

// SourceProblem checks portability and syntax, without claiming remote access.
func SourceProblem(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "missing_source"
	}
	if strings.ContainsAny(source, "?#") {
		return "credentials"
	}
	if u, err := url.Parse(source); err == nil && u.User != nil {
		_, password := u.User.Password()
		if password || u.Scheme != "ssh" {
			return "credentials"
		}
	}
	if strings.IndexFunc(source, unicode.IsControl) >= 0 {
		return "invalid_source"
	}
	if strings.HasPrefix(source, `\\`) || (len(source) > 2 && source[1] == ':') {
		return "local_source"
	}
	parsed, err := install.ParseSource(source)
	if err != nil {
		return "invalid_source"
	}
	if parsed.Type == install.SourceTypeLocalPath || strings.HasPrefix(source, "file://") {
		return "local_source"
	}
	return ""
}

func validateDraft(d Draft) error {
	if len(d.Entries) > 5000 {
		return fmt.Errorf("a draft can contain at most 5000 skills")
	}
	seen := make(map[string]bool)
	for i, e := range d.Entries {
		if e.ID == "" || seen[e.ID] {
			return fmt.Errorf("skill %d: missing or duplicate row ID", i+1)
		}
		seen[e.ID] = true
		if e.Data == nil {
			return fmt.Errorf("skill %d: expected an object", i+1)
		}
		for _, key := range []string{"name", "description", "source", "skill"} {
			if v, ok := e.Data[key]; ok {
				var s string
				if string(v) == "null" || json.Unmarshal(v, &s) != nil {
					return fmt.Errorf("skill %d: %s must be a string", i+1, key)
				}
			}
		}
		if v, ok := e.Data["tags"]; ok {
			var tags []string
			if string(v) == "null" || json.Unmarshal(v, &tags) != nil {
				return fmt.Errorf("skill %d: tags must be an array of strings", i+1)
			}
		}
		if SourceProblem(field(e.Data, "source")) == "credentials" {
			return fmt.Errorf("skill %d: remove URL credentials, query strings and fragments before saving", i+1)
		}
	}
	return nil
}

// DraftProblems lists all blockers, including local skills; none are silently omitted.
func DraftProblems(d Draft) []DraftProblem {
	problems := make([]DraftProblem, 0)
	for _, e := range d.Entries {
		if strings.TrimSpace(field(e.Data, "name")) == "" {
			problems = append(problems, DraftProblem{e.ID, "missing_name"})
		}
		if code := SourceProblem(field(e.Data, "source")); code != "" {
			problems = append(problems, DraftProblem{e.ID, code})
		}
		if selector := field(e.Data, "skill"); selector != "" {
			// Selectors refer to a skill inside the repository, never a local file.
			if strings.HasPrefix(selector, "/") || strings.Contains(selector, "\\") || strings.Contains(selector, ":") || strings.IndexFunc(selector, unicode.IsControl) >= 0 {
				problems = append(problems, DraftProblem{e.ID, "invalid_selector"})
			} else {
				for _, part := range strings.Split(selector, "/") {
					if part == ".." {
						problems = append(problems, DraftProblem{e.ID, "invalid_selector"})
						break
					}
				}
			}
		}
	}
	if len(d.Entries) == 0 {
		problems = append(problems, DraftProblem{"", "empty"})
	}
	return problems
}

// ImportDraft accepts v1 documents and retains extension fields verbatim.
func ImportDraft(raw []byte) (Draft, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return Draft{}, fmt.Errorf("expected a Hub JSON object")
	}
	var version int
	if json.Unmarshal(doc["schemaVersion"], &version) != nil || version != 1 {
		return Draft{}, fmt.Errorf("only Hub schemaVersion 1 is supported")
	}
	var entries []map[string]json.RawMessage
	if string(doc["skills"]) == "null" || json.Unmarshal(doc["skills"], &entries) != nil {
		return Draft{}, fmt.Errorf("skills must be an array of objects")
	}
	sourcePath := field(doc, "sourcePath")
	if v, ok := doc["sourcePath"]; ok {
		var s string
		if json.Unmarshal(v, &s) != nil {
			return Draft{}, fmt.Errorf("sourcePath must be a string")
		}
	}
	d := Draft{Name: "Imported Hub", Fields: doc, Entries: make([]DraftEntry, 0, len(entries))}
	for _, data := range entries {
		if data == nil {
			return Draft{}, fmt.Errorf("each skill must be an object")
		}
		source := field(data, "source")
		if source == "" {
			source = field(data, "name")
		}
		if sourcePath != "" && relativeHubSource(source) {
			// Match the legacy consumer: owner/repo under sourcePath is a local path.
			source = filepath.Join(sourcePath, source)
			if !filepath.IsAbs(source) {
				source = "./" + source
			}
			setField(data, "source", source)
		}
		id, err := draftID()
		if err != nil {
			return Draft{}, err
		}
		d.Entries = append(d.Entries, DraftEntry{ID: id, Data: data})
	}
	for _, key := range []string{"schemaVersion", "generatedAt", "sourcePath", "skills"} {
		delete(d.Fields, key)
	}
	if err := validateDraft(d); err != nil {
		return Draft{}, err
	}
	return d, nil
}

func relativeHubSource(source string) bool {
	for _, p := range []string{"/", "~", "git@", "ssh://", "http://", "https://", "file://", `\\`} {
		if strings.HasPrefix(source, p) {
			return false
		}
	}
	if len(source) > 2 && source[1] == ':' {
		return false
	}
	first, _, _ := strings.Cut(source, "/")
	return !strings.Contains(first, ".") || first == "." || first == ".."
}

// ExportDraft emits a portable index, never the draft envelope or author paths.
func ExportDraft(d Draft) ([]byte, error) {
	if err := validateDraft(d); err != nil {
		return nil, err
	}
	if problems := DraftProblems(d); len(problems) > 0 {
		return nil, fmt.Errorf("resolve %d export problems before downloading", len(problems))
	}
	doc := make(map[string]json.RawMessage)
	for k, v := range d.Fields {
		doc[k] = v
	}
	// Builder bookkeeping: where the index is hosted is not part of the published schema.
	for _, key := range []string{"sourcePath", "publishUrl"} {
		delete(doc, key)
	}
	doc["schemaVersion"] = json.RawMessage("1")
	setField(doc, "generatedAt", time.Now().UTC().Format(time.RFC3339))
	skills := make([]map[string]json.RawMessage, 0, len(d.Entries))
	for _, e := range d.Entries {
		data := make(map[string]json.RawMessage)
		for k, v := range e.Data {
			data[k] = v
		}
		for _, key := range []string{"sourcePath", "relPath", "flatName", "installedAt", "isInRepo"} {
			delete(data, key)
		}
		if source := field(data, "repoUrl"); source != "" && SourceProblem(source) != "" {
			delete(data, "repoUrl")
		}
		skills = append(skills, data)
	}
	doc["skills"], _ = json.Marshal(skills)
	return json.MarshalIndent(doc, "", "  ")
}
