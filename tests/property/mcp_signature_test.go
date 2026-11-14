package property

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"hegel.dev/go/hegel"
	"skillshare/internal/mcp"
)

// The external verifier must reject a structurally valid edit independently of
// the contract hashes. Keys are synthetic ephemeral fixture files only.
func TestSignaturePropertyPolicyTamperingNeverGainsTrust(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, endpoint, _ := propertyServer(t)
	hegel.Test(t, func(ht *hegel.T) {
		n := hegel.Draw(ht, hegel.Integers(0, 1000000))
		s := propertyService(t, endpoint)
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			ht.Fatal(err)
		}
		publicDER, err := x509.MarshalPKIXPublicKey(public)
		if err != nil {
			ht.Fatal(err)
		}
		privateDER, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			ht.Fatal(err)
		}
		publicPath, privatePath := filepath.Join(s.Home, "verify.pem"), filepath.Join(s.Home, "sign.pem")
		for path, block := range map[string]*pem.Block{publicPath: {Type: "PUBLIC KEY", Bytes: publicDER}, privatePath: {Type: "PRIVATE KEY", Bytes: privateDER}} {
			if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
				ht.Fatal(err)
			}
		}
		opts := mcp.CheckOptions{Live: true, SkipDNS: true, Names: []string{"fixture"}, VerifyKeyPath: publicPath}
		preview, err := s.Check(opts)
		if err != nil || preview.Servers[0].Contract == nil || preview.Servers[0].Contract.Revision == "" {
			ht.Fatalf("signed preview: %v", err)
		}
		opts.AcceptTools, opts.Revision, opts.SigningKeyPath = true, preview.Servers[0].Contract.Revision, privatePath
		accepted, err := s.Check(opts)
		if err != nil || accepted.ToolsAcceptance != mcp.ToolsAcceptanceAccepted {
			ht.Fatalf("signed acceptance: %v", err)
		}
		path := filepath.Join(s.StateDir, "mcp", "tool-pins.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			ht.Fatal(err)
		}
		var pins map[string]json.RawMessage
		if err := json.Unmarshal(raw, &pins); err != nil {
			ht.Fatal(err)
		}
		var records map[string]map[string]json.RawMessage
		if err := json.Unmarshal(pins["records"], &records); err != nil {
			ht.Fatal(err)
		}
		if len(records) != 1 || pins["signature"] == nil {
			ht.Fatal("fixture did not create one signed record")
		}
		for _, record := range records {
			record["policy"], _ = json.Marshal(fmt.Sprintf("tampered-policy-%d", n))
		}
		pins["records"], _ = json.Marshal(records)
		tampered, err := json.Marshal(pins)
		if err != nil {
			ht.Fatal(err)
		}
		if err := os.WriteFile(path, tampered, 0600); err != nil {
			ht.Fatal(err)
		}
		observing := opts
		observing.AcceptTools, observing.Revision, observing.SigningKeyPath = false, "", ""
		refused, err := s.Check(observing)
		if err != nil || refused.Summary.Errors == 0 || refused.ToolsAcceptance == mcp.ToolsAcceptanceAccepted {
			ht.Fatalf("tampered trust accepted: %v", err)
		}
		for _, server := range refused.Servers {
			if server.Contract != nil && server.Contract.Revision != "" {
				ht.Fatal("tampered store issued revision")
			}
		}
		refused, err = s.Check(opts)
		if err != nil || refused.Summary.Errors == 0 || refused.ToolsAcceptance == mcp.ToolsAcceptanceAccepted {
			ht.Fatalf("tampered approval accepted: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, tampered) {
			ht.Fatal("failed verification modified store")
		}
	}, hegel.WithTestCases(40), hegel.WithDatabase(""))
}
