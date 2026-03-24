package config

import "testing"

func TestValidateExtraFlatten_SymlinkRejects(t *testing.T) {
	err := ValidateExtraFlatten(true, "symlink")
	if err == nil {
		t.Error("expected error for flatten + symlink")
	}
}

func TestValidateExtraFlatten_MergeAllows(t *testing.T) {
	if err := ValidateExtraFlatten(true, "merge"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExtraFlatten_CopyAllows(t *testing.T) {
	if err := ValidateExtraFlatten(true, "copy"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExtraFlatten_FalseAlwaysAllows(t *testing.T) {
	if err := ValidateExtraFlatten(false, "symlink"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateExtraFlatten_EmptyModeAllows(t *testing.T) {
	if err := ValidateExtraFlatten(true, ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
