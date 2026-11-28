package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rexInitRecord is the machine-local account of what Skillshare did to init.lua.
// Rex init files are often synced between machines, so provenance cannot live there.
type rexInitRecord struct {
	CreatedInit  bool `json:"created_init"`
	InsertedLine bool `json:"inserted_line"`
}

func rexRecordPath(root string) string {
	return filepath.Join(root, ".skillshare-rex-init.json")
}

// rexReadRecord returns the zero record when none exists, so a lost record
// removes nothing Skillshare cannot prove it added.
func rexReadRecord(root string) (rexInitRecord, bool, error) {
	path := rexRecordPath(root)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return rexInitRecord{}, false, nil
	}
	if err != nil {
		return rexInitRecord{}, false, fmt.Errorf("%s: %w", path, err)
	}
	rec, err := rexDecodeRecord(path, data)
	return rec, err == nil, err
}

// rexInitState loads the record; with no record and no loader, nothing of ours is
// wired yet, so this install defines what Skillshare itself will add.
func rexInitState(root string, hasLoader bool, original string, initExisted bool) (rexInitRecord, bool, error) {
	rec, found, err := rexReadRecord(root)
	if err != nil || found || hasLoader {
		return rec, found, err
	}
	return rexInitRecord{CreatedInit: !initExisted, InsertedLine: rexLineIndex(original) < 0}, false, nil
}

func rexWriteRecord(root string, rec rexInitRecord) error {
	data, err := json.Marshal(rexVersionedRecord{Version: 1, rexInitRecord: rec})
	if err != nil {
		return err
	}
	return atomicNativeWrite(root, rexRecordPath(root), append(data, '\n'), 0644)
}

func rexRemoveRecord(root string) error {
	if err := os.Remove(rexRecordPath(root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func rexRemoveLine(content string) string {
	if i := rexLineIndex(content); i >= 0 {
		return content[:i] + content[i+len(rexLine):]
	}
	return content
}

// rexInsertLine adds the loader line at the top, after any shebang.
func rexInsertLine(content string) (string, error) {
	offset := 0
	if strings.HasPrefix(content, "\xef\xbb\xbf") {
		offset = 3
	}
	if strings.HasPrefix(content[offset:], "#!") {
		i := strings.IndexByte(content[offset:], '\n')
		if i < 0 {
			return content + "\n" + rexLine, nil
		}
		offset += i + 1
	}
	return content[:offset] + rexLine + content[offset:], nil
}

// rexVersionedRecord accepts legacy records without a version while writing v1.
type rexVersionedRecord struct {
	Version int `json:"version"`
	rexInitRecord
}

func rexDecodeRecord(path string, data []byte) (rexInitRecord, error) {
	var rec rexVersionedRecord
	var version struct {
		Version *int `json:"version"`
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rexInitRecord{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return rexInitRecord{}, fmt.Errorf("%s: %w", path, err)
	}
	if version.Version != nil && *version.Version != 1 {
		return rexInitRecord{}, fmt.Errorf("%s: unsupported Rex init record version %d", path, *version.Version)
	}
	return rec.rexInitRecord, nil
}
