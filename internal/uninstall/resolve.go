package uninstall

import (
	"path/filepath"
	"strings"

	"skillshare/internal/install"
)

// ResolveName selects an exact declaration without a checkout before disk,
// then tries declaration shorthand. A nil store disables declaration removal
// (project mode). disk returns an empty Item for no match, or an error for a
// refusal that must not fall back to a declaration. Record-only intent requires
// an exact declaration and refuses a checkout independently of the store.
func ResolveName(sourceDir string, store *install.MetadataStore, name string, recordOnly bool, disk func() (Item, error)) (Item, error) {
	name = strings.TrimSpace(name)
	if recordOnly {
		if err := CheckRecordOnly(sourceDir, name); err != nil {
			return Item{}, err
		}
	}
	var declaration string
	var exact bool
	var declarationErr error
	if store != nil {
		declaration, exact, declarationErr = install.FindTrackedDeclaration(sourceDir, store, name)
	}
	if recordOnly && !exact {
		return Item{}, ErrNotDeclared
	}
	if exact {
		return declarationItem(sourceDir, declaration), nil
	}
	item, err := disk()
	if err != nil || item.Path != "" {
		return item, err
	}
	if declarationErr != nil {
		return Item{}, declarationErr
	}
	if declaration != "" {
		return declarationItem(sourceDir, declaration), nil
	}
	return Item{}, ErrNotDeclared
}

func declarationItem(sourceDir, name string) Item {
	return Item{Name: name, Path: filepath.Join(sourceDir, filepath.FromSlash(name)), MetadataOnly: true}
}

// CheckRecordOnly refuses stale record-only intent independently of metadata
// loading, confirmation, and force. Run checks it again at execution time.
func CheckRecordOnly(sourceDir, name string) error {
	if install.IsTrackedCheckout(filepath.Join(sourceDir, filepath.FromSlash(name))) {
		return ErrCheckoutRestored
	}
	return nil
}
