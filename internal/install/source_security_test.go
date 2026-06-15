package install

import "testing"

// SEC003: Subdir path traversal
// Claim: GitHub URL subdir can contain .. for path traversal
// Status: FIXED — validateRepoSubdir now rejects traversal
func TestSecurity_ParseSource_SEC003_SubdirTraversal(t *testing.T) {
	traversalURLs := []struct {
		input     string
		badSubdir string
	}{
		{"github.com/owner/repo/../../etc/passwd", "../../etc/passwd"},
		{"github.com/owner/repo/../../../etc/shadow", "../../../etc/shadow"},
		{"https://github.com/owner/repo/../../etc/passwd", "../../etc/passwd"},
	}

	for _, tc := range traversalURLs {
		t.Run(tc.input, func(t *testing.T) {
			_, err := ParseSourceWithOptions(tc.input, ParseOptions{})
			if err == nil {
				t.Errorf("expected error for traversal subdir, input=%s", tc.input)
			}
			t.Logf("correctly rejected: input=%s err=%v", tc.input, err)
		})
	}
}

// SEC003: Subdir null byte
func TestSecurity_ParseSource_SEC003_SubdirNullByte(t *testing.T) {
	input := "github.com/owner/repo/path\x00/../../etc/passwd"
	_, err := ParseSourceWithOptions(input, ParseOptions{})
	if err == nil {
		t.Errorf("expected error for null byte subdir, input=%s", input)
	}
	t.Logf("correctly rejected: err=%v", err)
}

// SEC003: URL-encoded traversal
func TestSecurity_ParseSource_SEC003_SubdirEncodedTraversal(t *testing.T) {
	encodedURLs := []string{
		"github.com/owner/repo/..%2F..%2Fetc%2Fpasswd",
		"github.com/owner/repo/%2e%2e/%2e%2e/etc/passwd",
	}

	for _, input := range encodedURLs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSourceWithOptions(input, ParseOptions{})
			if err == nil {
				t.Errorf("expected error for encoded traversal, input=%s", input)
			}
			t.Logf("correctly rejected: err=%v", err)
		})
	}
}

// SEC003: Safe subdirs are still accepted
func TestSecurity_ParseSource_SEC003_AllowsSafeSubdir(t *testing.T) {
	safeCases := []struct {
		input      string
		wantSubdir string
		wantName   string
	}{
		// GitHub shorthand
		{"github.com/owner/repo/skills/frontend", "skills/frontend", "frontend"},
		// GitHub web URL with tree/branch
		{"https://github.com/user/repo/tree/main/path/to/skill", "path/to/skill", "skill"},
		// GitLab with nested groups and tree URL
		{"https://gitlab.com/group/subgroup/project/-/tree/main/skills/foo", "skills/foo", "foo"},
		// Azure DevOps HTTPS
		{"https://dev.azure.com/org/project/_git/repo/skills/bar", "skills/bar", "bar"},
		// Azure DevOps SSH (no subdir — single slash after repo is not subdir)
		{"git@ssh.dev.azure.com:v3/org/proj/repo", "", "repo"},
		// Generic HTTPS (non-GitHub, non-GitLab)
		{"https://git.example.com/owner/repo/skills/foo", "skills/foo", "foo"},
		// Generic HTTPS with subdir
		{"https://gitea.example.com/user/repo/sub/dir", "sub/dir", "dir"},
	}

	for _, tc := range safeCases {
		t.Run(tc.input, func(t *testing.T) {
			source, err := ParseSourceWithOptions(tc.input, ParseOptions{})
			if err != nil {
				t.Fatalf("unexpected error for safe subdir: %v", err)
			}
			if source.Subdir != tc.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tc.wantSubdir)
			}
			if source.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tc.wantName)
			}
		})
	}
}

// SEC003: Embedded traversal is not silently cleaned into a safe path
func TestSecurity_ParseSource_SEC003_DoesNotSilentlyCleanTraversal(t *testing.T) {
	maliciousInputs := []string{
		"github.com/owner/repo/a/../b",
		"github.com/owner/repo/skills/../../../etc",
	}

	for _, input := range maliciousInputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSourceWithOptions(input, ParseOptions{})
			if err == nil {
				t.Errorf("expected error for embedded traversal, input=%s", input)
			}
		})
	}
}

