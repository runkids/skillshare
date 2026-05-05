package server

import (
	"testing"

	"skillshare/internal/config"
)

func TestParseOpts_GlobalMode(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.GitLabHosts = []string{"global.host"}

	opts := s.parseOpts()
	if len(opts.GitLabHosts) != 1 || opts.GitLabHosts[0] != "global.host" {
		t.Errorf("global mode: expected [global.host], got %v", opts.GitLabHosts)
	}
}

func TestParseOpts_ProjectMode_UsesProjectConfig(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.GitLabHosts = []string{"global.host"}
	s.projectRoot = t.TempDir()
	s.projectCfg = &config.ProjectConfig{
		GitLabHosts: []string{"project.host"},
	}

	opts := s.parseOpts()
	if len(opts.GitLabHosts) != 1 || opts.GitLabHosts[0] != "project.host" {
		t.Errorf("project mode: expected [project.host], got %v", opts.GitLabHosts)
	}
}

func TestParseOpts_ProjectMode_NoFallbackToGlobal(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.GitLabHosts = []string{"global.host"}
	s.projectRoot = t.TempDir()
	s.projectCfg = &config.ProjectConfig{} // no gitlab_hosts

	opts := s.parseOpts()
	if len(opts.GitLabHosts) != 0 {
		t.Errorf("project mode with no hosts should not fall back to global, got %v", opts.GitLabHosts)
	}
}

func TestParseOpts_ProjectMode_MergesEnvVar(t *testing.T) {
	s, _ := newTestServer(t)
	s.projectRoot = t.TempDir()
	s.projectCfg = &config.ProjectConfig{
		GitLabHosts: []string{"project.host"},
	}

	t.Setenv("SKILLSHARE_GITLAB_HOSTS", "env.host")

	opts := s.parseOpts()
	if len(opts.GitLabHosts) != 2 {
		t.Fatalf("expected 2 hosts (project + env), got %d: %v", len(opts.GitLabHosts), opts.GitLabHosts)
	}
	if opts.GitLabHosts[0] != "project.host" {
		t.Errorf("[0] = %s, want project.host", opts.GitLabHosts[0])
	}
	if opts.GitLabHosts[1] != "env.host" {
		t.Errorf("[1] = %s, want env.host", opts.GitLabHosts[1])
	}
}

func TestParseOpts_GlobalMode_AzureHosts(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AzureHosts = []string{"azure.company.com"}

	opts := s.parseOpts()
	if len(opts.AzureHosts) != 1 || opts.AzureHosts[0] != "azure.company.com" {
		t.Errorf("global mode: expected [azure.company.com], got %v", opts.AzureHosts)
	}
}

func TestParseOpts_ProjectMode_AzureHosts(t *testing.T) {
	s, _ := newTestServer(t)
	s.projectRoot = t.TempDir()
	s.projectCfg = &config.ProjectConfig{
		AzureHosts: []string{"azure.company.com"},
	}

	opts := s.parseOpts()
	if len(opts.AzureHosts) != 1 || opts.AzureHosts[0] != "azure.company.com" {
		t.Errorf("project mode: expected [azure.company.com], got %v", opts.AzureHosts)
	}
}
