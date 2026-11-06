package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestHandleListSkills_SourceLinkWarnings(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			s, source := newUpdateFollowServer(t, project, true)
			addSkill(t, source, "healthy")
			if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(source, "gone")); err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			s.handleListSkills(rr, httptest.NewRequest(http.MethodGet, "/api/resources", nil))
			var resp struct {
				Resources []skillItem `json:"resources"`
				Warnings  []string    `json:"sourceLinkWarnings"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if len(resp.Resources) != 1 || resp.Resources[0].Name != "healthy" {
				t.Fatalf("healthy inventory lost: %s", rr.Body.String())
			}
			if len(resp.Warnings) != 1 || !strings.Contains(resp.Warnings[0], "source link gone not followed: target is missing") {
				t.Fatalf("missing warning: %s", rr.Body.String())
			}
		})
	}
}

func TestHandleGetSkill_FollowedLinkRootFiles(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/follow=%t", project, enabled), func(t *testing.T) {
				s, source := newUpdateFollowServer(t, project, enabled)
				checkout := t.TempDir()
				if err := os.MkdirAll(filepath.Join(checkout, "assets"), 0755); err != nil {
					t.Fatal(err)
				}
				for name, content := range map[string]string{"SKILL.md": "# Linked root skill\n", "assets/notes.txt": "attachment\n"} {
					if err := os.WriteFile(filepath.Join(checkout, name), []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
				link := filepath.Join(source, "_dev-skills")
				if err := os.Symlink(checkout, link); err != nil {
					t.Fatal(err)
				}
				rr := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/resources/_dev-skills", nil)
				req.SetPathValue("name", "_dev-skills")
				s.handleGetSkill(rr, req)
				if !enabled {
					if rr.Code != http.StatusNotFound {
						t.Fatalf("disabled link discovered: %d %s", rr.Code, rr.Body.String())
					}
					return
				}
				var resp struct {
					Resource skillItem `json:"resource"`
					Files    []string  `json:"files"`
					Content  string    `json:"skillMdContent"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatal(err)
				}
				if rr.Code != http.StatusOK || !slices.Equal(resp.Files, []string{"SKILL.md", "assets/notes.txt"}) {
					t.Fatalf("root-of-link files missing: %d %s", rr.Code, rr.Body.String())
				}
				if resp.Resource.SourcePath != link || resp.Resource.RelPath != "_dev-skills" || resp.Content != "# Linked root skill\n" {
					t.Fatalf("logical skill response changed: %+v", resp)
				}
			})
		}
	}
}

func TestSetTargetsRefusesNestedSkillLink(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/batch=%t", project, batch), func(t *testing.T) {
				s, source := newUpdateFollowServer(t, project, true)
				checkout := t.TempDir()
				addSkill(t, checkout, "healthy")
				if err := os.Mkdir(filepath.Join(checkout, "linked-file"), 0755); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "outside.md")
				original := "---\nname: linked-file\n---\nOutside content\n"
				if err := os.WriteFile(outside, []byte(original), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(checkout, "linked-file", "SKILL.md")); err != nil {
					t.Skip(err)
				}
				if err := os.Symlink(checkout, filepath.Join(source, "linked-source")); err != nil {
					t.Skip(err)
				}
				rr := httptest.NewRecorder()
				if batch {
					s.handleBatchSetTargets(rr, httptest.NewRequest(http.MethodPost, "/api/skills/batch/targets", strings.NewReader(`{"folder":"*","target":"claude"}`)))
					var resp batchSetTargetsResponse
					if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
						t.Fatal(err)
					}
					if rr.Code != http.StatusOK || resp.Updated != 1 || len(resp.Errors) != 1 || !strings.Contains(resp.Errors[0], "is a link") {
						t.Fatalf("batch refusal missing: %d %s", rr.Code, rr.Body.String())
					}
				} else {
					req := httptest.NewRequest(http.MethodPatch, "/api/resources/linked-source__linked-file/targets", strings.NewReader(`{"target":"claude"}`))
					req.SetPathValue("name", "linked-source__linked-file")
					s.handleSetSkillTargets(rr, req)
					if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "is a link") {
						t.Fatalf("single refusal missing: %d %s", rr.Code, rr.Body.String())
					}
					rr = httptest.NewRecorder()
					req = httptest.NewRequest(http.MethodPatch, "/api/resources/linked-source__healthy/targets", strings.NewReader(`{"target":"claude"}`))
					req.SetPathValue("name", "linked-source__healthy")
					s.handleSetSkillTargets(rr, req)
					if rr.Code != http.StatusOK {
						t.Fatalf("healthy skill failed: %d %s", rr.Code, rr.Body.String())
					}
				}
				if got, err := os.ReadFile(outside); err != nil || string(got) != original {
					t.Fatalf("outside file changed: %q, %v", got, err)
				}
				got, err := os.ReadFile(filepath.Join(checkout, "healthy", "SKILL.md"))
				if err != nil || !strings.Contains(string(got), "- claude") {
					t.Fatalf("healthy targets missing: %q, %v", got, err)
				}
			})
		}
	}
}

func TestHandleAuditAll_ReportsUnfollowedSourceLink(t *testing.T) {
	s, source := newUpdateFollowServer(t, false, true)
	addSkill(t, source, "healthy")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(source, "gone")); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handleAuditAll(rr, httptest.NewRequest(http.MethodGet, "/api/audit", nil))
	var resp struct {
		Warnings   []string `json:"warnings"`
		Incomplete bool     `json:"incomplete"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Incomplete || len(resp.Warnings) != 1 || !strings.Contains(resp.Warnings[0], "source link gone not followed: target is missing") {
		t.Fatalf("partial scan reported as complete: %s", rr.Body.String())
	}
}