// SEC003: Double-encoded traversal
func TestSecurity_ParseSource_SEC003_DoubleEncodedTraversal(t *testing.T) {
	inputs := []string{
		"github.com/owner/repo/%252e%252e/%252e%252e/etc/passwd",
		"github.com/owner/repo/..%252F..%252Fetc%252Fpasswd",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSourceWithOptions(input, ParseOptions{})
			if err == nil {
				t.Errorf("expected error for double-encoded traversal: %s", input)
			}
			t.Logf("correctly rejected: err=%v", err)
		})
	}
}

// SEC003: Overly deep URL encoding — still unstable after max decode rounds
func TestSecurity_ParseSource_SEC003_RejectsTooDeeplyEncodedTraversal(t *testing.T) {
	inputs := []string{
		"github.com/owner/repo/%252525252e%252525252e%252525252fetc/passwd",
		"github.com/owner/repo/%2525252e%2525252e%2525252fetc/passwd",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSourceWithOptions(input, ParseOptions{})
			if err == nil {
				t.Errorf("expected error for deeply encoded traversal: %s", input)
			}
			t.Logf("correctly rejected: err=%v", err)
		})
	}
}

// SEC003: Each parser branch must reject unsafe repo subdirs.
func TestSecurity_ParseSource_SEC003_RejectsUnsafeSubdirAcrossParserBranches(t *testing.T) {
	unsafeCases := []struct {
		name  string
		input string
	}{
		{
			name:  "git ssh scp-style",
			input: "git@example.com:owner/repo.git//../etc/passwd",
		},
		{
			name:  "ssh url",
			input: "ssh://git@example.com/owner/repo.git//../etc/passwd",
		},
		{
			name:  "file url",
			input: "file:///tmp/repo//../etc/passwd",
		},
		{
			name:  "azure devops https",
			input: "https://dev.azure.com/org/project/_git/repo/../etc/passwd",
		},
		{
			name:  "azure devops ssh",
			input: "git@ssh.dev.azure.com:v3/org/project/repo//../etc/passwd",
		},
		{
			name:  "generic https git suffix",
			input: "https://git.example.com/owner/repo.git/../etc/passwd",
		},
		{
			name:  "generic https encoded traversal",
			input: "https://git.example.com/owner/repo/%2e%2e/etc/passwd",
		},
	}

	for _, tc := range unsafeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSourceWithOptions(tc.input, ParseOptions{})
			if err == nil {
				t.Fatalf("expected error for unsafe subdir, input=%s", tc.input)
			}
			t.Logf("correctly rejected: input=%s err=%v", tc.input, err)
		})
	}
}

// SEC003: Blob URLs ending in SKILL.md must not clean traversal while trimming
// the file suffix into a parent subdir.
func TestSecurity_ParseSource_SEC003_RejectsUnsafeBlobSkillPath(t *testing.T) {
	unsafeCases := []struct {
		name  string
		input string
	}{
		{
			name:  "github blob skill",
			input: "https://github.com/user/repo/blob/main/skills/foo/../../SKILL.md",
		},
		{
			name:  "gitlab blob skill",
			input: "https://gitlab.com/group/project/-/blob/main/skills/foo/../../SKILL.md",
		},
		{
			name:  "bitbucket skill path",
			input: "https://bitbucket.org/owner/repo/src/main/skills/foo/../../SKILL.md",
		},
	}

	for _, tc := range unsafeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSourceWithOptions(tc.input, ParseOptions{})
			if err == nil {
				t.Fatalf("expected error for unsafe blob skill path, input=%s", tc.input)
			}
			t.Logf("correctly rejected: input=%s err=%v", tc.input, err)
		})
	}
}

// SEC003: Versioned subdirs with dots are not affected
func TestSecurity_ParseSource_SEC003_AllowsVersionedSubdir(t *testing.T) {
	safeCases := []struct {
		input      string
		wantSubdir string
		wantName   string
	}{
		{"github.com/owner/repo/docs/v1.2.3", "docs/v1.2.3", "v1.2.3"},
		{"github.com/owner/repo/skills/frontend", "skills/frontend", "frontend"},
		{"github.com/owner/repo/a.b/c.d", "a.b/c.d", "c.d"},
	}

	for _, tc := range safeCases {
		t.Run(tc.input, func(t *testing.T) {
			source, err := ParseSourceWithOptions(tc.input, ParseOptions{})
			if err != nil {
				t.Fatalf("unexpected error for safe subdir: %v", err)
			}
			if source.Subdir != tc.wantSubdir {
				t.Errorf("Subdir = %q, want %q", source.Subdir, tc.wantSubdir)
			}
			if source.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", source.Name, tc.wantName)
			}
		})
	}
}
