package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ruleMetadataSuffix     = ".metadata.yaml"
	ruleMetadataTempPrefix = ".rule-metadata-tmp-"
)

type ruleMetadata struct {
	Targets    []string `yaml:"targets,omitempty"`
	SourceType string   `yaml:"sourceType,omitempty"`
	Disabled   bool     `yaml:"disabled,omitempty"`
}

func loadRuleMetadata(rulePath string) (ruleMetadata, error) {
	data, err := os.ReadFile(ruleMetadataPath(rulePath))
	if err != nil {
		if os.IsNotExist(err) {
			return ruleMetadata{}, nil
		}
		return ruleMetadata{}, err
	}

	var metadata ruleMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return ruleMetadata{}, fmt.Errorf("parse rule metadata %q: %w", ruleMetadataPath(rulePath), err)
	}
	return sanitizeRuleMetadata(metadata), nil
}

func saveRuleMetadata(rulePath string, metadata ruleMetadata) error {
	metadata = sanitizeRuleMetadata(metadata)
	metadataPath := ruleMetadataPath(rulePath)
	if metadata.isZero() {
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	data, err := yaml.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal rule metadata: %w", err)
	}

	dir := filepath.Dir(metadataPath)
	tempFile, err := os.CreateTemp(dir, ruleMetadataTempPrefix+"*")
	if err != nil {
		return fmt.Errorf("create metadata temp file: %w", err)
	}

	tempPath := tempFile.Name()
	closeWithCleanup := func(writeErr error) error {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return writeErr
	}

	if err := tempFile.Close(); err != nil {
		return closeWithCleanup(fmt.Errorf("close metadata temp file: %w", err))
	}
	if err := ruleWriteFile(tempPath, data, 0644); err != nil {
		return closeWithCleanup(err)
	}
	if err := replaceRuleFile(tempPath, metadataPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("rename metadata temp file: %w", err)
	}
	return nil
}

func deleteRuleMetadata(rulePath string) error {
	if err := os.Remove(ruleMetadataPath(rulePath)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ruleMetadataPath(rulePath string) string {
	base := filepath.Base(rulePath)
	return filepath.Join(filepath.Dir(rulePath), "."+base+ruleMetadataSuffix)
}

func isRuleMetadataFile(name string) bool {
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ruleMetadataSuffix)
}

func sanitizeRuleMetadata(metadata ruleMetadata) ruleMetadata {
	if len(metadata.Targets) > 0 {
		targets := make([]string, 0, len(metadata.Targets))
		for _, target := range metadata.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			targets = append(targets, target)
		}
		metadata.Targets = targets
	} else {
		metadata.Targets = nil
	}
	metadata.SourceType = strings.TrimSpace(metadata.SourceType)
	return metadata
}

func (m ruleMetadata) isZero() bool {
	return len(m.Targets) == 0 && m.SourceType == "" && !m.Disabled
}
