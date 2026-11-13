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
	if !strings.Contains(raw, "://") {
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
	}
	if rule != "" {
		return fmt.Errorf("%w (%s)", ErrSourceCredentials, rule)
	}
	return nil
}
