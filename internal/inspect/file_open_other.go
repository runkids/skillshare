//go:build !unix

package inspect

import "os"

func openReadOnlyFile(path string) (*os.File, error) {
	return os.Open(path)
}
