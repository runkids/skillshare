package gitenv

import (
	"slices"
	"testing"
)

func TestEnvironDropsRepositorySelection(t *testing.T) {
	t.Setenv("GIT_DIR", "/caller/.git")
	t.Setenv("GIT_WORK_TREE", "/caller")
	t.Setenv("GIT_INDEX_FILE", "/caller/.git/index")

	for _, item := range Environ() {
		if item == "GIT_DIR=/caller/.git" || item == "GIT_WORK_TREE=/caller" || item == "GIT_INDEX_FILE=/caller/.git/index" {
			t.Fatalf("Environ() kept %s", item)
		}
	}
}

// Users pass credentials through GIT_CONFIG_COUNT, and install's token auth
// appends to it.
func TestEnvironKeepsConfigVariables(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "http.proxy")
	t.Setenv("GIT_CONFIG_VALUE_0", "http://proxy")

	env := Environ()
	for _, want := range []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.proxy", "GIT_CONFIG_VALUE_0=http://proxy"} {
		if !slices.Contains(env, want) {
			t.Errorf("Environ() dropped %s", want)
		}
	}
}
