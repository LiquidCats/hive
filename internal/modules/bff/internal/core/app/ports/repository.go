package ports

import (
	"context"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
)

type UserRepository interface {
	// CreateUser stores the user with its credentials atomically and returns the stored user:
	// the first user is active, later ones are disabled until approved;
	// [errors.ErrUsernameTaken] on duplicate name.
	CreateUser(ctx context.Context, user entities.User) (*entities.User, error)
	// GetUserByName returns the user without credentials;
	// [errors.ErrUserNotFound] if missing.
	GetUserByName(ctx context.Context, name values.Username) (*entities.User, error)
	// GetUserByID returns the user with its active credentials;
	// [errors.ErrUserNotFound] if missing.
	GetUserByID(ctx context.Context, id values.UserID) (*entities.User, error)
	// ListUsers returns all users without credentials, oldest first.
	ListUsers(ctx context.Context) ([]entities.User, error)
	// DisableUserByID disables the user and deletes their sessions atomically;
	// [errors.ErrUserNotFound] if missing.
	DisableUserByID(ctx context.Context, id values.UserID) error
	// EnableUserByID enables the user; [errors.ErrUserNotFound] if missing.
	EnableUserByID(ctx context.Context, id values.UserID) error
}

type SessionRepository interface {
	// CreateSession stores the session and purges expired ones.
	CreateSession(ctx context.Context, session entities.Session) error
	// GetSessionUser returns the session's user (without credentials) and expiry;
	// [errors.ErrUnauthenticated] if missing.
	GetSessionUser(ctx context.Context, tokenHash string) (*entities.User, time.Time, error)
	// DeleteSession deletes the session; missing ones are ignored.
	DeleteSession(ctx context.Context, tokenHash string) error
}

type CredentialRepository interface {
	// UpdateCredential stores the post-login state (sign counter, flags) of a credential.
	UpdateCredential(ctx context.Context, credential webauthn.Credential) error
	// RevokeCredentialByID revokes the credential with the given ID.
	RevokeCredentialByID(ctx context.Context, id values.CredentialID) error
}

type CeremonyRepository interface {
	// CreateCeremony stores the ceremony.
	CreateCeremony(ctx context.Context, ceremony entities.Ceremony) error
	// TakeCeremony deletes and returns the ceremony for the challenge, so it can be used once;
	// [errors.ErrCeremonyNotFound] if missing.
	TakeCeremony(ctx context.Context, challenge string) (*entities.Ceremony, error)
}
