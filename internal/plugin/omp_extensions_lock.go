package plugin

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
)

// OMP v18.6.1 pi-natives file_lock/mod.rs hashes the absolute sidecar path
// with two seeded XXH64 hashes. Linux sockets and Windows mutexes share this name.
func ompLockName(path string) string {
	return fmt.Sprintf("omp-file-lock-%016x%016x", ompXXH64([]byte(path), 0x4f4d502d4c4f434b), ompXXH64([]byte(path), 0x50492d46494c454c))
}

// Match native realpath-based write locking, including Windows canonical case.
func ompResolvedLockPath(path string) (string, error) {
	if err := ompSafePath(path); err != nil {
		return "", err
	}
	var missing []string
	for current := path; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(current) == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
	}
}

func ompXXH64(data []byte, seed uint64) uint64 {
	const p1 uint64 = 11400714785074694791
	const p2 uint64 = 14029467366897019727
	const p3 uint64 = 1609587929392839161
	const p4 uint64 = 9650029242287828579
	const p5 uint64 = 2870177450012600261
	round := func(a, b uint64) uint64 { return bits.RotateLeft64(a+b*p2, 31) * p1 }
	length := len(data)
	var h uint64
	if length >= 32 {
		v1, v2, v3, v4 := seed+p1+p2, seed+p2, seed, seed-p1
		for len(data) >= 32 {
			v1 = round(v1, binary.LittleEndian.Uint64(data))
			v2 = round(v2, binary.LittleEndian.Uint64(data[8:]))
			v3 = round(v3, binary.LittleEndian.Uint64(data[16:]))
			v4 = round(v4, binary.LittleEndian.Uint64(data[24:]))
			data = data[32:]
		}
		h = bits.RotateLeft64(v1, 1) + bits.RotateLeft64(v2, 7) + bits.RotateLeft64(v3, 12) + bits.RotateLeft64(v4, 18)
		for _, v := range []uint64{v1, v2, v3, v4} {
			h = (h^round(0, v))*p1 + p4
		}
	} else {
		h = seed + p5
	}
	h += uint64(length)
	for len(data) >= 8 {
		h = bits.RotateLeft64(h^round(0, binary.LittleEndian.Uint64(data)), 27)*p1 + p4
		data = data[8:]
	}
	if len(data) >= 4 {
		h = bits.RotateLeft64(h^uint64(binary.LittleEndian.Uint32(data))*p1, 23)*p2 + p3
		data = data[4:]
	}
	for _, b := range data {
		h = bits.RotateLeft64(h^uint64(b)*p5, 11) * p1
	}
	h ^= h >> 33
	h *= p2
	h ^= h >> 29
	h *= p3
	h ^= h >> 32
	return h
}
