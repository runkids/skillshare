//go:build !online

package integration

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/testutil"
)

const serveMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`

func TestMCPServe_StdioListsValidSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("pdf", map[string]string{"SKILL.md": "---\nname: pdf\ndescription: Use when editing PDFs\n---\n# PDF\n"})
	sb.CreateSkill("renamed", map[string]string{"SKILL.md": "---\nname: original\ndescription: Use when testing\n---\n# Renamed\n"})

	// The server drops requests still in flight when stdin closes, so keep it
	// open until both responses arrive, as a spawning gateway does.
	cmd := exec.Command(sb.BinaryPath, "mcp", "serve", "-g")
	cmd.Env = append(os.Environ(), "HOME="+sb.Home, "SKILLSHARE_CONFIG="+sb.ConfigPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+serveMeta+`}}`)
	fmt.Fprintln(stdin, `{"jsonrpc":"2.0","id":2,"method":"skills/list","params":{`+serveMeta+`}}`)
	scanner := bufio.NewScanner(stdout)
	var out strings.Builder
	for range 2 {
		if !scanner.Scan() {
			t.Fatalf("missing response: %v\nstderr: %s", scanner.Err(), stderr.String())
		}
		out.WriteString(scanner.Text())
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("serve exited with %v\nstderr: %s", err, stderr.String())
	}

	for _, want := range []string{`"io.modelcontextprotocol/skills"`, `"uri":"skill://pdf/SKILL.md"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("responses missing %s:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "skill://renamed") {
		t.Errorf("renamed skill was served:\n%s", out.String())
	}
	if want := `skipped renamed: SKILL.md name "original" does not match directory name "renamed"`; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr missing %q:\n%s", want, stderr.String())
	}
}

func TestMCPServe_CheckListsSkippedSkillsWithoutServing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("pdf", map[string]string{"SKILL.md": "---\nname: pdf\ndescription: Use when editing PDFs\n---\n# PDF\n"})
	sb.CreateSkill("renamed", map[string]string{"SKILL.md": "---\nname: original\ndescription: Use when testing\n---\n# Renamed\n"})

	r := sb.RunCLI("mcp", "serve", "--check", "-g")

	r.AssertSuccess(t)
	r.AssertRowContains(t, "renamed", `SKILL.md name "original" does not match directory name "renamed"`)
	r.AssertOutputContains(t, "1 skill served, 1 skipped")
}

func TestMCPServe_CheckCountsEditableSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp:\n  serve:\n    edit:\n      skills: [pdf]\n")
	sb.CreateSkill("pdf", map[string]string{"SKILL.md": "---\nname: pdf\ndescription: Use when editing PDFs\n---\n# PDF\n"})
	sb.CreateSkill("review", map[string]string{"SKILL.md": "---\nname: review\ndescription: Use when reviewing\n---\n# Review\n"})

	r := sb.RunCLI("mcp", "serve", "--check", "-g")

	r.AssertSuccess(t)
	r.AssertRowContains(t, "Editable", "1 of 2 (mcp.serve.edit.skills)")
}

func TestMCPServe_RejectsAnInvalidPreviewLifetime(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp:\n  serve:\n    edit:\n      skills: [pdf]\n      preview_ttl: soon\n")

	r := sb.RunCLI("mcp", "serve", "--check", "-g")

	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, `mcp.serve.edit.preview_ttl must be a positive duration such as 30m, not "soon"`)
}

func TestMCPServe_RefusesUnauthenticatedNetworkListener(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	r := sb.RunCLI("mcp", "serve", "--http", "0.0.0.0:0")

	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "SKILLSHARE_MCP_TOKEN")
}

// A token sent over plain HTTP off loopback can be read by anyone on the path.
func TestMCPServe_RefusesPlainHTTPNetworkListener(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	r := sb.RunCLIEnv(map[string]string{"SKILLSHARE_MCP_TOKEN": "secret"}, "mcp", "serve", "--http", "0.0.0.0:0")

	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "--tls-cert")
}

func TestMCPServe_RequiresBothTLSFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	r := sb.RunCLI("mcp", "serve", "--http", "127.0.0.1:0", "--tls-cert", "cert.pem")

	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "--tls-cert and --tls-key go together")
}

func TestMCPServe_ServesHTTPSOffLoopbackWithTokenAndCertificate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	certFile, keyFile := writeSelfSignedCert(t)

	cmd := exec.Command(sb.BinaryPath, "mcp", "serve", "--http", "0.0.0.0:0", "--tls-cert", certFile, "--tls-key", keyFile)
	cmd.Env = append(os.Environ(), "HOME="+sb.Home, "SKILLSHARE_CONFIG="+sb.ConfigPath, "SKILLSHARE_MCP_TOKEN=secret")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	line, _ := bufio.NewReader(stderr).ReadString('\n')
	if !strings.Contains(line, "https://") {
		t.Errorf("first stderr line = %q, want the https:// address", line)
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func TestMCPServe_RejectsUnknownTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	r := sb.RunCLI("mcp", "serve", "--target", "nope")

	// stdout is the protocol stream, so the error goes to stderr alone.
	r.AssertFailure(t)
	r.AssertErrorContains(t, `unknown target "nope"`)
	if r.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", r.Stdout)
	}
}

func TestMCPServe_ReportsLegacyMigrationOnStderr(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	// A legacy trash folder in the config dir is moved to the data dir by the first command.
	if err := os.MkdirAll(filepath.Join(sb.Home, ".config", "skillshare", "trash"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(sb.Home, ".local", "share", "skillshare", "trash")); err != nil {
		t.Fatal(err)
	}

	r := sb.RunCLI("mcp", "serve", "--target", "nope")

	r.AssertErrorContains(t, "Moved legacy data")
	if r.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", r.Stdout)
	}
}
