package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"skillshare/internal/check/checktest"
)

// checkPayload is the body of GET /api/check and of the stream's done event.
type checkPayload struct {
	TrackedRepos []repoCheckResult  `json:"tracked_repos"`
	LinkedRepos  []any              `json:"linked_repos"`
	Skills       []skillCheckResult `json:"skills"`
}

func decodeCheckPayload(t *testing.T, raw string) checkPayload {
	t.Helper()
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		t.Fatalf("invalid payload: %v: %s", err, raw)
	}
	for _, key := range []string{"tracked_repos", "linked_repos", "skills"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("payload lacks %q: %s", key, raw)
		}
	}
	if len(keys) != 3 {
		t.Errorf("payload has unexpected keys: %s", raw)
	}
	var payload checkPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

// wantDashboardSkills is what the dashboard reports for checktest.Seed. Unlike
// the CLI it gives only name, status and message for a skill without a remote.
func wantDashboardSkills(versions map[string]string) []skillCheckResult {
	statuses := map[string]string{
		"broken":  "error",
		"changed": "update_available",
		"current": "up_to_date",
		"doomed":  "stale",
		"notree":  "update_available",
		"same":    "up_to_date",
		"slash":   "up_to_date",
	}
	want := []skillCheckResult{{Name: "plain", Status: "local"}}
	for name, status := range statuses {
		r := skillCheckResult{Name: name, Source: "src/" + name, Version: versions[name], Status: status}
		if name == "current" {
			r.InstalledAt = "2024-05-06"
		}
		want = append(want, r)
	}
	sortSkillResults(want)
	return want
}

func sortSkillResults(results []skillCheckResult) {
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
}

func TestHandleCheck_ResolvesEveryStatus(t *testing.T) {
	s, src := newTestServer(t)
	versions := checktest.Seed(t, t.TempDir(), src)
	s.reloadSkillsStore()

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/check", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	payload := decodeCheckPayload(t, rr.Body.String())
	if payload.Skills[0].Name != "plain" {
		t.Errorf("skills without a remote must come first, got %+v", payload.Skills[0])
	}
	sortSkillResults(payload.Skills)
	if want := wantDashboardSkills(versions); !reflect.DeepEqual(payload.Skills, want) {
		t.Errorf("skills:\n got %+v\nwant %+v", payload.Skills, want)
	}
	if len(payload.TrackedRepos) != 0 || len(payload.LinkedRepos) != 0 {
		t.Errorf("unexpected repos: %s", rr.Body.String())
	}
}

func TestHandleCheckStream_EventSequence(t *testing.T) {
	s, src := newTestServer(t)
	versions := checktest.Seed(t, t.TempDir(), src)
	s.reloadSkillsStore()

	rr := httptest.NewRecorder()
	s.handleCheckStream(rr, httptest.NewRequest(http.MethodGet, "/api/check/stream", nil))

	type event struct{ name, data string }
	var events []event
	for _, block := range strings.Split(strings.TrimSpace(rr.Body.String()), "\n\n") {
		name, data, ok := strings.Cut(block, "\n")
		if !ok {
			t.Fatalf("malformed event %q", block)
		}
		events = append(events, event{strings.TrimPrefix(name, "event: "), strings.TrimPrefix(data, "data: ")})
	}
	if len(events) < 3 {
		t.Fatalf("expected at least discovering, start and done, got %+v", events)
	}

	if want := (event{"discovering", `{"phase":"scanning source directory"}`}); events[0] != want {
		t.Errorf("first event = %+v, want %+v", events[0], want)
	}
	// Three remotes: the moved one, the pinned branch, and the unreachable one.
	if want := (event{"start", `{"repos":0,"sources":3,"total":3}`}); events[1] != want {
		t.Errorf("second event = %+v, want %+v", events[1], want)
	}
	for _, e := range events[2 : len(events)-1] {
		var progress map[string]int64
		if e.name != "progress" || json.Unmarshal([]byte(e.data), &progress) != nil || len(progress) != 1 || progress["checked"] > 3 {
			t.Errorf("unexpected event between start and done: %+v", e)
		}
	}

	done := events[len(events)-1]
	if done.name != "done" {
		t.Fatalf("last event = %+v, want done", done)
	}
	payload := decodeCheckPayload(t, done.data)
	sortSkillResults(payload.Skills)
	if want := wantDashboardSkills(versions); !reflect.DeepEqual(payload.Skills, want) {
		t.Errorf("skills:\n got %+v\nwant %+v", payload.Skills, want)
	}
}

func TestHandleCheckStream_NamesLimitTheCheck(t *testing.T) {
	s, src := newTestServer(t)
	checktest.Seed(t, t.TempDir(), src)
	addTrackedRepoWithBrokenIndex(t, src, "_team")
	addTrackedRepoWithBrokenIndex(t, src, "_other")
	s.reloadSkillsStore()

	rr := httptest.NewRecorder()
	s.handleCheckStream(rr, httptest.NewRequest(http.MethodGet, "/api/check/stream?names=_team,changed", nil))

	blocks := strings.Split(strings.TrimSpace(rr.Body.String()), "\n\n")
	if len(blocks) < 3 {
		t.Fatalf("expected at least discovering, start and done, got %q", rr.Body.String())
	}
	if want := "event: start\ndata: " + `{"repos":1,"sources":1,"total":2}`; blocks[1] != want {
		t.Errorf("second event = %q, want %q", blocks[1], want)
	}
	_, done, ok := strings.Cut(blocks[len(blocks)-1], "event: done\ndata: ")
	if !ok {
		t.Fatalf("last event = %q, want done", blocks[len(blocks)-1])
	}
	payload := decodeCheckPayload(t, done)
	if len(payload.TrackedRepos) != 1 || payload.TrackedRepos[0].Name != "_team" {
		t.Errorf("tracked repos = %+v, want only _team", payload.TrackedRepos)
	}
	if len(payload.Skills) != 1 || payload.Skills[0].Name != "changed" {
		t.Errorf("skills = %+v, want only changed", payload.Skills)
	}
}
