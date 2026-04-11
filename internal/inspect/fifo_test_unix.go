//go:build unix

package inspect

import "golang.org/x/sys/unix"

func createTestFIFO(path string, mode uint32) error {
	return unix.Mkfifo(path, mode)
}
