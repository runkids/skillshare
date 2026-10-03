//go:build !windows

package plugin

import "os"

func makePiRegistrationDir(path string) error { return os.MkdirAll(path, 0o700) }

func checkPiRegistrationPrivate(string) error { return nil }
