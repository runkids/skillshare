package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseRunbook_BasicTwoStep(t *testing.T) {
	input := `# My Test Runbook

## Scope

- Test scope item

## Environment

Docker container.

## Steps

### Step 1: Create directory

` + "```bash" + `
mkdir -p /tmp/test
` + "```" + `

Expected:

- Directory created
- No errors

### Step 2: Verify directory

` + "```bash" + `
ls /tmp/test
` + "```" + `

Expected:

- Directory exists

## Pass Criteria

- All steps pass
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if rb.Meta.Title != "My Test Runbook" {
		t.Errorf("Title = %q, want %q", rb.Meta.Title, "My Test Runbook")
	}
	if rb.Meta.Scope == "" {
		t.Error("Scope should not be empty")
	}
	if rb.Meta.Env == "" {
		t.Error("Env should not be empty")
	}

	if len(rb.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(rb.Steps))
	}

	s1 := rb.Steps[0]
	if s1.Number != 1 {
		t.Errorf("Step 1 Number = %d, want 1", s1.Number)
	}
	if s1.Title != "Create directory" {
		t.Errorf("Step 1 Title = %q, want %q", s1.Title, "Create directory")
	}
	if s1.Command != "mkdir -p /tmp/test" {
		t.Errorf("Step 1 Command = %q, want %q", s1.Command, "mkdir -p /tmp/test")
	}
	if s1.Lang != "bash" {
		t.Errorf("Step 1 Lang = %q, want %q", s1.Lang, "bash")
	}
	if len(s1.Expected) != 2 {
		t.Fatalf("Step 1 Expected len = %d, want 2", len(s1.Expected))
	}
	if s1.Expected[0] != "Directory created" {
		t.Errorf("Step 1 Expected[0] = %q", s1.Expected[0])
	}

	s2 := rb.Steps[1]
	if s2.Number != 2 {
		t.Errorf("Step 2 Number = %d, want 2", s2.Number)
	}
	if len(s2.Expected) != 1 {
		t.Fatalf("Step 2 Expected len = %d, want 1", len(s2.Expected))
	}
}

func TestParseRunbook_NumberedFormat(t *testing.T) {
	input := `# Numbered Runbook

## Scope

Test.

## Environment

Local.

## Steps

### 1. Setup environment

` + "```bash" + `
echo hello
` + "```" + `

Expected:

- Prints hello

### 2. Check result

` + "```bash" + `
echo done
` + "```" + `

Expected:

- Prints done
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(rb.Steps))
	}
	if rb.Steps[0].Number != 1 {
		t.Errorf("Step Number = %d, want 1", rb.Steps[0].Number)
	}
	if rb.Steps[0].Title != "Setup environment" {
		t.Errorf("Step Title = %q, want %q", rb.Steps[0].Title, "Setup environment")
	}
	if rb.Steps[1].Number != 2 {
		t.Errorf("Step 2 Number = %d, want 2", rb.Steps[1].Number)
	}
}

func TestParseRunbook_MultiCodeBlocks(t *testing.T) {
	input := `# Multi Code Block Runbook

## Scope

Test.

## Steps

### Step 1: Run and verify

` + "```bash" + `
echo "first"
` + "```" + `

Verify:

` + "```bash" + `
echo "second"
` + "```" + `

Expected:

- first printed
- second printed
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1 (merged)", len(rb.Steps))
	}

	s := rb.Steps[0]
	if !strings.Contains(s.Command, "first") || !strings.Contains(s.Command, "second") {
		t.Errorf("Command should contain both code blocks, got: %q", s.Command)
	}
	if len(s.Expected) != 2 {
		t.Fatalf("Expected len = %d, want 2", len(s.Expected))
	}
}

func TestParseRunbook_Step0(t *testing.T) {
	input := `# Step Zero Runbook

## Scope

Test.

## Step 0: Verify entrypoint

` + "```bash" + `
which ss
` + "```" + `

Expected:

- ss found

## Step 1: Do work

` + "```bash" + `
echo work
` + "```" + `

Expected:

- work done
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(rb.Steps))
	}
	if rb.Steps[0].Number != 0 {
		t.Errorf("First step Number = %d, want 0", rb.Steps[0].Number)
	}
	if rb.Steps[0].Title != "Verify entrypoint" {
		t.Errorf("First step Title = %q", rb.Steps[0].Title)
	}
	if rb.Steps[1].Number != 1 {
		t.Errorf("Second step Number = %d, want 1", rb.Steps[1].Number)
	}
}

func TestParseRunbook_BoldExpected(t *testing.T) {
	input := `# Bold Expected Runbook

## Scope

Test.

## Steps

### Step 1: Check something

` + "```bash" + `
echo ok
` + "```" + `

**Expected:**

- ` + "`result`" + ` is **valid**
- No errors
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}

	s := rb.Steps[0]
	if len(s.Expected) != 2 {
		t.Fatalf("Expected len = %d, want 2", len(s.Expected))
	}
	// Inline markdown should be stripped
	if strings.Contains(s.Expected[0], "`") {
		t.Errorf("Expected[0] should strip backticks, got: %q", s.Expected[0])
	}
	if strings.Contains(s.Expected[0], "**") {
		t.Errorf("Expected[0] should strip bold markers, got: %q", s.Expected[0])
	}
}

func TestParseRunbook_StopAtPassCriteria(t *testing.T) {
	input := `# Runbook

## Scope

Test.

### Step 1: Do something

` + "```bash" + `
echo hello
` + "```" + `

## Pass Criteria

- Everything passes

### Step 99: This should not be parsed

` + "```bash" + `
echo should_not_appear
` + "```" + `
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1 (should stop at Pass Criteria)", len(rb.Steps))
	}
}

func TestParseRunbook_SkipOptionalSection(t *testing.T) {
	input := `# Runbook

## Scope

Test scope.

## Optional: use ssenv

` + "```bash" + `
ssenv create demo
` + "```" + `

## Step 1: Real step

` + "```bash" + `
echo real
` + "```" + `

Expected:

- real printed
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1 (Optional section should be skipped)", len(rb.Steps))
	}
	if rb.Steps[0].Title != "Real step" {
		t.Errorf("Step Title = %q, want %q", rb.Steps[0].Title, "Real step")
	}
}

func TestParseRunbook_RealFile_FirstUseCLI(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "ai_docs", "tests", "first_use_cli_e2e_runbook.md"))
	if err != nil {
		t.Skipf("Skipping real file test: %v", err)
	}
	defer f.Close()

	rb, err := ParseRunbook(f)
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if rb.Meta.Title == "" {
		t.Error("Title should not be empty")
	}
	t.Logf("Title: %s", rb.Meta.Title)
	t.Logf("Scope: %s", rb.Meta.Scope)
	t.Logf("Steps: %d", len(rb.Steps))

	if len(rb.Steps) < 5 {
		t.Errorf("Expected at least 5 steps, got %d", len(rb.Steps))
	}

	// Step 0 should exist
	if rb.Steps[0].Number != 0 {
		t.Errorf("First step should be Step 0, got %d", rb.Steps[0].Number)
	}

	// All steps should have commands
	for i, s := range rb.Steps {
		if s.Command == "" {
			t.Errorf("Step %d (%s) has no command", i, s.Title)
		}
		t.Logf("  Step %d: %s (cmd=%d bytes, expected=%d)",
			s.Number, s.Title, len(s.Command), len(s.Expected))
	}
}

func TestParseRunbook_AllRealRunbooks(t *testing.T) {
	dir := filepath.Join("..", "..", "ai_docs", "tests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("Skipping: cannot read %s: %v", dir, err)
	}

	var runbooks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_runbook.md") {
			runbooks = append(runbooks, e.Name())
		}
	}

	if len(runbooks) == 0 {
		t.Skip("No runbook files found")
	}

	t.Logf("Found %d runbook files", len(runbooks))

	for _, name := range runbooks {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()

			rb, err := ParseRunbook(f)
			if err != nil {
				t.Fatalf("ParseRunbook error: %v", err)
			}

			if rb.Meta.Title == "" {
				t.Error("Title should not be empty")
			}

			if len(rb.Steps) == 0 {
				t.Error("Should have at least 1 step")
			}

			hasCommand := false
			for _, s := range rb.Steps {
				if s.Command != "" {
					hasCommand = true
					break
				}
			}
			if !hasCommand {
				t.Error("At least one step should have a command")
			}

			t.Logf("  Title: %s", rb.Meta.Title)
			t.Logf("  Steps: %d", len(rb.Steps))
			for _, s := range rb.Steps {
				t.Logf("    Step %d: %s (cmd=%d bytes, expected=%d)",
					s.Number, s.Title, len(s.Command), len(s.Expected))
			}
		})
	}
}

func TestParseRunbook_DoubleLabeling(t *testing.T) {
	input := `# Double Label Runbook

## Scope

Test.

### 1. Step 1: Do something fancy

` + "```bash" + `
echo fancy
` + "```" + `

Expected:

- fancy printed
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	if rb.Steps[0].Number != 1 {
		t.Errorf("Number = %d, want 1", rb.Steps[0].Number)
	}
	if rb.Steps[0].Title != "Do something fancy" {
		t.Errorf("Title = %q, want %q", rb.Steps[0].Title, "Do something fancy")
	}
}

func TestParseRunbook_LetterSuffix(t *testing.T) {
	input := `# Letter Suffix Runbook

## Scope

Test.

### 1b. Variant step

` + "```bash" + `
echo variant
` + "```" + `

Expected:

- variant printed
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	if rb.Steps[0].Number != 1 {
		t.Errorf("Number = %d, want 1", rb.Steps[0].Number)
	}
	if rb.Steps[0].Title != "Variant step" {
		t.Errorf("Title = %q, want %q", rb.Steps[0].Title, "Variant step")
	}
}

func TestParseRunbook_ExpectedBoldVariant(t *testing.T) {
	input := `# Bold Variant Runbook

## Scope

Test.

### Step 1: Check

` + "```bash" + `
echo ok
` + "```" + `

**Expected**:

- All good
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	if len(rb.Steps[0].Expected) != 1 {
		t.Fatalf("Expected len = %d, want 1", len(rb.Steps[0].Expected))
	}
	if rb.Steps[0].Expected[0] != "All good" {
		t.Errorf("Expected[0] = %q, want %q", rb.Steps[0].Expected[0], "All good")
	}
}

func TestParseRunbook_DescriptionBetweenHeadingAndCode(t *testing.T) {
	input := `# Description Runbook

## Scope

Test.

### Step 1: Do something

Change only cursor to copy mode (leave global/default as-is).

` + "```bash" + `
echo something
` + "```" + `

Expected:

- Something done
`
	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	if rb.Steps[0].Description == "" {
		t.Error("Description should capture text between heading and code block")
	}
	t.Logf("Description: %q", rb.Steps[0].Description)
}

func TestParseRunbook_HeredocWithEmbeddedCodeFence(t *testing.T) {
	// A heredoc containing ``` should NOT terminate the code block.
	input := "# Heredoc Runbook\n\n## Scope\n\nTest.\n\n### Step 1: Create script\n\n" +
		"```bash\n" +
		"cat << 'EOF' > /tmp/test.md\n" +
		"# My Document\n" +
		"```bash\n" +
		"echo hello\n" +
		"```\n" +
		"EOF\n" +
		"echo done\n" +
		"```\n" +
		"\nExpected:\n\n- done\n"

	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	cmd := rb.Steps[0].Command
	if !strings.Contains(cmd, "cat << 'EOF'") {
		t.Error("command should contain the heredoc start")
	}
	if !strings.Contains(cmd, "echo done") {
		t.Errorf("command should include 'echo done' after heredoc, got:\n%s", cmd)
	}
	if !strings.Contains(cmd, "echo hello") {
		t.Errorf("command should include heredoc body with 'echo hello', got:\n%s", cmd)
	}
}

func TestParseRunbook_HeredocDoubleQuoted(t *testing.T) {
	// <<-"DELIM" variant.
	input := "# Heredoc Runbook\n\n### Step 1: Test\n\n" +
		"```bash\n" +
		"cat <<-\"MARKER\" > /tmp/out\n" +
		"```python\n" +
		"print('hi')\n" +
		"```\n" +
		"MARKER\n" +
		"echo ok\n" +
		"```\n" +
		"\nExpected:\n\n- ok\n"

	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(rb.Steps))
	}
	if !strings.Contains(rb.Steps[0].Command, "echo ok") {
		t.Errorf("command should include 'echo ok', got:\n%s", rb.Steps[0].Command)
	}
}

