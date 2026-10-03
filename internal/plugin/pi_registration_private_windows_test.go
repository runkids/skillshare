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
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(file, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
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
