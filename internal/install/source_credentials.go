package install

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrSourceCredentials identifies remote sources that contain credentials.
var ErrSourceCredentials = errors.New("remote source must not contain credentials")

// RejectSourceCredentials checks a remote without including it in errors.
func RejectSourceCredentials(raw string) error {
	raw = strings.TrimSpace(raw)
	if credentialFreeFileSource(raw) || isLocalPath(raw) || gitSSHPattern.MatchString(raw) {
		return nil
	}
	if !strings.Contains(raw, "://") {
		first := strings.SplitN(raw, "/", 2)[0]
		rule := ""
		switch {
		case strings.Contains(first, "@"):
			rule = "userinfo"
		case strings.Contains(first, ":") && !strings.HasPrefix(first, "ado:") && !extraHostPort(first):
			rule = "invalid URL"
		case strings.Contains(raw, "?"):
			rule = "query"
		case strings.Contains(raw, "#"):
			rule = "fragment"
		}
		if rule != "" {
			return fmt.Errorf("%w (%s)", ErrSourceCredentials, rule)
		}
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		authority := strings.SplitN(strings.SplitN(raw, "://", 2)[1], "/", 2)[0]
		if strings.Contains(authority, "@") {
			return fmt.Errorf("%w (userinfo)", ErrSourceCredentials)
		}
		// Parsing errors can hide query or fragment credentials. Do not return
		// the parser's error: it may contain the complete source URL.
		return fmt.Errorf("%w (invalid URL)", ErrSourceCredentials)
	}
	rule := ""
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		switch {
		case u.User != nil:
			rule = "userinfo"
		case u.RawQuery != "" || u.ForceQuery:
			rule = "query"
		case u.Fragment != "":
			rule = "fragment"
		}
	case "ssh":
		if u.User != nil {
			if _, ok := u.User.Password(); ok {
				rule = "password"
			}
		}
	default:
		if u.User != nil {
			rule = "userinfo"
		}
	}
	if rule != "" {
		return fmt.Errorf("%w (%s)", ErrSourceCredentials, rule)
	}
	return nil
}

func credentialFreeFileSource(raw string) bool {
	if !strings.HasPrefix(strings.ToLower(raw), "file://") {
		return false
	}
	authority, _, _ := strings.Cut(raw[len("file://"):], "/")
	return !strings.Contains(authority, "@")
}

func extraHostPort(first string) bool {
	if strings.Count(first, ":") != 1 {
		return false
	}
	host, port, _ := strings.Cut(first, ":")
	if host == "" || port == "" {
		return false
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
