package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// HerdrPanelPlan describes a personal copy of the active layout. The original
// is never patched, including when it is a user layout rather than a shipped one.
type HerdrPanelPlan struct {
	From   string `json:"from"`
	Name   string `json:"name"`
	Mod    string `json:"mod,omitempty"`
	digest string
	dock   herdrDock
}

var herdrLayoutName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	errHerdrLayoutOpen  = agentError{key: "plugins.error.herdrLayoutOpen", message: "Open Herdr on this computer to add the Skillshare panel to its layout."}
	errHerdrLayoutShape = agentError{key: "plugins.error.herdrLayoutShape", message: "Skillshare could not find a unique tab bar and work area in this Herdr layout. No layout was changed."}
	errHerdrLayoutStale = agentError{key: "plugins.error.herdrLayoutStale", message: "The Herdr layout changed since preview. Preview the installation again."}
)

// Layout placement is a first-party integration, not a way to run instructions
// or apply arbitrary layout files from a plugin source.
func supportsHerdrPanel(source, name string) bool {
	link, kind := githubPluginLink(source)
	return name == "skillshare" && link != nil && strings.HasPrefix(link.Path, "/runkids/skillshare/") &&
		(kind == "tree" && strings.HasSuffix(link.Path, "/herdr/skillshare-mod") ||
			kind == "blob" && strings.HasSuffix(link.Path, "/herdr/skillshare-mod/mods/skillshare/main.js"))
}

func (s *Service) currentHerdrLayout(ctx context.Context) (string, error) {
	out, err := s.run(ctx, "herdr-app", "status")
	var status struct{ Layout string }
	if err != nil || json.Unmarshal(out, &status) != nil || !herdrLayoutName.MatchString(status.Layout) {
		return "", errHerdrLayoutOpen
	}
	return status.Layout, nil
}

func readHerdrLayout(home, name string) ([]byte, error) {
	return readHerdrLayoutFile(home, filepath.Join("layouts", name+".json"))
}

func readHerdrLayoutFile(home, rel string) ([]byte, error) {
	path := filepath.Join(home, rel)
	if err := noSymlink(home, path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errHerdrLayoutShape
	}
	return os.ReadFile(path)
}

func (s *Service) previewHerdrPanel(ctx context.Context) (*HerdrPanelPlan, error) {
	return s.previewHerdrPlacement(ctx, "", herdrDock{})
}

func (p *HerdrPanelPlan) patch(raw []byte) ([]byte, error) {
	if p.Mod != "" {
		return patchHerdrDock(raw, p.Mod, p.dock)
	}
	return patchHerdrPanel(raw)
}

func (s *Service) previewHerdrPlacement(ctx context.Context, mod string, dock herdrDock) (*HerdrPanelPlan, error) {
	home := herdrHome()
	current, err := s.currentHerdrLayout(ctx)
	if err != nil {
		if mod != "" {
			return nil, errHerdrDockOpen
		}
		return nil, err
	}
	raw, err := readHerdrLayout(home, current)
	if err != nil {
		if mod != "" {
			return nil, errHerdrDockShape
		}
		return nil, errHerdrLayoutShape
	}
	p := &HerdrPanelPlan{From: current, Mod: mod, dock: dock, digest: hash(raw)}
	patch, err := p.patch(raw)
	if err != nil || patch == nil {
		return nil, err
	}
	for i := 1; i <= 100; i++ {
		name := current + "-skillshare"
		if i > 1 {
			name = fmt.Sprintf("%s-%d", name, i)
		}
		if _, err := os.Lstat(filepath.Join(home, "layouts", name+".json")); os.IsNotExist(err) {
			p.Name = name
			return p, nil
		} else if err != nil {
			return nil, err
		}
	}
	return nil, errHerdrLayoutShape
}

