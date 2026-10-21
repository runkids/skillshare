package projects

// Resource is one resource kind of a project. nil means it is not sent.
type Resource struct {
	Mode         string   `json:"mode"`
	TargetNaming string   `json:"targetNaming,omitempty"`
	Include      []string `json:"include"`
	Exclude      []string `json:"exclude"`
}

type Group struct {
	Target     string   `json:"target"` // target name, as the sync and target lists show it
	Tools      []string `json:"tools"`
	SkillsPath string   `json:"skillsPath"`
	AgentsPath string   `json:"agentsPath"`
}

type Item struct {
	Root         string    `json:"root"` // as written in config.yaml
	Path         string    `json:"path"` // absolute
	Name         string    `json:"name"`
	CustomName   string    `json:"customName,omitempty"`
	Targets      []string  `json:"targets"`
	Skills       *Resource `json:"skills"`
	Agents       *Resource `json:"agents"`
	Groups       []Group   `json:"groups"`
	Missing      bool      `json:"missing"`
	HasOwnConfig bool      `json:"hasOwnConfig"`
}
