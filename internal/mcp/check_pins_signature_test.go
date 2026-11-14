package mcp

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// All key material is ephemeral and synthetic. No fixture opens a real key,
// provider account, or executable server.
func baselineKeyPair(t *testing.T) (string, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pubPath, privPath := filepath.Join(dir, "verify.pem"), filepath.Join(dir, "sign.pem")
	for path, block := range map[string]*pem.Block{pubPath: {Type: "PUBLIC KEY", Bytes: pubDER}, privPath: {Type: "PRIVATE KEY", Bytes: privDER}} {
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return pubPath, privPath
}

func signedAccept(t *testing.T, s *Service, source *Source, server Server, now contractObservation, opts CheckOptions) *CheckReport {
	t.Helper()
	opts.Live = true
	previewOpts := opts
	previewOpts.SigningKeyPath = ""
	_, preview := baselineObserve(s, source, "", "echo", server, now, previewOpts)
	revision := preview.Servers[0].Contract.Revision
	if revision == "" {
		t.Fatalf("no signed review: %+v", preview.Servers[0])
	}
	opts.AcceptTools, opts.Names, opts.Revision = true, []string{"echo"}, revision
	b, report := baselineObserve(s, source, "", "echo", server, now, opts)
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceAccepted {
		t.Fatalf("signing refused: %+v", report.Servers[0])
	}
	return report
}

func assertSignedRefusal(t *testing.T, report *CheckReport) {
	t.Helper()
	r := report.Servers[0].Contract
	if r.Status != "error" || r.Revision != "" || len(r.Tools) != 0 {
		t.Fatalf("integrity refusal disclosed review: %+v", r)
	}
	assertBaselineLevel(t, report, "error")
}

func TestSignedBaselineFirstTrustAndNoop(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	pub, priv := baselineKeyPair(t)
	opts := CheckOptions{Live: true, VerifyKeyPath: pub}
	b, report := baselineObserve(s, source, "", "echo", server, now, opts)
	if report.Servers[0].Contract.Status != "untrusted" {
		t.Fatalf("first trust: %+v", report)
	}
	b.save(s, report)
	if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
		t.Fatal("preview wrote store")
	}
	opts.SigningKeyPath = priv
	signedAccept(t, s, source, server, now, opts)
	before := baselineBytes(t, s)
	var envelope struct {
		Signature *toolPinSignature `json:"signature"`
	}
	if json.Unmarshal(before, &envelope) != nil || envelope.Signature == nil || envelope.Signature.Algorithm != "Ed25519" {
		t.Fatalf("missing signature: %s", before)
	}
	// Observation uses only the external public key. No private key is needed.
	_, report = baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub})
	if report.Servers[0].Contract.Status != "unchanged" {
		t.Fatalf("verified observation: %+v", report.Servers[0])
	}
	// Preserve arbitrary valid outer whitespace too: no-op must not reformat.
	writeBaseline(t, s, append([]byte(" \n"), before...))
	before = baselineBytes(t, s)
	signedAccept(t, s, source, server, now, opts)
	if !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("no-op resigned or reformatted store")
	}
}

