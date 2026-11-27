package plugin

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
)

type herdrDock struct {
	Place string  `json:"place"`
	Size  float64 `json:"size,omitempty"`
}

// Only an explicit dock is eligible. In particular, place:none often means
// the mod is a building block for a separate layout, not a standalone panel.
func readHerdrDock(root, path string) *herdrDock {
	raw, err := readHerdrLayoutFile(root, filepath.Join(path, "mod.json"))
	var dock herdrDock
	if err != nil || json.Unmarshal(raw, &dock) != nil || !slices.Contains([]string{"left", "right", "top", "bottom"}, dock.Place) {
		return nil
	}
	if dock.Size <= 0 {
		dock.Size = 320
	} else if dock.Size > 4096 {
		return nil
	}
	return &dock
}

func herdrDockID(mod string) string { return "skillshare-dock-" + mod }

func walkHerdrNodes(node map[string]any, visit func(map[string]any)) {
	visit(node)
	if children, ok := node["children"].([]any); ok {
		for _, child := range children {
			if child, ok := child.(map[string]any); ok {
				walkHerdrNodes(child, visit)
			}
		}
	}
}

func patchHerdrDock(raw []byte, mod string, dock herdrDock) ([]byte, error) {
	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if !json.Valid(raw) || dec.Decode(&root) != nil || root == nil || root["hidden"] == true {
		return nil, errHerdrDockShape
	}
	id := herdrDockID(mod)
	found, collision := false, false
	walkHerdrNodes(root, func(node map[string]any) {
		content, _ := node["content"].(map[string]any)
		found = found || content["mod"] == mod
		collision = collision || node["id"] == id || node["id"] == id+"-root"
	})
	if found {
		return nil, nil // Do not duplicate or take ownership of an existing placement.
	}
	children, ok := root["children"].([]any)
	if collision || !ok || len(children) == 0 || (root["type"] != "row" && root["type"] != "column") {
		return nil, errHerdrDockShape
	}
	block := map[string]any{"id": id, "type": "block", "content": map[string]string{"mod": mod}, "size": map[string]float64{"px": dock.Size}}
	axis := "row"
	if dock.Place == "top" || dock.Place == "bottom" {
		axis = "column"
	}
	if root["type"] != axis {
		// Keep window-level properties on the root and the old container's
		// identity and spacing on the branch that still holds its children.
		old := map[string]any{}
		for _, key := range []string{"id", "type", "children", "gap", "divider", "resizable"} {
			if v, ok := root[key]; ok {
				old[key] = v
				delete(root, key)
			}
		}
		root["id"], root["type"] = id+"-root", axis
		children = []any{old}
	}
	if dock.Place == "left" || dock.Place == "top" {
		root["children"] = append([]any{block}, children...)
	} else {
		root["children"] = append(children, block)
	}
	data, err := json.MarshalIndent(root, "", "  ")
	return append(data, '\n'), err
}

var (
	errHerdrDockShape = agentError{key: "plugins.error.herdrDockShape", message: "This Herdr layout cannot be copied with the requested dock. No layout was changed."}
	errHerdrDockOpen  = agentError{key: "plugins.error.herdrDockOpen", message: "Open Herdr on this computer to show the mod panel, or uncheck placement to install files only."}
)
