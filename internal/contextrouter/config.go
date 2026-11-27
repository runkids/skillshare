package contextrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// ConfigRel is where the router keeps its config inside a folder.
const ConfigRel = "wiki/ai-context.json"

// The router's defaults.
const (
	defaultRootInstructions = "AGENTS.md"
	defaultKernelCap        = 8192
	defaultTopicCap         = 16384
)

// Mount is a mounted router the config declares. It is reported as written:
// nothing under it is opened or validated.
type Mount struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

type sourceCfg struct {
	Path        string
	Heading     *string
	Strip       *bool // the source's own stripFrontmatter, when set
	Frontmatter bool
}

type topicCfg struct {
	Name        string
	Description string
	MaxBytes    *int
	Sources     []sourceCfg
}

type routerCfg struct {
	RootInstructions string
	KernelCap        int
	TopicCap         int
	Strip            bool
	Compact          bool
	Topics           []topicCfg // by name
	Mounts           []Mount    // by name
}

type object = map[string]json.RawMessage

func decodeObject(raw json.RawMessage, field string) (object, error) {
	var out object
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return nil, fmt.Errorf("%s: expected an object", field)
	}
	return out, nil
}

func decodeString(raw json.RawMessage, field string) (string, error) {
	var out *string
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return "", fmt.Errorf("%s: expected a string", field)
	}
	return *out, nil
}

func decodeBool(raw json.RawMessage, field string) (bool, error) {
	var out *bool
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return false, fmt.Errorf("%s: expected a boolean", field)
	}
	return *out, nil
}

// decodeCap accepts a nonnegative integer, as the router does.
func decodeCap(raw json.RawMessage, field string) (int, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err == nil {
		if number, ok := value.(json.Number); ok {
			if n, err := number.Int64(); err == nil && n >= 0 && n <= 1<<40 {
				return int(n), nil
			}
		}
	}
	return 0, fmt.Errorf("%s: expected a nonnegative integer", field)
}

// parseConfig decodes wiki/ai-context.json. A wrong type is an error, never a
// silent default, so a broken config is not measured as if it were another.
func parseConfig(text string) (routerCfg, error) {
	cfg := routerCfg{RootInstructions: defaultRootInstructions, KernelCap: defaultKernelCap, TopicCap: defaultTopicCap}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return cfg, fmt.Errorf("invalid JSON: %w", err)
	}
	top, err := decodeObject(raw, "config")
	if err != nil {
		return cfg, err
	}
	if v, ok := top["rootInstructions"]; ok {
		if cfg.RootInstructions, err = decodeString(v, "rootInstructions"); err != nil {
			return cfg, err
		}
	}
	if v, ok := top["stripFrontmatter"]; ok {
		if cfg.Strip, err = decodeBool(v, "stripFrontmatter"); err != nil {
			return cfg, err
		}
	}
	if v, ok := top["compactPathLinks"]; ok {
		if cfg.Compact, err = decodeBool(v, "compactPathLinks"); err != nil {
			return cfg, err
		}
	}
	if v, ok := top["budgets"]; ok {
		budgets, err := decodeObject(v, "budgets")
		if err != nil {
			return cfg, err
		}
		if b, ok := budgets["rootInstructionsMaxBytes"]; ok {
			if cfg.KernelCap, err = decodeCap(b, "budgets.rootInstructionsMaxBytes"); err != nil {
				return cfg, err
			}
		}
		if b, ok := budgets["defaultTopicMaxBytes"]; ok {
			if cfg.TopicCap, err = decodeCap(b, "budgets.defaultTopicMaxBytes"); err != nil {
				return cfg, err
			}
		}
	}

	v, ok := top["topics"]
	if !ok {
		return cfg, fmt.Errorf("topics: expected an object")
	}
	topics, err := decodeObject(v, "topics")
	if err != nil {
		return cfg, err
	}
	for name, rawTopic := range topics {
		topic, err := parseTopic(name, rawTopic)
		if err != nil {
			return cfg, err
		}
		cfg.Topics = append(cfg.Topics, topic)
	}
	sort.Slice(cfg.Topics, func(i, j int) bool { return cfg.Topics[i].Name < cfg.Topics[j].Name })

	if v, ok := top["mounts"]; ok {
		mounts, err := decodeObject(v, "mounts")
		if err != nil {
			return cfg, err
		}
		for name, rawMount := range mounts {
			mount := Mount{Name: name}
			if fields, err := decodeObject(rawMount, "mount"); err == nil {
				// Reported as written; an odd declaration is the router's to judge.
				mount.Root, _ = decodeString(fields["root"], "root")
			}
			cfg.Mounts = append(cfg.Mounts, mount)
		}
		sort.Slice(cfg.Mounts, func(i, j int) bool { return cfg.Mounts[i].Name < cfg.Mounts[j].Name })
	}
	return cfg, nil
}

func parseTopic(name string, raw json.RawMessage) (topicCfg, error) {
	field := "topics." + name
	topic := topicCfg{Name: name}
	fields, err := decodeObject(raw, field)
	if err != nil {
		return topic, err
	}
	if topic.Description, err = decodeString(fields["description"], field+".description"); err != nil {
		return topic, err
	}
	if v, ok := fields["maxBytes"]; ok {
		n, err := decodeCap(v, field+".maxBytes")
		if err != nil {
			return topic, err
		}
		topic.MaxBytes = &n
	}
	var sources []json.RawMessage
	if err := json.Unmarshal(fields["sources"], &sources); err != nil || sources == nil {
		return topic, fmt.Errorf("%s.sources: expected a list", field)
	}
	for _, rawSource := range sources {
		entry, err := decodeObject(rawSource, field+".sources")
		if err != nil {
			return topic, err
		}
		var source sourceCfg
		if source.Path, err = decodeString(entry["path"], field+".sources.path"); err != nil {
			return topic, err
		}
		if v, ok := entry["heading"]; ok {
			heading, err := decodeString(v, field+".sources.heading")
			if err != nil {
				return topic, err
			}
			source.Heading = &heading
		}
		if v, ok := entry["stripFrontmatter"]; ok {
			strip, err := decodeBool(v, field+".sources.stripFrontmatter")
			if err != nil {
				return topic, err
			}
			source.Strip = &strip
		}
		if v, ok := entry["frontmatter"]; ok {
			if source.Frontmatter, err = decodeBool(v, field+".sources.frontmatter"); err != nil {
				return topic, err
			}
		}
		topic.Sources = append(topic.Sources, source)
	}
	return topic, nil
}