func TestToolPinSigningPayloadIsDomainSeparatedCompleteEnvelope(t *testing.T) {
	pins := emptyToolPins()
	payload, err := toolPinSigningPayload(pins)
	if err != nil || string(payload) != "skillshare-mcp-pins-v2\n"+`{"version":2,"servers":{},"records":{}}` {
		t.Fatalf("unexpected signature preimage: %q %v", payload, err)
	}
	pubPath, privPath := baselineKeyPair(t)
	keys, err := loadToolPinKeys(CheckOptions{VerifyKeyPath: pubPath, SigningKeyPath: privPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := signToolPins(&pins, keys); err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(pins.Signature.Value)
	if err != nil || !ed25519.Verify(keys.verify, payload, sig) || ed25519.Verify(keys.verify, bytes.TrimPrefix(payload, []byte("skillshare-mcp-pins-v2\n")), sig) {
		t.Fatal("signature is not bound to exact domain and complete envelope")
	}
}

func TestSigningInputsDoNotReachLiveProbe(t *testing.T) {
	_, _, server, _ := baselineFixture(t)
	plain := newLiveProbe(server, "", DefaultLiveTimeout, CheckOptions{})
	opts := CheckOptions{VerifyKeyPath: "/synthetic/verify.pem", SigningKeyPath: "/synthetic/private.pem", SignUnsigned: true}
	protected := newLiveProbe(server, "", DefaultLiveTimeout, opts)
	// Compare every probe field: connection identity, argv, env, headers and
	// client metadata must all be independent of local key inputs.
	if !reflect.DeepEqual(plain, protected) {
		t.Fatal("local signing options reached the live probe")
	}
}

func TestSignedBaselineAcceptsReviewedDriftAndPreservesKeyPrivacy(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	pub, priv := baselineKeyPair(t)
	opts := CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv}
	signedAccept(t, s, source, server, now, opts)
	before := baselineBytes(t, s)
	name, tool, err := captureTool(json.RawMessage(`{"name":"echo","description":"Reviewed change","inputSchema":{"type":"object"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{name: tool}
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub})
	if preview.Servers[0].Contract.Status != "drifted" || !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("drift was trusted or persisted")
	}
	report := signedAccept(t, s, source, server, now, opts)
	after := baselineBytes(t, s)
	if bytes.Equal(before, after) {
		t.Fatal("signed change did not persist")
	}
	_, verified := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub})
	if verified.Servers[0].Contract.Status != "unchanged" {
		t.Fatal("updated signature did not verify")
	}
	serialized, _ := json.Marshal(report)
	privatePEM, err := os.ReadFile(priv)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(privatePEM)
	for _, forbidden := range []string{pub, priv, string(privatePEM), base64.StdEncoding.EncodeToString(block.Bytes)} {
		if bytes.Contains(after, []byte(forbidden)) || bytes.Contains(serialized, []byte(forbidden)) {
			t.Fatal("key data reached persisted state or report")
		}
	}
}

func TestSignedBaselineBootstrapRevisionBindsExternalAnchor(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	pub, _ := baselineKeyPair(t)
	otherPub, otherPriv := baselineKeyPair(t)
	_, first := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub})
	_, other := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: otherPub})
	if first.Servers[0].Contract.Revision == other.Servers[0].Contract.Revision {
		t.Fatal("external anchor absent from review")
	}
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: otherPub, SigningKeyPath: otherPriv, AcceptTools: true, Names: []string{"echo"}, Revision: first.Servers[0].Contract.Revision})
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceStale {
		t.Fatal("different anchor reused bootstrap review")
	}
	if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
		t.Fatal("stale anchor wrote store")
	}
}

func TestSignedBaselineLegacyV1Migration(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	legacy := []byte(`{"version":1,"servers":{"/legacy":{"old":{"echo":"` + strings.Repeat("a", 64) + `"}}}}`)
	writeBaseline(t, s, legacy)
	pub, priv := baselineKeyPair(t)
	signedAccept(t, s, source, server, now, CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv, SignUnsigned: true})
	keys, err := loadToolPinKeys(CheckOptions{VerifyKeyPath: pub})
	if err != nil {
		t.Fatal(err)
	}
	pins, err := parseProtectedToolPins(baselineBytes(t, s), keys, false)
	if err != nil || pins.Version != 2 || pins.Servers["/legacy"]["old"]["echo"] != strings.Repeat("a", 64) {
		t.Fatalf("legacy migration lost evidence: %+v %v", pins, err)
	}
}

func TestSignedBaselineRequiredExternalAnchor(t *testing.T) {
	for _, mode := range []string{"missing-verifier", "wrong-verifier", "stripped", "unsigned-required", "no-signer", "mismatched-signer"} {
		t.Run(mode, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			pub, priv := baselineKeyPair(t)
			otherPub, otherPriv := baselineKeyPair(t)
			opts := CheckOptions{Live: true, VerifyKeyPath: pub}
			if mode == "unsigned-required" {
				baselineAccept(t, s, source, "", "echo", server, now)
			} else {
				signedAccept(t, s, source, server, now, CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv})
			}
			switch mode {
			case "missing-verifier":
				opts.VerifyKeyPath = ""
			case "wrong-verifier":
				opts.VerifyKeyPath = otherPub
			case "stripped":
				pins, err := parseToolPins(baselineBytes(t, s))
				if err != nil {
					t.Fatal(err)
				}
				pins.Signature = nil
				raw, err := marshalToolPins(pins)
				if err != nil {
					t.Fatal(err)
				}
				writeBaseline(t, s, raw)
			case "no-signer", "mismatched-signer":
				_, preview := baselineObserve(s, source, "", "echo", server, now, opts)
				opts.AcceptTools, opts.Names, opts.Revision = true, []string{"echo"}, preview.Servers[0].Contract.Revision
				if mode == "mismatched-signer" {
					opts.SigningKeyPath = otherPriv
				}
			}
			before := baselineBytes(t, s)
			b, report := baselineObserve(s, source, "", "echo", server, now, opts)
			assertSignedRefusal(t, report)
			b.save(s, report)
			if !bytes.Equal(before, baselineBytes(t, s)) {
				t.Fatal("refusal changed store")
			}
			raw, _ := json.Marshal(report)
			if strings.Contains(string(raw), pub) || strings.Contains(string(raw), priv) {
				t.Fatal("key path leaked")
			}
		})
	}
}

func TestSignedBaselineCoversAllScopesAndLegacyEvidence(t *testing.T) {
	for _, change := range []string{"policy", "identity", "root", "legacy", "signature", "algorithm", "unknown", "duplicate", "null", "extra-signature-field", "wrong-case", "bad-unicode", "v1-signature"} {
		t.Run(change, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			baselineAccept(t, s, source, "/other", "echo", server, now)
			pins, err := parseToolPins(baselineBytes(t, s))
			if err != nil {
				t.Fatal(err)
			}
			pins.Servers["/legacy"] = map[string]map[string]string{"old": {"echo": strings.Repeat("a", 64)}}
			raw, _ := marshalToolPins(pins)
			writeBaseline(t, s, raw)
			pub, priv := baselineKeyPair(t)
			signedAccept(t, s, source, server, now, CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv, SignUnsigned: true})
			pins, err = parseToolPins(baselineBytes(t, s))
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "policy", "identity", "root":
				for key, record := range pins.Records {
					if record.Root != "/other" {
						continue
					}
					delete(pins.Records, key)
					switch change {
					case "policy":
						record.Policy = contractPolicy
					case "identity":
						record.Identity = strings.Repeat("b", 64)
					case "root":
						record.Root = "/changed"
					}
					pins.Records[toolRecordKey(record.Config, record.Root, record.Name)] = record
				}
			case "legacy":
				pins.Servers["/legacy"]["old"]["echo"] = strings.Repeat("b", 64)
			case "signature":
				pins.Signature.Value = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
			case "algorithm":
				pins.Signature.Algorithm = "unknown"
			}
			raw, _ = marshalToolPins(pins)
			switch change {
			case "unknown":
				raw = bytes.Replace(raw, []byte(`"version":2`), []byte(`"extra":true,"version":2`), 1)
			case "duplicate":
				raw = bytes.Replace(raw, []byte(`"algorithm":"Ed25519"`), []byte(`"algorithm":"Ed25519","algorithm":"Ed25519"`), 1)
			case "null":
				raw = bytes.Replace(raw, []byte(`"signature":{`), []byte(`"signature":null,"ignored":{`), 1)
			case "extra-signature-field":
				raw = bytes.Replace(raw, []byte(`"algorithm":"Ed25519"`), []byte(`"extra":true,"algorithm":"Ed25519"`), 1)
			case "wrong-case":
				raw = bytes.Replace(raw, []byte(`"algorithm":`), []byte(`"Algorithm":`), 1)
			case "bad-unicode":
				raw = bytes.Replace(raw, []byte(`"Ed25519"`), []byte(`"Ed25519\ud800"`), 1)
			case "v1-signature":
				raw = bytes.Replace(raw, []byte(`"version":2`), []byte(`"version":1`), 1)
			}
			writeBaseline(t, s, raw)
			// Explicit migration must never bypass a malformed or invalid signature.
			for _, migration := range []bool{false, true} {
				_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub, SignUnsigned: migration})
				assertSignedRefusal(t, report)
			}
		})
	}
}

func TestSignedBaselineMigrationPreservesUnselectedEvidenceWithoutTrust(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	baselineAccept(t, s, source, "/other", "echo", server, now)
	baselineAccept(t, s, source, "", "echo", server, now)
	unsigned, err := parseToolPins(baselineBytes(t, s))
	if err != nil {
		t.Fatal(err)
	}
	pub, priv := baselineKeyPair(t)
	_, plain := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub, SignUnsigned: true})
	if preview.Servers[0].Contract.Status != "legacy" || !preview.Servers[0].Contract.IdentityChanged {
		t.Fatal("migration preview presented unsigned evidence as trusted")
	}
	_, strict := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub, SignUnsigned: true, StrictTools: true})
	assertBaselineLevel(t, strict, "error")
	if plain.Servers[0].Contract.Revision == preview.Servers[0].Contract.Revision {
		t.Fatal("migration not bound to revision")
	}
	before := baselineBytes(t, s)
	bad := CheckOptions{Live: true, VerifyKeyPath: pub, SigningKeyPath: priv, SignUnsigned: true, AcceptTools: true, Names: []string{"echo"}, Revision: plain.Servers[0].Contract.Revision}
	b, report := baselineObserve(s, source, "", "echo", server, now, bad)
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceStale || !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("ordinary unsigned revision authorized signing")
	}
	signedAccept(t, s, source, server, now, CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv, SignUnsigned: true})
	pins, err := parseToolPins(baselineBytes(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if pins.Signature == nil {
		t.Fatal("matching record no-op bypassed signing")
	}
	for key, record := range pins.Records {
		if record.Root == "/other" {
			if record.Policy != "legacy-unsigned" {
				t.Fatal("unreviewed scope silently approved")
			}
			old := unsigned.Records[key]
			old.Policy = "legacy-unsigned"
			a, _ := json.Marshal(old)
			b, _ := json.Marshal(record)
			if !bytes.Equal(a, b) {
				t.Fatal("migration destroyed unselected evidence")
			}
		} else if record.Policy != contractPolicy {
			t.Fatal("selected record was not approved")
		}
	}
	_, report = baselineObserve(s, source, "/other", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub})
	if report.Servers[0].Contract.Status != "legacy" {
		t.Fatalf("unreviewed scope trusted: %+v", report.Servers[0])
	}
	_, report = baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, VerifyKeyPath: pub, SignUnsigned: true})
	assertSignedRefusal(t, report) // Migration is only for existing unsigned stores.
}

func TestSignedBaselineLockedCASAndIncompleteObservation(t *testing.T) {
	for _, change := range []string{"source", "store", "incomplete"} {
		t.Run(change, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			pub, priv := baselineKeyPair(t)
			signedAccept(t, s, source, server, now, CheckOptions{VerifyKeyPath: pub, SigningKeyPath: priv})
			opts := CheckOptions{Live: true, VerifyKeyPath: pub, SigningKeyPath: priv}
			_, preview := baselineObserve(s, source, "", "echo", server, now, opts)
			opts.AcceptTools, opts.Names, opts.Revision = true, []string{"echo"}, preview.Servers[0].Contract.Revision
			if change == "incomplete" {
				now.Live = nil
			}
			b, report := baselineObserve(s, source, "", "echo", server, now, opts)
			if change == "source" {
				if err := os.WriteFile(source.Path, []byte("# changed\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if change == "store" {
				writeBaseline(t, s, append(baselineBytes(t, s), ' '))
			}
			before := baselineBytes(t, s)
			b.save(s, report)
			if report.ToolsAcceptance == ToolsAcceptanceAccepted || !bytes.Equal(before, baselineBytes(t, s)) {
				t.Fatal("unsafe acceptance changed signed store")
			}
		})
	}
}

func TestToolPinKeysBoundedStrictAndPrivate(t *testing.T) {
	for _, kind := range []string{"missing", "malformed", "oversized", "symlink", "directory", "trailing-pem", "leading-junk", "leading-bad-pem", "wrong-type", "wrong-algorithm", "pem-headers", "private-mode"} {
		t.Run(kind, func(t *testing.T) {
			for _, private := range []bool{false, true} {
				pub, priv := baselineKeyPair(t)
				path := pub
				if private {
					path = priv
				}
				switch kind {
				case "missing":
					path = filepath.Join(t.TempDir(), "absent")
				case "malformed":
					if err := os.WriteFile(path, []byte("private-secret-parser-input"), 0600); err != nil {
						t.Fatal(err)
					}
				case "oversized":
					if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, (64<<10)+1), 0600); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					link := filepath.Join(t.TempDir(), "link")
					if err := os.Symlink(path, link); err != nil {
						t.Skip(err)
					}
					path = link
				case "directory":
					path = t.TempDir()
				case "trailing-pem":
					raw, _ := os.ReadFile(path)
					if err := os.WriteFile(path, append(raw, raw...), 0600); err != nil {
						t.Fatal(err)
					}
				case "leading-junk", "leading-bad-pem":
					raw, _ := os.ReadFile(path)
					prefix := []byte("ignored junk\n")
					if kind == "leading-bad-pem" {
						prefix = []byte("-----BEGIN PUBLIC KEY-----\n!bad\n-----END PUBLIC KEY-----\n")
					}
					if err := os.WriteFile(path, append(prefix, raw...), 0600); err != nil {
						t.Fatal(err)
					}
				case "wrong-type":
					if private {
						path = pub
					} else {
						path = priv
					}
				case "wrong-algorithm":
					key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
					if err != nil {
						t.Fatal(err)
					}
					var der []byte
					kind := "PUBLIC KEY"
					if private {
						der, err = x509.MarshalPKCS8PrivateKey(key)
						kind = "PRIVATE KEY"
					} else {
						der, err = x509.MarshalPKIXPublicKey(&key.PublicKey)
					}
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0600); err != nil {
						t.Fatal(err)
					}
				case "pem-headers":
					raw, _ := os.ReadFile(path)
					block, _ := pem.Decode(raw)
					block.Headers = map[string]string{"Comment": "not-allowed"}
					if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
						t.Fatal(err)
					}
				case "private-mode":
					if !private || runtime.GOOS == "windows" {
						continue
					}
					if err := os.Chmod(path, 0640); err != nil {
						t.Fatal(err)
					}
				}
				opts := CheckOptions{Live: true, VerifyKeyPath: path}
				if private {
					opts.VerifyKeyPath, opts.SigningKeyPath, opts.AcceptTools = pub, path, true
				}
				_, err := loadToolPinKeys(opts)
				if err == nil {
					t.Fatalf("accepted %s private=%v", kind, private)
				}
				if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "private-secret-parser-input") {
					t.Fatal("key details leaked")
				}
			}
		})
	}
}

func TestSigningOptionsRequireExplicitLiveWorkflow(t *testing.T) {
	for _, opts := range []CheckOptions{
		{VerifyKeyPath: "fixture"}, {SigningKeyPath: "fixture"}, {SignUnsigned: true},
		{Live: true, SigningKeyPath: "fixture"}, {Live: true, SignUnsigned: true},
		{Live: true, VerifyKeyPath: "fixture", SigningKeyPath: "fixture"},
	} {
		if validateToolCheckOptions(opts) == nil {
			t.Fatalf("accepted invalid options: %+v", opts)
		}
	}
	for _, opts := range []CheckOptions{{Live: true, VerifyKeyPath: "fixture"}, {Live: true, VerifyKeyPath: "fixture", SignUnsigned: true}} {
		if err := validateToolCheckOptions(opts); err != nil {
			t.Fatal(err)
		}
	}
}
