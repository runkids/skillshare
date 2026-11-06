package audit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const headerReadLimit = 512

// Final-release CPython 3.6–3.14 magic values, pinned to CPython v3.14.0:
// https://github.com/python/cpython/blob/v3.14.0/Include/internal/pycore_magic_number.h
// Review new values against a tagged CPython source and add a fixture before
// extending this table. This identifies a format, never decodes bytecode.
var pythonMagicVersions = map[uint16]string{
	3379: "3.6", 3394: "3.7", 3413: "3.8", 3425: "3.9", 3439: "3.10",
	3495: "3.11", 3531: "3.12", 3571: "3.13", 3627: "3.14",
}

var disguiseExtensions = map[string]bool{
	".md": true, ".txt": true, ".rst": true, ".pdf": true,
	".doc": true, ".docx": true, ".rtf": true, ".html": true,
	".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".webp": true, ".ico": true, ".bmp": true,
	".tif": true, ".tiff": true,
}

// Cache payloads and content/extension mismatches are stronger evidence than
// dot names, which are common in tooling and virtual environments.
var disguiseCacheDirectories = map[string]bool{
	".cache": true, "__pycache__": true,
}

func hiddenParent(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}

func disguisedPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts[:len(parts)-1] {
		if disguiseCacheDirectories[part] {
			return true
		}
	}
	return disguiseExtensions[strings.ToLower(filepath.Ext(path))]
}

func validELFHeader(data []byte) bool {
	if len(data) < 24 || data[4] < 1 || data[4] > 2 || data[5] < 1 || data[5] > 2 || data[6] != 1 {
		return false
	}
	class := data[4]
	headerSize, sizeOffset := 52, 40
	if class == 2 {
		headerSize, sizeOffset = 64, 52
	}
	if len(data) < headerSize {
		return false
	}
	var order binary.ByteOrder = binary.LittleEndian
	if data[5] == 2 {
		order = binary.BigEndian
	}
	return order.Uint16(data[16:18]) != 0 &&
		order.Uint16(data[18:20]) != 0 &&
		order.Uint32(data[20:24]) == 1 &&
		int(order.Uint16(data[sizeOffset:sizeOffset+2])) == headerSize
}

func validZIPHeader(data []byte) bool {
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		if len(data) < 30 {
			return false
		}
		version := binary.LittleEndian.Uint16(data[4:6])
		nameSize := uint64(binary.LittleEndian.Uint16(data[26:28]))
		extraSize := uint64(binary.LittleEndian.Uint16(data[28:30]))
		return version >= 10 && version <= 63 && nameSize > 0 && 30+nameSize+extraSize <= uint64(len(data))
	case bytes.HasPrefix(data, []byte("PK\x05\x06")):
		if len(data) < 22 {
			return false
		}
		commentSize := uint64(binary.LittleEndian.Uint16(data[20:22]))
		return binary.LittleEndian.Uint16(data[4:6]) == 0 &&
			binary.LittleEndian.Uint16(data[6:8]) == 0 &&
			binary.LittleEndian.Uint16(data[8:10]) == 0 &&
			binary.LittleEndian.Uint16(data[10:12]) == 0 &&
			binary.LittleEndian.Uint32(data[12:16]) == 0 &&
			binary.LittleEndian.Uint32(data[16:20]) == 0 &&
			22+commentSize <= uint64(len(data))
	default:
		return false
	}
}

// readHeader never follows links selected by the walker: callers must first
// require a regular file. No executable bit is used as evidence.
func readHeader(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, headerReadLimit))
	if err != nil {
		return nil, err
	}
	// A PE header may be beyond the initial window. Read only the fixed
	// COFF header and enough optional header bytes to validate the format.
	if len(data) >= 64 && string(data[:2]) == "MZ" {
		offset := int64(binary.LittleEndian.Uint32(data[60:64]))
		if offset >= 64 && peOutsideWindow(data, uint64(offset)) {
			extra := make([]byte, 24+112)
			n, readErr := f.ReadAt(extra, offset)
			if info, statErr := f.Stat(); statErr == nil && offset <= info.Size() && (readErr == nil || readErr == io.EOF) {
				data = normalizePEWindow(data, extra[:n], uint64(info.Size())-uint64(offset))
			}
		}
	}
	return data, nil
}

// normalizePEWindow relocates the bounded PE validation bytes into the first
// window; the content itself is never executed or otherwise interpreted.
func normalizePEWindow(data, pe []byte, available uint64) []byte {
	if len(data) < 64 || len(pe) < 26 || string(pe[:4]) != "PE\x00\x00" {
		return data
	}
	optionalSize := binary.LittleEndian.Uint16(pe[20:22])
	var min uint16
	switch binary.LittleEndian.Uint16(pe[24:26]) {
	case 0x10b:
		min = 96
	case 0x20b:
		min = 112
	case 0x107:
		min = 56
	}
	if min == 0 || optionalSize < min || len(pe) < 24+int(min) || uint64(24)+uint64(optionalSize) > available {
		return data
	}
	// The classifier checks the declared size; cap it to the validated prefix.
	out := make([]byte, 64+24+int(min))
	copy(out, data[:64])
	copy(out[64:], pe[:24+int(min)])
	binary.LittleEndian.PutUint32(out[60:64], 64)
	binary.LittleEndian.PutUint16(out[84:86], min)
	return out
}

