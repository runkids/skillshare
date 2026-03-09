package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Tests run on the host — allow execution for test suite.
	os.Setenv("RUNBOOK_ALLOW_EXECUTE", "1")
	os.Exit(m.Run())
}

func TestIsContainerEnv_EnvOverride(t *testing.T) {
	// Already set by TestMain, so should return true.
	if !IsContainerEnv() {
		t.Fatal("expected IsContainerEnv=true with RUNBOOK_ALLOW_EXECUTE set")
	}
}

func TestIsContainerEnv_NoOverride(t *testing.T) {
	old := os.Getenv("RUNBOOK_ALLOW_EXECUTE")
	os.Unsetenv("RUNBOOK_ALLOW_EXECUTE")
	defer os.Setenv("RUNBOOK_ALLOW_EXECUTE", old)

	// On host (no /.dockerenv), should return false.
	// Inside a real container, this test still passes (/.dockerenv exists).
	got := IsContainerEnv()
	_, hasDocker := os.Stat("/.dockerenv")
	_, hasPodman := os.Stat("/run/.containerenv")
	inContainer := hasDocker == nil || hasPodman == nil

	if got != inContainer {
		t.Fatalf("IsContainerEnv=%v, expected %v (inContainer=%v)", got, inContainer, inContainer)
	}
}
