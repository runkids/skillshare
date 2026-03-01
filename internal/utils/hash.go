package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// FileHash returns the hex-encoded SHA-256 digest of a file.
func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileHashFormatted returns the hash in "sha256:<hex>" format.
func FileHashFormatted(path string) (string, error) {
	h, err := FileHash(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%s", h), nil
}