func peOutsideWindow(data []byte, offset uint64) bool {
	if offset+26 > headerReadLimit {
		return true
	}
	if offset+22 > uint64(len(data)) {
		return false
	}
	size := uint64(binary.LittleEndian.Uint16(data[offset+20 : offset+22]))
	return offset+24+size > headerReadLimit
}

func normalizePEData(data []byte) []byte {
	if len(data) < 64 || string(data[:2]) != "MZ" {
		return data
	}
	offset := uint64(binary.LittleEndian.Uint32(data[60:64]))
	if offset < 64 || !peOutsideWindow(data, offset) || offset >= uint64(len(data)) {
		return data
	}
	return normalizePEWindow(data, data[offset:], uint64(len(data))-offset)
}

// A normal CPython cache name and an adjacent regular source file are weaker
// evidence than orphaned bytecode. Symlinks are not followed for this check.
func sourceBackedBytecode(path string) bool {
	name := filepath.Base(path)
	dir := filepath.Dir(path)
	var source string
	if filepath.Base(dir) == "__pycache__" {
		i := strings.Index(name, ".cpython-")
		if i <= 0 || !strings.HasSuffix(name, ".pyc") {
			return false
		}
		source = filepath.Join(filepath.Dir(dir), name[:i]+".py")
	} else if strings.HasSuffix(name, ".pyc") {
		source = strings.TrimSuffix(path, ".pyc") + ".py"
	} else {
		return false
	}
	fi, err := os.Lstat(source)
	return err == nil && fi.Mode().IsRegular()
}

func pythonLikePrefix(data []byte) bool {
	if len(data) < 4 || data[2] != '\r' || data[3] != '\n' {
		return false
	}
	if _, known := pythonMagicVersions[binary.LittleEndian.Uint16(data[:2])]; known {
		return true
	}
	// Whitespace followed by CRLF is ordinary text, not bytecode evidence.
	return bytes.IndexFunc(data[:2], func(r rune) bool {
		return r == utf8.RuneError || (unicode.IsControl(r) && !unicode.IsSpace(r))
	}) >= 0
}

