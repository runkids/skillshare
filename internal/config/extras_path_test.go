package config

import (
	"path/filepath"
	"testing"
)

func TestExtrasSourceDirProject(t *testing.T) {
	got := ExtrasSourceDirProject("/projects/myapp", "rules")
	want := filepath.Join("/projects/myapp", ".skillshare", "extras", "rules")
	if got != want {
		t.Errorf("ExtrasSourceDirProject() = %q, want %q", got, want)
	}
}

func TestResolveExtrasSourceDir_PerExtraSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: "/custom/rules"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	if got != "/custom/rules" {
		t.Errorf("expected /custom/rules, got %s", got)
	}
}

func TestResolveExtrasSourceDir_GlobalExtrasSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	want := filepath.Join("/global-extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_Default(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "", "/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_PerExtraOverridesAll(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: "/exact/path"}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	if got != "/exact/path" {
		t.Errorf("expected /exact/path, got %s", got)
	}
}

func TestResolveExtrasSourceDir_EmptyExtrasSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules"}
	got := ResolveExtrasSourceDir(extra, "", "/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceDir_EmptyPerExtraSource(t *testing.T) {
	extra := ExtraConfig{Name: "rules", Source: ""}
	got := ResolveExtrasSourceDir(extra, "/global-extras", "/skills")
	want := filepath.Join("/global-extras", "rules")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolveExtrasSourceType(t *testing.T) {
	tests := []struct {
		name         string
		extra        ExtraConfig
		extrasSource string
		want         string
	}{
		{"per-extra", ExtraConfig{Source: "/custom"}, "/global", "per-extra"},
		{"extras_source", ExtraConfig{}, "/global", "extras_source"},
		{"default", ExtraConfig{}, "", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveExtrasSourceType(tt.extra, tt.extrasSource)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExtrasParentDir(t *testing.T) {
	got := ExtrasParentDir("/home/user/.config/skillshare/skills")
	want := filepath.Join("/home/user/.config/skillshare", "extras")
	if got != want {
		t.Errorf("ExtrasParentDir() = %q, want %q", got, want)
	}
}
