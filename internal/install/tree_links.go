package install

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

// LinkTexts reads the text of each link, given as its path mapped to its blob
// id, from the repository at dir. In a blobless repository, reading a blob
// that is not there yet fetches it from the promisor remote.
func LinkTexts(dir string, oids map[string]string, extraEnv []string) (map[string]string, error) {
	links := make(map[string]string, len(oids))
	if len(oids) == 0 {
		return links, nil
	}
	paths := make([]string, 0, len(oids))
	var stdin strings.Builder
	for p := range oids {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		stdin.WriteString(oids[p] + "\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cat := gitCommand(ctx, "cat-file", "--batch")
	cat.Dir = dir
	cat.Env = append(cat.Env, extraEnv...)
	cat.Stdin = strings.NewReader(stdin.String())
	out, err := cat.Output()
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for _, p := range paths {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		// "<oid> blob <size>", then the content and a newline.
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("cannot read link %s: %s", p, strings.TrimSpace(header))
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, err
		}
		text := make([]byte, size+1)
		if _, err := io.ReadFull(r, text); err != nil {
			return nil, err
		}
		links[p] = string(text[:size])
	}
	return links, nil
}

// ResolveTreeLinks follows the links along p, a slash-separated path in a git
// tree whose links maps each link path to its text. It fails when a link is
// absolute, leads out of the tree, or the links loop.
func ResolveTreeLinks(p string, links map[string]string) (string, bool) {
	for hops := 0; hops <= len(links); hops++ {
		next, followed := resolveFirstTreeLink(p, links)
		if !followed {
			return p, true
		}
		if next == "" {
			return "", false
		}
		p = next
	}
	return "", false
}

// resolveFirstTreeLink replaces the first link along p with its target. It
// reports followed=false when p holds no link, and next="" when the link
// cannot be followed.
func resolveFirstTreeLink(p string, links map[string]string) (next string, followed bool) {
	parts := strings.Split(p, "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		text, ok := links[prefix]
		if !ok {
			continue
		}
		if text == "" || strings.HasPrefix(text, "/") {
			return "", true
		}
		next = path.Join(path.Dir(prefix), text, strings.Join(parts[i+1:], "/"))
		if next == ".." || strings.HasPrefix(next, "../") {
			return "", true
		}
		return next, true
	}
	return "", false
}
