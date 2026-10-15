package values

import (
	"encoding"

	"github.com/rotisserie/eris"
)

var ErrInvalidRole = eris.New("invalid role")

var _ encoding.TextUnmarshaler = (*Role)(nil)

// Role is ordered: each role may do everything the ones below it may.
// The zero value is the least privileged, so an unset role grants nothing extra.
type Role int

const (
	// RoleUser may only read.
	RoleUser Role = iota
	// RoleAdmin may read and write, except user management.
	RoleAdmin
	// RoleOwner is the first user, the only one who manages users.
	RoleOwner
)

var mapTextToRole = map[string]Role{
	"user":  RoleUser,
	"admin": RoleAdmin,
	"owner": RoleOwner,
}

var mapRoleToText = map[Role]string{
	RoleUser:  "user",
	RoleAdmin: "admin",
	RoleOwner: "owner",
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (r *Role) UnmarshalText(text []byte) error {
	if role, ok := mapTextToRole[string(text)]; ok {
		*r = role

		return nil
	}

	return ErrInvalidRole
}

func (r Role) String() string {
	if text, ok := mapRoleToText[r]; ok {
		return text
	}

	return "unknown"
}
