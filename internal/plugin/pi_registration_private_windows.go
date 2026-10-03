package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func makePiRegistrationDir(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	// The new directory is private from creation, before any raw entry is written.
	// Never change an existing directory's ACL or its owner's permissions.
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs := windows.SecurityAttributes{SecurityDescriptor: sd}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	if err := windows.CreateDirectory(name, &attrs); err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return err
	}
	return checkPiRegistrationPrivate(path)
}

func checkPiRegistrationPrivate(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	// Elevated Windows tokens can default new files' owner to Administrators.
	// These privileged owners still require an explicit current-user DACL below.
	if err != nil || owner == nil || (!owner.Equals(user.User.Sid) && !owner.IsWellKnown(windows.WinLocalSystemSid) && !owner.IsWellKnown(windows.WinBuiltinAdministratorsSid)) {
		return errors.New("Pi private state belongs to another Windows owner")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errors.New("Pi private state has no verifiable Windows DACL")
	}
	ownerAccess := false
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("Pi private state has unsupported Windows access rules")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.Equals(user.User.Sid) {
			ownerAccess = ownerAccess || ace.Mask != 0
			continue
		}
		// SYSTEM/administrators are privileged principals, like root on Unix.
		if sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid) {
			continue
		}
		if ace.Mask != 0 {
			return errors.New("Pi private state grants access to other Windows users; preserve it and repair its ACL before retrying")
		}
	}
	if !ownerAccess {
		return errors.New("Pi private state does not grant its Windows owner access")
	}
	return nil
}
