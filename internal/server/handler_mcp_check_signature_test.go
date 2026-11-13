package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/mcp"
)

// Exercise the real shared service and API handler boundary: the dashboard has
// no key inputs and must never downgrade a signed store into unsigned approval.
func TestMCPCheckAndAcceptSignedStoreRequireCLIVerifier(t *testing.T) {
	s, calls := checkContractFixture(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	keyRoot := t.TempDir()
	publicPath, privatePath := filepath.Join(keyRoot, "fixture-public.pem"), filepath.Join(keyRoot, "fixture-private.pem")
	for path, block := range map[string]*pem.Block{publicPath: {Type: "PUBLIC KEY", Bytes: publicDER}, privatePath: {Type: "PRIVATE KEY", Bytes: privateDER}} {
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := s.mcpService()
	root := ""
	opts := mcp.CheckOptions{Live: true, SkipDNS: true, Names: []string{"docs"}, Root: &root, VerifyKeyPath: publicPath}
	preview, err := service.Check(opts)
	if err != nil || preview.Servers[0].Contract == nil || preview.Servers[0].Contract.Revision == "" {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	opts.AcceptTools, opts.SigningKeyPath, opts.Revision = true, privatePath, preview.Servers[0].Contract.Revision
	accepted, err := service.Check(opts)
	if err != nil || accepted.ToolsAcceptance != mcp.ToolsAcceptanceAccepted {
		t.Fatalf("signed fixture: %+v %v", accepted, err)
	}
	opts.AcceptTools, opts.SigningKeyPath, opts.Revision = false, "", ""
	verified, err := service.Check(opts)
	if err != nil || verified.Servers[0].Contract == nil || verified.Servers[0].Contract.Status != "unchanged" {
		t.Fatalf("verified fixture: %+v %v", verified, err)
	}
	path := filepath.Join(service.StateDir, "mcp", "tool-pins.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeCalls := calls.Load()
	observed := checkGet(t, s, "live=1&root=")
	if observed.Summary.Errors == 0 || observed.Servers[0].Contract == nil || observed.Servers[0].Contract.Revision != "" {
		t.Fatalf("dashboard trusted signed state without key: %+v", observed)
	}
	body, _ := json.Marshal(map[string]string{"name": "docs", "root": "", "revision": verified.Servers[0].Contract.Revision})
	w := checkAccept(s, string(body))
	if w.Code != 422 || !strings.Contains(w.Body.String(), `"accepted":false`) || !strings.Contains(w.Body.String(), "requires CLI verification with --verify-key") {
		t.Fatalf("unsigned API approval: %d %s", w.Code, w.Body.String())
	}
	if calls.Load() != beforeCalls {
		t.Fatal("verification failure contacted endpoint")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("API modified signed trust")
	}
	if strings.Contains(w.Body.String(), publicPath) || strings.Contains(w.Body.String(), privatePath) {
		t.Fatal("key paths leaked to API")
	}
}
