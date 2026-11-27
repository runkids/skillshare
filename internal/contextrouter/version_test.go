package contextrouter

import "testing"

func TestParseScriptVersion(t *testing.T) {
	for text, want := range map[string]string{
		"VERSION = '0.5.0'  # x-release-please-version\n": "0.5.0",
		"import os\n\nVERSION = \"0.6.0\"\n":              "0.6.0",
		"VERSION='1.2.3'":                                 "1.2.3",
		"VERSION \t=\t'10.20.30'\n":                       "10.20.30",
		"VERSION = '0.2.1'\r\nrest":                       "0.2.1",
		"VERSION = '0.2.1'\nVERSION = '9.9.9'\n":          "0.2.1",
	} {
		got, ok := parseScriptVersion(text)
		if !ok || got.String() != want {
			t.Errorf("%q: got %v %v, want %s", text, got, ok, want)
		}
	}
	for _, text := range []string{
		"",
		"print('no version')\n",
		"VERSION = '0.6.0-rc.1'\n", // a prerelease is not a release this port has goldens for
		"VERSION = '0.6'\n",
		"VERSION = 0.6.0\n",
		"  VERSION = '0.6.0'\n",
		"# VERSION = '0.6.0'\n",
		"MY_VERSION = '0.6.0'\n",
		"VERSION = 'garbage'\n",
		"VERSION = '99999999999999999999.0.0'\n",
	} {
		if got, ok := parseScriptVersion(text); ok {
			t.Errorf("%q parsed as %v", text, got)
		}
	}
	if !(version{0, 4, 9}).less(version{0, 5, 0}) || (version{0, 5, 0}).less(version{0, 5, 0}) || !(version{0, 9, 9}).less(version{1, 0, 0}) {
		t.Error("version order")
	}
}

func TestScriptVersionFile(t *testing.T) {
	dir := t.TempDir()
	root := openRoot(t, dir)
	if _, ok := scriptVersion(root, 100); ok {
		t.Error("missing script has a version")
	}
	writeFile(t, dir+"/scripts/ai-context.py", "VERSION = '0.6.0'\n")
	if got, ok := scriptVersion(root, 100); !ok || got != (version{0, 6, 0}) {
		t.Errorf("got %v %v", got, ok)
	}
	if _, ok := scriptVersion(root, 5); ok {
		t.Error("a script over the limit was read")
	}
}
