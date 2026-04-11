package main

import "testing"

func TestParseResourceFlags(t *testing.T) {
	defaultSelection := resourceSelection{skills: true}

	tests := []struct {
		name      string
		args      []string
		opts      resourceFlagOptions
		want      resourceSelection
		wantRest  []string
		wantError string
	}{
		{
			name:     "defaults to skills only",
			args:     []string{"--dry-run"},
			opts:     resourceFlagOptions{defaultSelection: defaultSelection},
			want:     resourceSelection{skills: true},
			wantRest: []string{"--dry-run"},
		},
		{
			name:     "parses explicit resources",
			args:     []string{"--resources", "rules,hooks", "--force"},
			opts:     resourceFlagOptions{defaultSelection: defaultSelection},
			want:     resourceSelection{rules: true, hooks: true},
			wantRest: []string{"--force"},
		},
		{
			name:     "parses repeated resources case insensitively",
			args:     []string{"--resources", "Rules", "--resources", "hooks"},
			opts:     resourceFlagOptions{defaultSelection: defaultSelection},
			want:     resourceSelection{rules: true, hooks: true},
			wantRest: []string{},
		},
		{
			name:     "sync all selects every resource",
			args:     []string{"--all", "--dry-run"},
			opts:     resourceFlagOptions{defaultSelection: defaultSelection, allowAll: true},
			want:     resourceSelection{skills: true, rules: true, hooks: true},
			wantRest: []string{"--dry-run"},
		},
		{
			name:      "rejects unknown resource",
			args:      []string{"--resources", "rules,unknown"},
			opts:      resourceFlagOptions{defaultSelection: defaultSelection},
			wantError: `unsupported resource "unknown"`,
		},
		{
			name:      "rejects missing resources value",
			args:      []string{"--resources"},
			opts:      resourceFlagOptions{defaultSelection: defaultSelection},
			wantError: "--resources requires a comma-separated value",
		},
		{
			name:      "rejects conflicting all and resources",
			args:      []string{"--all", "--resources", "skills"},
			opts:      resourceFlagOptions{defaultSelection: defaultSelection, allowAll: true},
			wantError: "--all and --resources cannot be used together",
		},
		{
			name:     "collect parser leaves target all flag alone",
			args:     []string{"--all", "--resources", "rules"},
			opts:     resourceFlagOptions{defaultSelection: defaultSelection},
			want:     resourceSelection{rules: true},
			wantRest: []string{"--all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rest, err := parseResourceFlags(tt.args, tt.opts)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantError)
				}
				if err.Error() != tt.wantError {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseResourceFlags() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("selection = %#v, want %#v", got, tt.want)
			}
			assertStringSlice(t, "rest", rest, tt.wantRest)
		})
	}
}
