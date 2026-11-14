package mcp

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"runtime"
)

const toolPinSignatureDomain = "skillshare-mcp-pins-v2\n"
const maxToolPinKeyBytes = 64 << 10

var (
	errToolPinKey            = errors.New("MCP baseline key is invalid or unreadable; use an Ed25519 PEM key with safe file permissions")
	errToolPinVerifyRequired = errors.New("signed MCP tool baseline requires CLI verification with --verify-key")
	errToolPinSignature      = errors.New("MCP tool baseline signature could not be verified; supply the correct external verification key")
	errToolPinUnsigned       = errors.New("unsigned MCP tool baseline requires explicit --sign-unsigned review before signing")
	errToolPinSigner         = errors.New("accepting signed MCP tool baselines requires a matching --signing-key and --verify-key")
	errToolPinMigration      = errors.New("--sign-unsigned requires an existing unsigned MCP tool baseline")
)

type toolPinSignature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type toolPinKeys struct {
	verify ed25519.PublicKey
	sign   ed25519.PrivateKey
}

func validateToolPinSignature(raw json.RawMessage, signature *toolPinSignature) error {
	var fields map[string]json.RawMessage
	if signature == nil || json.Unmarshal(raw, &fields) != nil || !exactToolPinFields(fields, "algorithm", "value") || signature.Algorithm != "Ed25519" {
		return errToolPins
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(signature.Value)
	if err != nil || len(sig) != ed25519.SignatureSize || base64.StdEncoding.EncodeToString(sig) != signature.Value {
		return errToolPins
	}
	return nil
}

// Read the same bounded regular file that was checked. Refuse links, including
// replacement races; never return OS/parser errors containing key paths or data.
func readToolPinKey(path string, private bool) (*pem.Block, error) {
	valid := func(info os.FileInfo) bool {
		return info.Mode().IsRegular() && info.Size() <= maxToolPinKeyBytes &&
			(!private || runtime.GOOS == "windows" || info.Mode().Perm()&0077 == 0)
	}
	before, err := os.Lstat(path)
	if err != nil || !valid(before) {
		return nil, errToolPinKey
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errToolPinKey
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !valid(opened) || !os.SameFile(before, opened) {
		return nil, errToolPinKey
	}
	data, err := io.ReadAll(io.LimitReader(f, maxToolPinKeyBytes+1))
	if err != nil || len(data) > maxToolPinKeyBytes {
		return nil, errToolPinKey
	}
	after, err := os.Lstat(path)
	if err != nil || !valid(after) || !os.SameFile(opened, after) {
		return nil, errToolPinKey
	}
	trimmed := bytes.TrimSpace(data)
	// pem.Decode otherwise silently discards arbitrary leading text.
	want := "PUBLIC KEY"
	if private {
		want = "PRIVATE KEY"
	}
	header := []byte("-----BEGIN " + want + "-----")
	if !bytes.HasPrefix(trimmed, append(header, '\n')) && !bytes.HasPrefix(trimmed, append(header, '\r', '\n')) {
		return nil, errToolPinKey
	}
	if bytes.Count(trimmed, []byte("-----BEGIN ")) != 1 {
		return nil, errToolPinKey
	}
	block, rest := pem.Decode(trimmed)
	if block == nil || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errToolPinKey
	}
	if block.Type != want {
		return nil, errToolPinKey
	}
	return block, nil
}

func loadToolPinKeys(opts CheckOptions) (toolPinKeys, error) {
	var keys toolPinKeys
	if opts.VerifyKeyPath != "" {
		block, err := readToolPinKey(opts.VerifyKeyPath, false)
		if err != nil {
			return keys, err
		}
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return keys, errToolPinKey
		}
		var ok bool
		keys.verify, ok = key.(ed25519.PublicKey)
		if !ok || len(keys.verify) != ed25519.PublicKeySize {
			return toolPinKeys{}, errToolPinKey
		}
	}
	if opts.SigningKeyPath != "" {
		block, err := readToolPinKey(opts.SigningKeyPath, true)
		if err != nil {
			return keys, err
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return keys, errToolPinKey
		}
		var ok bool
		keys.sign, ok = key.(ed25519.PrivateKey)
		if !ok || len(keys.sign) != ed25519.PrivateKeySize {
			return toolPinKeys{}, errToolPinKey
		}
		if !bytes.Equal(keys.sign.Public().(ed25519.PublicKey), keys.verify) {
			return toolPinKeys{}, errToolPinSigner
		}
	}
	if opts.AcceptTools && len(keys.verify) != 0 && len(keys.sign) == 0 {
		return toolPinKeys{}, errToolPinSigner
	}
	return keys, nil
}

// Parse and verify one immutable byte slice, never a later read of the path.
// A structurally valid unsigned parse is not sufficient when an anchor is set.
func parseProtectedToolPins(data []byte, keys toolPinKeys, migrate bool) (toolPinFile, error) {
	pins, err := parseToolPins(data)
	if err != nil {
		return pins, err
	}
	if pins.Signature == nil {
		if len(keys.verify) != 0 && !migrate {
			return pins, errToolPinUnsigned
		}
		return pins, nil
	}
	if len(keys.verify) == 0 {
		return pins, errToolPinVerifyRequired
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(pins.Signature.Value)
	if err != nil {
		return pins, errToolPinSignature
	}
	payload, err := toolPinSigningPayload(pins)
	if err != nil || !ed25519.Verify(keys.verify, payload, sig) {
		return pins, errToolPinSignature
	}
	if migrate {
		return pins, errToolPinMigration
	}
	return pins, nil
}

func toolPinSigningPayload(pins toolPinFile) ([]byte, error) {
	pins.Signature = nil
	data, err := marshalToolPins(pins)
	if err != nil {
		return nil, err
	}
	return append([]byte(toolPinSignatureDomain), bytes.TrimSuffix(data, []byte{'\n'})...), nil
}

func signToolPins(pins *toolPinFile, keys toolPinKeys) error {
	if len(keys.sign) != ed25519.PrivateKeySize || !bytes.Equal(keys.sign.Public().(ed25519.PublicKey), keys.verify) {
		return errToolPinSigner
	}
	payload, err := toolPinSigningPayload(*pins)
	if err != nil {
		return err
	}
	pins.Signature = &toolPinSignature{Algorithm: "Ed25519", Value: base64.StdEncoding.EncodeToString(ed25519.Sign(keys.sign, payload))}
	return nil
}

// Copy the record map so preview cannot mutate the loaded trusted snapshot.
// All unselected evidence survives, but gains no trust from another scope's review.
func proposedToolPins(pins toolPinFile, record toolPinRecord, migrate bool) toolPinFile {
	next := pins
	next.Version = 2
	next.Signature = nil
	next.Records = make(map[string]toolPinRecord, len(pins.Records)+1)
	for key, old := range pins.Records {
		if migrate {
			old.Policy = "legacy-unsigned"
		}
		next.Records[key] = old
	}
	next.Records[toolRecordKey(record.Config, record.Root, record.Name)] = record
	return next
}
