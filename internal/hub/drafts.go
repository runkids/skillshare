package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/gofrs/flock"
)

var ErrDraftConflict = errors.New("draft changed in another window; reload before saving")
var validDraftID = regexp.MustCompile(`^[a-f0-9]{32}$`)

// DraftStore is scoped to the active configuration directory.
type DraftStore struct{ Dir string }

func (s DraftStore) path(id string) (string, error) {
	if !validDraftID.MatchString(id) {
		return "", fmt.Errorf("invalid draft ID")
	}
	p := filepath.Join(s.Dir, id+".json")
	if err := noDraftSymlink(s.Dir); err != nil {
		return "", err
	}
	if err := noDraftSymlink(p); err != nil {
		return "", err
	}
	return p, nil
}

func noDraftSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("draft storage cannot be a symbolic link")
	}
	return nil
}

func (s DraftStore) Get(id string) (Draft, error) {
	path, err := s.path(id)
	if err != nil {
		return Draft{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Draft{}, err
	}
	var d Draft
	if err = json.Unmarshal(raw, &d); err != nil {
		return Draft{}, fmt.Errorf("read draft: %w", err)
	}
	return d, nil
}

func (s DraftStore) List() ([]Draft, error) {
	if err := noDraftSymlink(s.Dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return []Draft{}, nil
	}
	if err != nil {
		return nil, err
	}
	drafts := make([]Draft, 0)
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		id := name[:len(name)-5]
		if !validDraftID.MatchString(id) {
			continue
		}
		d, err := s.Get(id)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, d)
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].UpdatedAt > drafts[j].UpdatedAt })
	return drafts, nil
}

func (s DraftStore) locked(fn func() error) error {
	if err := noDraftSymlink(s.Dir); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	lockPath := filepath.Join(s.Dir, ".lock")
	if err := noDraftSymlink(lockPath); err != nil {
		return err
	}
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	return fn()
}

func (s DraftStore) Create(d Draft) (Draft, error) {
	id, err := draftID()
	if err != nil {
		return Draft{}, err
	}
	d.ID, d.Revision = id, ""
	if d.Entries == nil {
		d.Entries = []DraftEntry{}
	}
	return s.persist(d, true)
}

func (s DraftStore) Save(d Draft) (Draft, error) { return s.persist(d, false) }

func (s DraftStore) persist(d Draft, create bool) (Draft, error) {
	if err := validateDraft(d); err != nil {
		return Draft{}, err
	}
	err := s.locked(func() error {
		path, err := s.path(d.ID)
		if err != nil {
			return err
		}
		if !create {
			old, err := s.Get(d.ID)
			if err != nil {
				return err
			}
			if old.Revision != d.Revision {
				return ErrDraftConflict
			}
			origins := make(map[string]DraftEntry)
			for _, e := range old.Entries {
				origins[e.ID] = e
			}
			for _, e := range d.Entries {
				before, ok := origins[e.ID]
				if !ok || field(before.Data, "source") != field(e.Data, "source") || field(before.Data, "skill") != field(e.Data, "skill") {
					for _, k := range []string{"riskScore", "riskLabel", "auditedAt"} {
						delete(e.Data, k)
					}
				}
			}
		}
		d.Revision, err = draftID()
		if err != nil {
			return err
		}
		d.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		raw, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		tmp, err := os.CreateTemp(s.Dir, ".draft-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err = tmp.Write(raw); err != nil {
			tmp.Close()
			return err
		}
		if err = tmp.Sync(); err != nil {
			tmp.Close()
			return err
		}
		if err = tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), path)
	})
	return d, err
}

func (s DraftStore) Delete(id, revision string) error {
	return s.locked(func() error {
		d, err := s.Get(id)
		if err != nil {
			return err
		}
		if d.Revision != revision {
			return ErrDraftConflict
		}
		path, err := s.path(id)
		if err != nil {
			return err
		}
		return os.Remove(path)
	})
}
