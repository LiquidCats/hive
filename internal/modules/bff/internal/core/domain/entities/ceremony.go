package entities

import (
	"github.com/go-webauthn/webauthn/webauthn"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

// Ceremony is an in-flight registration or login, keyed by its challenge (Session.Challenge).
type Ceremony struct {
	ID   kernelvalues.CeremonyID
	Type values.CeremonyType
	// Username is the name being registered; empty for logins.
	Username kernelvalues.Username
	Session  webauthn.SessionData
}
