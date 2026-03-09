package main

import "testing"

func TestClassify_AutoBashWithExpected(t *testing.T) {
	s := Step{Command: "echo hello", Lang: "bash", Expected: []string{"hello"}}
	if got := Classify(s); got != "auto" {
		t.Errorf("Classify() = %q, want %q", got, "auto")
	}
}

func TestClassify_ManualNoCommand(t *testing.T) {
	s := Step{Title: "Check the dashboard", Description: "Verify visually"}
	if got := Classify(s); got != "manual" {
		t.Errorf("Classify() = %q, want %q", got, "manual")
	}
}

func TestClassify_ManualNonBashLang(t *testing.T) {
	s := Step{Command: "fmt.Println(\"hi\")", Lang: "go"}
	if got := Classify(s); got != "manual" {
		t.Errorf("Classify() = %q, want %q", got, "manual")
	}
}

func TestClassify_AutoBashNoExpected(t *testing.T) {
	s := Step{Command: "make build", Lang: "bash"}
	if got := Classify(s); got != "auto" {
		t.Errorf("Classify() = %q, want %q", got, "auto")
	}
}

func TestClassify_AutoShLang(t *testing.T) {
	s := Step{Command: "ls -la", Lang: "sh"}
	if got := Classify(s); got != "auto" {
		t.Errorf("Classify() = %q, want %q", got, "auto")
	}
}

func TestClassify_AutoEmptyLang(t *testing.T) {
	s := Step{Command: "whoami"}
	if got := Classify(s); got != "auto" {
		t.Errorf("Classify() = %q, want %q", got, "auto")
	}
}

func TestClassifyAll(t *testing.T) {
	steps := []Step{
		{Number: 1, Command: "echo ok", Lang: "bash"},
		{Number: 2, Title: "Manual check"},
		{Number: 3, Command: "go run .", Lang: "go"},
		{Number: 4, Command: "ls", Lang: "sh"},
	}

	result := ClassifyAll(steps)

	want := []string{"auto", "manual", "manual", "auto"}
	for i, w := range want {
		if result[i].Executor != w {
			t.Errorf("step %d: Executor = %q, want %q", result[i].Number, result[i].Executor, w)
		}
	}
}