func TestParseRunbook_NoHeredocRegularFenceStillWorks(t *testing.T) {
	// Ensure regular code blocks without heredocs still parse correctly.
	input := "# Regular Runbook\n\n### Step 1: Simple\n\n" +
		"```bash\n" +
		"echo hello\n" +
		"```\n" +
		"\n### Step 2: Also simple\n\n" +
		"```bash\n" +
		"echo world\n" +
		"```\n"

	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(rb.Steps))
	}
	if rb.Steps[0].Command != "echo hello" {
		t.Errorf("Step 1 command = %q, want %q", rb.Steps[0].Command, "echo hello")
	}
	if rb.Steps[1].Command != "echo world" {
		t.Errorf("Step 2 command = %q, want %q", rb.Steps[1].Command, "echo world")
	}
}

func TestParseRunbook_TimeoutDirective(t *testing.T) {
	input := "# Timeout Test\n\n" +
		"### Step 1: Quick step\n\n" +
		"```bash\necho fast\n```\n\n" +
		"### Step 2: Slow build (timeout: 10m)\n\n" +
		"```bash\necho slow\n```\n\n" +
		"### Step 3: Short timeout (timeout: 30s)\n\n" +
		"```bash\necho short\n```\n"

	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(rb.Steps))
	}

	// Step 1: no timeout directive.
	if rb.Steps[0].Timeout != 0 {
		t.Errorf("step 1: expected no timeout, got %v", rb.Steps[0].Timeout)
	}
	if rb.Steps[0].Title != "Quick step" {
		t.Errorf("step 1: title = %q, want %q", rb.Steps[0].Title, "Quick step")
	}

	// Step 2: 10m timeout, directive stripped from title.
	if rb.Steps[1].Timeout != 10*time.Minute {
		t.Errorf("step 2: expected 10m timeout, got %v", rb.Steps[1].Timeout)
	}
	if rb.Steps[1].Title != "Slow build" {
		t.Errorf("step 2: title = %q, want %q", rb.Steps[1].Title, "Slow build")
	}

	// Step 3: 30s timeout.
	if rb.Steps[2].Timeout != 30*time.Second {
		t.Errorf("step 3: expected 30s timeout, got %v", rb.Steps[2].Timeout)
	}
	if rb.Steps[2].Title != "Short timeout" {
		t.Errorf("step 3: title = %q, want %q", rb.Steps[2].Title, "Short timeout")
	}
}

func TestParseRunbook_DirectiveComment(t *testing.T) {
	input := "# Directive Comment Runbook\n\n" +
		"### Step 1: Quick check\n\n" +
		"```bash\necho ok\n```\n\n" +
		"### Step 2: Flaky service call\n\n" +
		"<!-- runbook: retry=3 delay=5s -->\n\n" +
		"```bash\ncurl http://example.com\n```\n\n" +
		"### Step 3: Depends on step 1\n\n" +
		"<!-- runbook: depends=1 timeout=2m -->\n\n" +
		"```bash\necho depends\n```\n"

	rb, err := ParseRunbook(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseRunbook error: %v", err)
	}

	if len(rb.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(rb.Steps))
	}

	// Step 1: no directives.
	if rb.Steps[0].Retry != 0 {
		t.Errorf("step 1: expected no retry, got %d", rb.Steps[0].Retry)
	}

	// Step 2: retry=3, delay=5s.
	if rb.Steps[1].Retry != 3 {
		t.Errorf("step 2: expected retry=3, got %d", rb.Steps[1].Retry)
	}
	if rb.Steps[1].RetryDelay != 5*time.Second {
		t.Errorf("step 2: expected delay=5s, got %v", rb.Steps[1].RetryDelay)
	}

	// Step 3: depends=1, timeout=2m.
	if rb.Steps[2].DependsOn != 1 {
		t.Errorf("step 3: expected depends=1, got %d", rb.Steps[2].DependsOn)
	}
	if rb.Steps[2].Timeout != 2*time.Minute {
		t.Errorf("step 3: expected timeout=2m, got %v", rb.Steps[2].Timeout)
	}
}

// Runbook is the parsed output from ParseRunbook — defined here for test clarity.
// The actual struct lives in parser.go.
