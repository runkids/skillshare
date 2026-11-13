package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/utils"
)

func TestExtrasMutators_TakeRootLockForRemote(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, mutation := range []string{"mode", "add-target", "remove-target", "edit-target", "edit"} {
			t.Run(mutation+map[bool]string{false: "/local", true: "/remote"}[remote], func(t *testing.T) {
				target := t.TempDir()
				extra := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: target}, {Path: t.TempDir()}}}
				if remote {
					extra.Remote = "example-org/ai-hub"
					extra.Branch = "main"
				}
				s, _ := newTestServerWithExtras(t, []config.ExtraConfig{extra}, "")
				called := 0
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(string) (func(), error) { called++; return func() {}, nil }
				defer func() { lockRemoteExtraRoot = old }()
				method, path, body := http.MethodPost, "/api/extras/rules/mode", `{"target":"`+target+`","mode":"copy"}`
				switch mutation {
				case "add-target":
					path = "/api/extras/rules/targets"
					body = `{"path":"` + t.TempDir() + `"}`
				case "remove-target":
					method = http.MethodDelete
					path = "/api/extras/rules/targets"
					body = `{"path":"` + target + `"}`
				case "edit-target":
					method = http.MethodPut
					path = "/api/extras/rules/targets"
					body = `{"path":"` + target + `","target":{"path":"` + target + `","mode":"copy"}}`
				case "edit":
					method = http.MethodPatch
					path = "/api/extras/rules"
					body = `{}`
				}
				rr := serveExtras(s, method, path, body)
				if rr.Code != http.StatusOK {
					t.Fatalf("%d %s", rr.Code, rr.Body.String())
				}
				want := 0
				if remote {
					want = 1
				}
				if called != want {
					t.Fatalf("locks %d want %d", called, want)
				}
				fresh, err := config.LoadReadOnly()
				if err != nil || fresh.Extras[0].Remote != extra.Remote || fresh.Extras[0].Branch != extra.Branch {
					t.Fatalf("origin lost: %+v %v", fresh, err)
				}
			})
		}
	}
}

func TestExtrasEdit_RemoteRenameAndAnySourceRefused(t *testing.T) {
	for _, body := range []string{`{"name":"renamed"}`, `{"source":""}`, `{"source":"elsewhere"}`} {
		s, _ := newTestServerWithExtras(t, []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{}}}, "")
		before, err := os.ReadFile(s.configPath())
		if err != nil {
			t.Fatal(err)
		}
		rr := serveExtras(s, http.MethodPatch, "/api/extras/rules", body)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "remove rules") || !strings.Contains(rr.Body.String(), "--kind extra") {
			t.Fatalf("rename/source refusal %d: %s", rr.Code, rr.Body.String())
		}
		after, err := os.ReadFile(s.configPath())
		if err != nil || string(after) != string(before) {
			t.Fatal("refusal changed config")
		}
	}
}

func TestExtrasDelete_RemoteSharedRemoval(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, edited := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/edited=%t", project, edited), func(t *testing.T) {
				extras := []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{}}}
				var s *Server
				if project {
					s, _ = newTestProjectServerWithExtras(t, extras)
				} else {
					s, _ = newTestServerWithExtras(t, extras, "")
				}
				root := s.cfg.EffectiveExtrasSource()
				if project {
					root = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
				}
				source := filepath.Join(root, "rules")
				file := filepath.Join(source, "rule.md")
				if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
					t.Fatal(err)
				}
				hash, err := utils.FileHashFormatted(file)
				if err != nil {
					t.Fatal(err)
				}
				store := install.NewMetadataStore()
				store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40), FileHashes: map[string]string{"rule.md": hash}})
				if err := store.Save(root); err != nil {
					t.Fatal(err)
				}
				if edited {
					if err := os.WriteFile(file, []byte("edit"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				rr := serveExtras(s, http.MethodDelete, "/api/extras/rules", "")
				if rr.Code != http.StatusOK {
					t.Fatalf("delete %d: %s", rr.Code, rr.Body.String())
				}
				var out struct {
					DeletedDir     bool `json:"deleted_dir"`
					KeptEditedDir  bool `json:"kept_edited_dir"`
					Remote, Commit string
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.DeletedDir == edited || out.KeptEditedDir != edited || out.Remote != "example-org/ai-hub" {
					t.Fatalf("response %+v %v", out, err)
				}
				_, err = os.Stat(source)
				if edited && err != nil || !edited && !os.IsNotExist(err) {
					t.Fatalf("directory %v", err)
				}
			})
		}
	}
}

func TestExtrasMutation_ReloadsAfterRootLock(t *testing.T) {
	s, _ := newTestServerWithExtras(t, []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub"}}, "")
	old := lockRemoteExtraRoot
	lockRemoteExtraRoot = func(string) (func(), error) {
		fresh, err := config.LoadReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		fresh.Extras[0].Branch = "published"
		fresh.Extras = append(fresh.Extras, config.ExtraConfig{Name: "concurrent"})
		if err := fresh.Save(); err != nil {
			t.Fatal(err)
		}
		return func() {}, nil
	}
	defer func() { lockRemoteExtraRoot = old }()
	rr := serveExtras(s, http.MethodPost, "/api/extras/rules/targets", `{"path":"`+t.TempDir()+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d: %s", rr.Code, rr.Body.String())
	}
	fresh, err := config.LoadReadOnly()
	if err != nil || len(fresh.Extras) != 2 || fresh.Extras[0].Branch != "published" || len(fresh.Extras[0].Targets) != 1 {
		t.Fatalf("stale save: %+v %v", fresh, err)
	}
}

func TestExtrasMutators_InterruptedRefused(t *testing.T) {
	s, _ := newTestServerWithExtras(t, []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub"}}, "")
	stage := filepath.Join(s.cfg.EffectiveExtrasSource(), ".skillshare-staging")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.configPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		rr := serveExtras(s, method, "/api/extras/rules", `{}`)
		if rr.Code < 400 || !strings.Contains(rr.Body.String(), "interrupted") {
			t.Fatalf("%s: %d %s", method, rr.Code, rr.Body.String())
		}
	}
	after, err := os.ReadFile(s.configPath())
	if err != nil || string(after) != string(before) {
		t.Fatal("interrupted mutation changed config")
	}
}

func TestExtrasList_RemoteFieldsAndZeroTargets(t *testing.T) {
	for _, project := range []bool{false, true} {
		extras := []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Branch: "main", Targets: []config.ExtraTargetConfig{}}}
		var s *Server
		if project {
			s, _ = newTestProjectServerWithExtras(t, extras)
		} else {
			s, _ = newTestServerWithExtras(t, extras, "")
		}
		root := s.cfg.EffectiveExtrasSource()
		if project {
			root = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
		}
		stage := filepath.Join(root, ".skillshare-staging")
		if err := os.MkdirAll(stage, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte(`{}`), 0644); err != nil {
			t.Fatal(err)
		}
		rr := serveExtras(s, http.MethodGet, "/api/extras", "")
		var entries []struct {
			Remote, Branch, State string
			Targets               []any
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &entries); rr.Code != 200 || err != nil || len(entries) != 1 || entries[0].Remote != extras[0].Remote || entries[0].Branch != "main" || entries[0].State != "interrupted" || entries[0].Targets == nil {
			t.Fatalf("list %d: %s %v", rr.Code, rr.Body, err)
		}
	}
}
