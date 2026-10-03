package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertPiRecordPrivate(t *testing.T, file string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Dir(file), file} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil {
			t.Fatalf("missing private DACL: %v", err)
		}
		ownerAccess := false
		for i := uint32(0); i < uint32(acl.AceCount); i++ {
			var ace *windows.ACCESS_ALLOWED_ACE
			if err := windows.GetAce(acl, i, &ace); err != nil {
				t.Fatal(err)
			}
			if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
				continue
			}
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if sid.Equals(user.User.Sid) {
				ownerAccess = true
				continue
			}
			if sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
				continue
			}
			if ace.Mask != 0 {
				t.Fatalf("private registration grants another principal access: %s", sd.String())
			}
		}
		if !ownerAccess {
			t.Fatal("private registration does not grant its owner access")
		}
	}
}

func setPiTestPublicACL(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPiRegistrationWindowsRefusesUnsafeDirectoryWithoutChangingACL(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:demo", "extensions": []string{"-a.ts"}}}})
	dir := filepath.Join(f.svc.StateDir, "pi-registrations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	setPiTestPublicACL(t, dir)
	before, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	same := unchanged(t, f.svc.ConfigPath, f.settingsPath())
	req := Request{Action: "import", From: "pi", Plugin: "npm:demo"}
	plan, err := f.svc.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Apply(context.Background(), req, plan.Revision); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("unsafe private directory accepted: %v", err)
	}
	same()
	after, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || before.String() != after.String() {
		t.Fatalf("existing ACL changed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("raw record written in unsafe directory: %v", err)
	}
}

func TestPiRegistrationWindowsRejectsBroadExistingACL(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:demo", "extensions": []string{"-a.ts"}}}})
	applyPluginRequest(t, f.svc, Request{Action: "import", From: "pi", Plugin: "npm:demo"})
	d, err := f.svc.load()
	if err != nil {
		t.Fatal(err)
	}
	b := d.packages["demo"].Bindings["pi"]
	file, err := f.svc.piRegistrationPath(b.PiRegistration)
	if err != nil {
		t.Fatal(err)
	}
	setPiTestPublicACL(t, file)
	same := unchanged(t, file, f.svc.ConfigPath, f.settingsPath())
	if _, err := f.svc.readPiRegistration(b.PiRegistration, "pi", b.ID); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("broadly accessible record accepted: %v", err)
	}
	// Re-import must not silently rewrite an existing record or change its ACL.
	p, err := f.svc.Preview(context.Background(), Request{Action: "import", From: "pi", Plugin: b.ID})
	if err == nil && !p.Blocked {
		if _, err = f.svc.Apply(context.Background(), Request{Action: "import", From: "pi", Plugin: b.ID}, p.Revision); err == nil {
			t.Fatal("unsafe existing record reused")
		}
	}
	same()
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
}
