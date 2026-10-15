package entities

import (
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/LiquidCats/hive/internal/kernel/values"
)

// User implements [webauthn.User]; the go-webauthn types are this module's passkey vocabulary.
type User struct {
	ID          values.UserID
	Name        values.Username
	Credentials []webauthn.Credential
	// Disabled is set for users awaiting approval and for disabled ones; neither may sign in.
	Disabled  bool
	CreatedAt time.Time
}

var _ webauthn.User = User{}

func (u User) WebAuthnID() []byte                         { return u.ID[:] }
func (u User) WebAuthnName() string                       { return u.Name.String() }
func (u User) WebAuthnDisplayName() string                { return u.Name.String() }
func (u User) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }
