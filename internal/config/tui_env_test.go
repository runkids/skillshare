package config

import (
	"testing"
)

func TestIsTUIEnabled_Default(t *testing.T) {
	cfg := &Config{}
	if !cfg.IsTUIEnabled() {
		t.Error("expected TUI enabled by default when TUI field is nil")
	}
}

func TestIsTUIEnabled_ConfigTrue(t *testing.T) {
	b := true
	cfg := &Config{TUI: &b}
	if !cfg.IsTUIEnabled() {
		t.Error("expected TUI enabled when config is true")
	}
}

func TestIsTUIEnabled_ConfigFalse(t *testing.T) {
	b := false
	cfg := &Config{TUI: &b}
	if cfg.IsTUIEnabled() {
		t.Error("expected TUI disabled when config is false")
	}
}

func TestIsTUIEnabled_EnvOverridesConfigTrue(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "1")
	b := true
	cfg := &Config{TUI: &b}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI=1 to disable TUI even when config is true")
	}
}

func TestIsTUIEnabled_EnvOverridesConfigNil(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "1")
	cfg := &Config{}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI=1 to disable TUI with nil config")
	}
}

func TestIsTUIEnabled_EnvValueTrue(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "true")
	cfg := &Config{}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI=true to disable TUI")
	}
}

func TestIsTUIEnabled_EnvValueYes(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "yes")
	cfg := &Config{}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI=yes to disable TUI")
	}
}

func TestIsTUIEnabled_EnvCaseInsensitive(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "TRUE")
	cfg := &Config{}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI=TRUE (uppercase) to disable TUI")
	}
}

func TestIsTUIEnabled_EnvWhitespace(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "  true  ")
	cfg := &Config{}
	if cfg.IsTUIEnabled() {
		t.Error("expected SKILLSHARE_NO_TUI with whitespace to disable TUI")
	}
}

func TestIsTUIEnabled_EnvInvalidValue(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "nope")
	cfg := &Config{}
	if !cfg.IsTUIEnabled() {
		t.Error("expected invalid env value to fall through to config default")
	}
}

func TestIsTUIEnabled_EnvEmptyString(t *testing.T) {
	t.Setenv("SKILLSHARE_NO_TUI", "")
	cfg := &Config{}
	if !cfg.IsTUIEnabled() {
		t.Error("expected empty env value to fall through to config default")
	}
}
