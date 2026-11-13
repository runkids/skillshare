package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanSkill_IgnoresAdministrativeExtrasLock(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	ResetGlobalCache()
	t.Cleanup(ResetGlobalCache)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rules.md"), []byte("# Be concise\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := ScanSkill(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".skillshare-extras.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	after, err := ScanSkill(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("administrative lock changed audit coverage/result: before=%+v after=%+v", before, after)
	}
}
