package install

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// MergeMetadata three-way merges .metadata.json contents per entry, for a git
// merge where two machines both installed or updated skills. An entry changed
// on one side only takes that side; changed on both, the later installed_at
// wins (the remote on a tie), and an edit beats a delete. An empty base means
// the file did not exist in the common ancestor. The Herdr panel mirrors this
// rule in herdr/skillshare/mods/skillshare/model.js (mergeMetadata); keep both in step.
func MergeMetadata(base, ours, theirs []byte) ([]byte, error) {
	parse := func(data []byte, side string) (*MetadataStore, error) {
		store := NewMetadataStore()
		if len(data) == 0 {
			return store, nil
		}
		if err := json.Unmarshal(data, store); err != nil {
			return nil, fmt.Errorf("parse %s metadata: %w", side, err)
		}
		if store.Entries == nil {
			store.Entries = make(map[string]*MetadataEntry)
		}
		return store, nil
	}
	b, err := parse(base, "base")
	if err != nil {
		return nil, err
	}
	o, err := parse(ours, "local")
	if err != nil {
		return nil, err
	}
	th, err := parse(theirs, "remote")
	if err != nil {
		return nil, err
	}

	merged := th
	for name, mine := range o.Entries {
		old, remote := b.Entries[name], th.Entries[name]
		switch {
		case reflect.DeepEqual(mine, old): // unchanged here: the remote side stands
		case reflect.DeepEqual(remote, old) || remote == nil || mine.InstalledAt.After(remote.InstalledAt):
			merged.Entries[name] = mine
		}
	}
	for name := range th.Entries {
		if _, kept := o.Entries[name]; !kept && b.Entries[name] != nil && reflect.DeepEqual(th.Entries[name], b.Entries[name]) {
			delete(merged.Entries, name) // deleted here, untouched remotely
		}
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return append(data, '\n'), nil
}
