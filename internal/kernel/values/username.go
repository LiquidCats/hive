package values

import (
	"regexp"
	"strings"
)

// ASCII only: no lookalike (homoglyph), invisible or control characters.
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

type Username string

func (u Username) Normalized() Username {
	return Username(strings.ToLower(strings.TrimSpace(u.String())))
}

func (u Username) IsValid() bool {
	return usernamePattern.MatchString(u.Normalized().String())
}

func (u Username) String() string {
	return string(u)
}
