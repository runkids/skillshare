package contextrouter

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig(`{"topics":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RootInstructions != "AGENTS.md" || cfg.KernelCap != 8192 || cfg.TopicCap != 16384 || cfg.Strip || cfg.Compact || len(cfg.Topics) != 0 {
		t.Fatalf("defaults: %+v", cfg)
	}

	cfg, err = parseConfig(`{
	  "rootInstructions": "CLAUDE.md", "stripFrontmatter": true, "compactPathLinks": true,
	  "budgets": {"rootInstructionsMaxBytes": 10, "defaultTopicMaxBytes": 20},
	  "mounts": {"kb": {"root": "modules/kb", "config": "x.json"}, "odd": 3},
	  "topics": {
	    "b": {"description": "B", "maxBytes": 5, "sources": []},
	    "a": {"description": "A", "sources": [
	      {"path": "wiki/a.md", "heading": "H", "stripFrontmatter": false, "frontmatter": true},
	      {"path": "wiki/a.md"}]}
	  }, "unknown": [1]}`)
	if err != nil {
		t.Fatal(err)
	}
	heading, no, five := "H", false, 5
	want := routerCfg{
		RootInstructions: "CLAUDE.md", KernelCap: 10, TopicCap: 20, Strip: true, Compact: true,
		// Config order, not name order: the router prints topics as written.
		Topics: []topicCfg{
			{Name: "b", Description: "B", HasDescription: true, MaxBytes: &five},
			{Name: "a", Description: "A", HasDescription: true, Sources: []sourceCfg{
				{Path: "wiki/a.md", Heading: &heading, Strip: &no, Frontmatter: true},
				{Path: "wiki/a.md"},
			}},
		},
		Mounts: []Mount{{Name: "kb", Root: "modules/kb"}, {Name: "odd"}},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got  %+v\nwant %+v", cfg, want)
	}

	// Fields the size path does not read never invalidate the config.
	cfg, err = parseConfig(`{"okfReservedPages": true, "indexed": 3, "mounts": [], "unrouted": "x", "topics": {
	  "z": {"sources": []},
	  "m": {"description": 3, "sources": []},
	  "a": {"description": null, "sources": []},
	  "z": {"description": "last wins", "sources": []}}}`)
	if err != nil {
		t.Fatal(err)
	}
	got := []topicCfg{}
	for _, topic := range cfg.Topics {
		got = append(got, topicCfg{Name: topic.Name, Description: topic.Description, HasDescription: topic.HasDescription})
	}
	wantTopics := []topicCfg{
		{Name: "z", Description: "last wins", HasDescription: true},
		{Name: "m", HasDescription: true},
		{Name: "a", HasDescription: true},
	}
	if !reflect.DeepEqual(got, wantTopics) || len(cfg.Mounts) != 0 {
		t.Fatalf("tolerant fields: %+v mounts %+v", got, cfg.Mounts)
	}
	if cfg, err = parseConfig(`{"topics":{"t":{"sources":[]}}}`); err != nil || cfg.Topics[0].HasDescription {
		t.Fatalf("missing description: %+v %v", cfg.Topics, err)
	}

	topic := func(body string) string { return `{"topics":{"t":` + body + `}}` }
	for text, field := range map[string]string{
		`{`:                                      "invalid JSON",
		`[]`:                                     "config",
		`{}`:                                     "topics",
		`{"topics":[]}`:                          "topics",
		`{"topics":null}`:                        "topics",
		topic(`3`):                               "topics.t",
		topic(`{"description":""}`):              "topics.t.sources",
		topic(`{"description":"","sources":{}}`): "topics.t.sources",
		topic(`{"description":"","sources":["a"]}`):                               "topics.t.sources",
		topic(`{"description":"","sources":[{}]}`):                                "topics.t.sources.path",
		topic(`{"description":"","sources":[{"path":3}]}`):                        "topics.t.sources.path",
		topic(`{"description":"","sources":[{"path":"a","heading":3}]}`):          "topics.t.sources.heading",
		topic(`{"description":"","sources":[{"path":"a","stripFrontmatter":1}]}`): "topics.t.sources.stripFrontmatter",
		topic(`{"description":"","sources":[],"maxBytes":-1}`):                    "topics.t.maxBytes",
		topic(`{"description":"","sources":[],"maxBytes":1.5}`):                   "topics.t.maxBytes",
		topic(`{"description":"","sources":[],"maxBytes":true}`):                  "topics.t.maxBytes",
		`{"topics":{},"budgets":{"defaultTopicMaxBytes":"1"}}`:                    "budgets.defaultTopicMaxBytes",
		`{"topics":{},"budgets":[]}`:                                              "budgets",
		`{"topics":{},"stripFrontmatter":"yes"}`:                                  "stripFrontmatter",
		`{"topics":{},"compactPathLinks":1}`:                                      "compactPathLinks",
		`{"topics":{},"rootInstructions":1}`:                                      "rootInstructions",
	} {
		if _, err := parseConfig(text); err == nil || !strings.Contains(err.Error(), field) {
			t.Errorf("%s: got %v, want an error naming %s", text, err, field)
		}
	}
}