func (s *Service) applyHerdrPanel(ctx context.Context, p *HerdrPanelPlan) (herdrLayoutReceipt, error) {
	var receipt herdrLayoutReceipt
	current, err := s.previewHerdrPlacement(ctx, p.Mod, p.dock)
	if err != nil || current == nil || *current != *p {
		return receipt, errHerdrLayoutStale
	}
	home, err := filepath.EvalSymlinks(herdrHome())
	if err != nil {
		return receipt, err
	}
	args, _ := json.Marshal(map[string]string{"name": p.From, "as": p.Name})
	// Herdr copies window click state, too. Do not use layout.set: it would
	// replace whichever layout happened to be selected at the time of the write.
	if _, err := s.run(ctx, "herdr-app", "do", "layout.clone", string(args)); err != nil {
		return receipt, err
	}
	active, err := s.currentHerdrLayout(ctx)
	if err != nil || active != p.Name {
		return receipt, errHerdrLayoutStale
	}
	original, err := readHerdrLayout(home, p.From)
	if err != nil || hash(original) != p.digest {
		return receipt, errHerdrLayoutStale
	}
	copy, err := readHerdrLayout(home, p.Name)
	if err != nil {
		return receipt, errHerdrLayoutShape
	}
	var clone struct {
		ClonedFrom struct{ Name string } `json:"cloned_from"`
	}
	if json.Unmarshal(copy, &clone) != nil || clone.ClonedFrom.Name != p.From {
		return receipt, errHerdrLayoutStale
	}
	patch, err := p.patch(copy)
	if err != nil {
		return receipt, err
	}
	if patch == nil {
		return receipt, errHerdrLayoutStale
	}
	clicks, _, err := herdrLayoutClicks(home, p.Name)
	if err != nil {
		return receipt, err
	}
	root, err := os.OpenRoot(filepath.Join(home, "layouts"))
	if err != nil {
		return receipt, err
	}
	defer root.Close()
	if err := rootAtomicWrite(root, p.Name+".json", patch, 0600, false); err != nil {
		return receipt, err
	}
	receipt = herdrLayoutReceipt{Home: home, From: p.From, Name: p.Name, Digest: hash(patch), Clicks: clicks, DockMod: p.Mod}
	if p.Mod != "" {
		// Load the dock before publishing the mod folder. Otherwise Herdr can
		// discover it and write its own first-seen placement into this copy.
		if active, err := s.currentHerdrLayout(ctx); err != nil || active != p.Name {
			return receipt, errHerdrLayoutStale
		}
		args, _ := json.Marshal(map[string]string{"name": p.Name})
		if _, err := s.run(ctx, "herdr-app", "do", "layout.switch", string(args)); err != nil {
			return receipt, err
		}
	}
	return receipt, nil
}

// patchHerdrPanel preserves fields we don't understand. Only the tab bar and
// the row around the active workspace are touched; user collisions are refused.
func patchHerdrPanel(raw []byte) ([]byte, error) {
	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if !json.Valid(raw) || dec.Decode(&root) != nil || root == nil {
		return nil, errHerdrLayoutShape
	}
	type position struct {
		parent, node map[string]any
		index        int
	}
	var tabs, work []position
	ids := map[string]map[string]any{}
	duplicate := false
	var walk func(map[string]any, position)
	walk = func(node map[string]any, pos position) {
		if id, ok := node["id"].(string); ok {
			if ids[id] != nil {
				duplicate = true
			}
			ids[id] = node
		}
		content, _ := node["content"].(map[string]any)
		if node["type"] == "block" && content["mod"] == "tabs" {
			tabs = append(tabs, pos)
		}
		if node["type"] == "block" && (content["tab"] == "active" || content["context"] == "active") {
			work = append(work, pos)
		}
		children, _ := node["children"].([]any)
		for i, child := range children {
			if child, ok := child.(map[string]any); ok {
				walk(child, position{node, child, i})
			}
		}
	}
	walk(root, position{})
	if duplicate {
		return nil, errHerdrLayoutShape
	}
	if ids["skillshare-toggle"] != nil || ids["skillshare-panel"] != nil || ids["skillshare"] != nil {
		toggle, _ := ids["skillshare-toggle"]["content"].(map[string]any)
		props, _ := toggle["props"].(map[string]any)
		panel, _ := ids["skillshare-panel"]["children"].([]any)
		if toggle["mod"] == "skillshare" && props["role"] == "toggle" && len(panel) == 1 {
			block, _ := panel[0].(map[string]any)
			content, _ := block["content"].(map[string]any)
			if block["id"] == "skillshare" && content["mod"] == "skillshare" {
				return nil, nil
			}
		}
		return nil, errHerdrLayoutShape
	}
	if len(tabs) != 1 || len(work) != 1 || tabs[0].parent == nil || work[0].parent["type"] != "row" || tabs[0].node["hidden"] == true {
		return nil, errHerdrLayoutShape
	}
	tab := tabs[0]
	toggle := map[string]any{"id": "skillshare-toggle", "type": "block", "content": map[string]any{"mod": "skillshare", "props": map[string]any{"role": "toggle"}}, "size": map[string]int{"px": 118}}
	switch tab.parent["type"] {
	case "row":
		children := tab.parent["children"].([]any)
		tab.parent["children"] = append(append(children[:tab.index+1:tab.index+1], toggle), children[tab.index+1:]...)
	case "column":
		if ids["skillshare-tab-row"] != nil {
			return nil, errHerdrLayoutShape
		}
		row := map[string]any{"id": "skillshare-tab-row", "type": "row", "children": []any{tab.node, toggle}, "resizable": false}
		for _, key := range []string{"size", "min", "max"} {
			if value, ok := tab.node[key]; ok {
				row[key] = value
				delete(tab.node, key)
			}
		}
		tab.parent["children"].([]any)[tab.index] = row
	default:
		return nil, errHerdrLayoutShape
	}
	body := work[0].parent
	body["children"] = append(body["children"].([]any), map[string]any{
		"id": "skillshare-panel", "type": "column", "size": map[string]int{"px": 400}, "min": 380, "max": 720,
		"children": []any{map[string]any{"id": "skillshare", "type": "block", "content": map[string]any{"mod": "skillshare"}}},
	})
	return json.MarshalIndent(root, "", "  ")
}
