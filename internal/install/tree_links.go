package install

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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

// maxTreeLinkHops bounds the links ResolveTreeLinks follows, as the 40 of
// path resolution on Linux, so links that loop fail.
const maxTreeLinkHops = 40

// ResolveTreeLinks follows the links along p, a slash-separated path in a git
// tree, one component at a time as the filesystem does, so a ".." after a
// link leaves the folder the link leads to. lookup reports whether a path is
// a link and its text. It fails when a link is empty or absolute, the path
// leads out of the tree or to its root, the links loop, or lookup fails.
func ResolveTreeLinks(p string, lookup func(string) (text string, isLink bool, err error)) (string, bool) {
	var resolved []string
	pending := strings.Split(p, "/")
	hops := 0
	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return "", false
			}
			resolved = resolved[:len(resolved)-1]
			continue
		}
		cur := part
		if len(resolved) > 0 {
			cur = strings.Join(resolved, "/") + "/" + part
		}
		text, isLink, err := lookup(cur)
		if err != nil {
			return "", false
		}
		if !isLink {
			resolved = append(resolved, part)
			continue
		}
		hops++
		if hops > maxTreeLinkHops || text == "" || strings.HasPrefix(text, "/") {
			return "", false
		}
		pending = append(strings.Split(text, "/"), pending...)
	}
	if len(resolved) == 0 {
		return "", false
	}
	return strings.Join(resolved, "/"), true
}