// classifyHeader returns at most one finding, plus whether the format is
// opaque binary content. The caller keeps that content out of text analysis.
func classifyHeader(data []byte, path string) ([]Finding, bool) {
	if len(data) > headerReadLimit {
		data = data[:headerReadLimit]
	}
	kind, uncertain, archive, script, bytecode := "", false, false, false, false
	switch {
	case bytes.HasPrefix(data, []byte("\x7fELF")):
		kind = "ELF"
		uncertain = !validELFHeader(data)
	case len(data) >= 4 && (string(data[:4]) == "\xfe\xed\xfa\xce" || string(data[:4]) == "\xce\xfa\xed\xfe" || string(data[:4]) == "\xfe\xed\xfa\xcf" || string(data[:4]) == "\xcf\xfa\xed\xfe"):
		kind = "Mach-O"
		var order binary.ByteOrder = binary.BigEndian
		if data[0] == 0xce || data[0] == 0xcf {
			order = binary.LittleEndian
		}
		minSize := 28
		if data[0] == 0xcf || data[3] == 0xcf {
			minSize = 32
		}
		uncertain = len(data) < minSize
		if !uncertain {
			fileType := order.Uint32(data[12:16])
			uncertain = order.Uint32(data[4:8]) == 0 || fileType < 1 || fileType > 12
		}
	case len(data) >= 4 && (string(data[:4]) == "\xca\xfe\xba\xbe" || string(data[:4]) == "\xbe\xba\xfe\xca" || string(data[:4]) == "\xca\xfe\xba\xbf" || string(data[:4]) == "\xbf\xba\xfe\xca"):
		kind = "Mach-O universal"
		if len(data) < 8 {
			uncertain = true
			break
		}
		var order binary.ByteOrder = binary.BigEndian
		if data[0] == 0xbe || data[0] == 0xbf {
			order = binary.LittleEndian
		}
		count := order.Uint32(data[4:8])
		// Java class files share CAFEBABE but have a version here, not a
		// small architecture count. Do not call those native executables.
		if count > 32 {
			return nil, isBinaryContent(data)
		}
		if count == 0 {
			uncertain = true
			break
		}
		entrySize := 20
		if data[0] == 0xbf || data[3] == 0xbf {
			entrySize = 32
		}
		tableEnd := 8 + uint64(count)*uint64(entrySize)
		uncertain = uint64(len(data)) < tableEnd
		if !uncertain {
			for i := uint64(0); i < uint64(count); i++ {
				entryStart := 8 + i*uint64(entrySize)
				if order.Uint32(data[entryStart:entryStart+4]) == 0 {
					uncertain = true
					break
				}
			}
		}
	case bytes.HasPrefix(data, []byte("MZ")):
		if len(data) > 2 && !isBinaryContent(data) {
			return nil, false
		}
		kind, uncertain = "PE/DOS-like", true
		if len(data) >= 64 {
			offset := uint64(binary.LittleEndian.Uint32(data[60:64]))
			if offset >= 64 && offset+26 <= uint64(len(data)) && string(data[offset:offset+4]) == "PE\x00\x00" && binary.LittleEndian.Uint16(data[offset+4:offset+6]) != 0 && binary.LittleEndian.Uint16(data[offset+6:offset+8]) != 0 {
				optionalSize := uint64(binary.LittleEndian.Uint16(data[offset+20 : offset+22]))
				optionalStart := offset + 24
				if optionalSize >= 2 && optionalStart+optionalSize <= uint64(len(data)) {
					var minOptionalSize uint64
					switch binary.LittleEndian.Uint16(data[optionalStart : optionalStart+2]) {
					case 0x010b:
						minOptionalSize = 96
					case 0x020b:
						minOptionalSize = 112
					case 0x0107:
						minOptionalSize = 56
					}
					if minOptionalSize != 0 && optionalSize >= minOptionalSize {
						kind, uncertain = "PE", false
					}
				}
			}
		}
	case bytes.HasPrefix(data, []byte("\x00asm")):
		kind = "WASM"
		uncertain = len(data) < 8 || string(data[4:8]) != "\x01\x00\x00\x00"
	case pythonLikePrefix(data):
		kind = "Python bytecode-like"
		if version, ok := pythonMagicVersions[binary.LittleEndian.Uint16(data[:2])]; ok {
			kind, bytecode = "CPython "+version+" bytecode", true
		}
		if bytecode && binary.LittleEndian.Uint16(data[:2]) == 3379 {
			uncertain = len(data) < 12 // Python 3.6 predates the flags field.
		} else {
			uncertain = !bytecode || len(data) < 16 || binary.LittleEndian.Uint32(data[4:8]) & ^uint32(3) != 0
		}
	case bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06")) || bytes.HasPrefix(data, []byte("PK\x07\x08")):
		if !validZIPHeader(data) {
			return nil, isBinaryContent(data)
		}
		kind, archive = "ZIP/JAR archive", true
	case bytes.HasPrefix(data, []byte("#!")):
		line, _, _ := bytes.Cut(data[2:], []byte("\n"))
		if len(bytes.TrimSpace(line)) == 0 || !disguisedPath(path) {
			return nil, isBinaryContent(data)
		}
		kind, script = "shebang script", true
	default:
		return nil, isBinaryContent(data)
	}
	pattern, severity := "executable-artifact", SeverityMedium
	if disguisedPath(path) {
		pattern, severity = "disguised-executable", SeverityHigh
	}
	if bytecode {
		pattern, severity = "python-bytecode", SeverityHigh
	}
	if archive {
		pattern, severity = "archive-artifact", SeverityInfo
	}
	if uncertain {
		pattern, severity = "uncertain-header", SeverityInfo
	}
	message := kind + " header detected; runtime behavior has not been analyzed"
	if archive {
		message = kind + " signature detected; contents have not been inspected"
	}
	if uncertain {
		message = kind + " header is unknown, truncated, invalid or outside the 512-byte window; format is uncertain"
	}
	return []Finding{{Severity: severity, Pattern: pattern, RuleID: pattern,
		Message: message, File: path, Analyzer: AnalyzerStatic, Category: CategoryRisk, Confidence: 1.0}}, isBinaryContent(data) || (!script && !uncertain)
}

// inspectHeader keeps collection accounting independent of analyzer policy.
func (r *Result) inspectHeader(path, classifyPath, displayPath string, size int64, enabled bool, disabled map[string]bool, content ...[]byte) (bool, error) {
	var data []byte
	if len(content) > 0 {
		data = normalizePEData(content[0])
	} else {
		var err error
		data, err = readHeader(path)
		if err != nil {
			return false, fmt.Errorf("read file header %s: %w", displayPath, err)
		}
	}
	findings, opaque := classifyHeader(data, classifyPath)
	for i := range findings {
		if findings[i].Pattern == "python-bytecode" && sourceBackedBytecode(path) {
			findings[i].Severity = SeverityMedium
		}
		findings[i].File = displayPath
	}
	if enabled {
		r.Findings = append(r.Findings, filterDisabledFindings(findings, disabled)...)
	}
	if opaque {
		r.BinaryFiles++
	}
	if size > maxScanFileSize {
		r.OversizedFiles++
	}
	skipped := opaque || size > maxScanFileSize || !isScannable(filepath.Base(displayPath))
	if skipped {
		r.SkippedFiles++
	}
	return skipped, nil
}
