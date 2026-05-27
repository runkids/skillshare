package sync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExtensionSpec describes a resolved transform extension applied to a target.
type ExtensionSpec struct {
	Run       []string // explicit argv, e.g. ["python", "convert.py"]
	OutputExt string   // output extension without dot, e.g. "toml"; empty = keep source ext
	Dir       string   // working directory for the subprocess
	Name      string   // display / oplog name
	Source    string   // resolved exec path (file or dir), for trust pinning
}

// extensionManifest mirrors the on-disk extension.yaml shape.
type extensionManifest struct {
	Run         []string `yaml:"run"`
	OutputExt   string   `yaml:"output_ext"`
	Description string   `yaml:"description"`
}

// LoadExtensionSpec resolves an extension at execPath into a spec. execPath may be
// a single executable file or a directory containing extension.yaml.
func LoadExtensionSpec(execPath, name string) (*ExtensionSpec, error) {
	info, err := os.Stat(execPath)
	if err != nil {
		return nil, fmt.Errorf("extension %q not found at %s: %w", name, execPath, err)
	}
	if info.IsDir() {
		manifestPath := filepath.Join(execPath, "extension.yaml")
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return nil, fmt.Errorf("extension %q: cannot read %s: %w", name, manifestPath, readErr)
		}
		var m extensionManifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("extension %q: invalid extension.yaml: %w", name, err)
		}
		if len(m.Run) == 0 {
			return nil, fmt.Errorf("extension %q: extension.yaml is missing 'run'", name)
		}
		return &ExtensionSpec{
			Run:       m.Run,
			OutputExt: strings.TrimPrefix(m.OutputExt, "."),
			Dir:       execPath,
			Name:      name,
			Source:    execPath,
		}, nil
	}
	// Single-file executable: exec directly (shebang on Unix); keep source extension.
	return &ExtensionSpec{
		Run:       []string{execPath},
		OutputExt: "",
		Dir:       filepath.Dir(execPath),
		Name:      name,
		Source:    execPath,
	}, nil
}

// applyOutputExt replaces rel's extension with outputExt (no leading dot).
// An empty outputExt keeps the original extension.
func applyOutputExt(rel, outputExt string) string {
	if outputExt == "" {
		return rel
	}
	return strings.TrimSuffix(rel, filepath.Ext(rel)) + "." + outputExt
}

// runExtensionFile pipes srcFile's content through the extension subprocess and
// writes the stdout to tgtFile. A non-zero exit code is returned as an error.
func runExtensionFile(spec *ExtensionSpec, srcFile, tgtFile string, env map[string]string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(tgtFile), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	cmd := exec.Command(spec.Run[0], spec.Run[1:]...)
	cmd.Dir = spec.Dir
	cmd.Stdin = src
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("extension %q failed: %s", spec.Name, msg)
		}
		return fmt.Errorf("extension %q failed: %w", spec.Name, err)
	}
	if err := os.WriteFile(tgtFile, stdout.Bytes(), 0644); err != nil {
		return fmt.Errorf("write target: %w", err)
	}
	return nil
}
