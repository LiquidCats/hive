package ports

import (
	"context"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainvalues "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

type UserRepository interface {
	// CreateUser stores the user with its credentials atomically and returns the stored user:
	// the first user is the active owner, later ones are users, disabled until approved;
	// [errors.ErrUsernameTaken] on duplicate name.
	CreateUser(ctx context.Context, user entities.User) (*entities.User, error)
	// GetUserByName returns the user without credentials;
	// [errors.ErrUserNotFound] if missing.
	GetUserByName(ctx context.Context, name values.Username) (*entities.User, error)
	// GetUserByID returns the user with its active credentials;
	// [errors.ErrUserNotFound] if missing.
	GetUserByID(ctx context.Context, id values.UserID) (*entities.User, error)
	// ListUsers returns up to limit users after the given ID, without credentials, oldest first;
	// the zero ID starts at the first user.
	ListUsers(ctx context.Context, after values.UserID, limit int) ([]entities.User, error)
	// DisableUserByID disables the user, demotes them to user and deletes their sessions atomically;
	// [errors.ErrUserNotFound] if missing.
	DisableUserByID(ctx context.Context, id values.UserID) error
	// EnableUserByID enables the user; [errors.ErrUserNotFound] if missing.
	EnableUserByID(ctx context.Context, id values.UserID) error
	// SetUserRoleByID sets an active user's role;
	// [errors.ErrUserNotFound] if missing, [errors.ErrUserDisabled] if pending or disabled.
	SetUserRoleByID(ctx context.Context, id values.UserID, role domainvalues.Role) error
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

type RepoRepository interface {
	// CreateRepo stores the repository and returns it; [errors.ErrRepoURLTaken] on duplicate URL.
	CreateRepo(ctx context.Context, repo entities.Repo) (*entities.Repo, error)
	// GetRepoByID returns the repository; [errors.ErrRepoNotFound] if missing.
	GetRepoByID(ctx context.Context, id values.RepoID) (*entities.Repo, error)
	// ListRepos returns up to limit repositories after the given ID, oldest first;
	// the zero ID starts at the first one.
	ListRepos(ctx context.Context, after values.RepoID, limit int) ([]entities.Repo, error)
	// UpdateRepo replaces the stored repository and returns it;
	// [errors.ErrRepoNotFound] if missing, [errors.ErrRepoURLTaken] on duplicate URL.
	UpdateRepo(ctx context.Context, repo entities.Repo) (*entities.Repo, error)
	// DeleteRepoByID deletes the repository; [errors.ErrRepoNotFound] if missing.
	DeleteRepoByID(ctx context.Context, id values.RepoID) error
}

type GitRemote interface {
	// Check lists the remote's refs with the repository's credentials; an empty repository is reachable.
	// [errors.ErrRepoAccessDenied] if it is missing or the credentials are rejected,
	// [errors.ErrInvalidRepoCredentials] if the SSH key does not parse, [errors.ErrRepoUnreachable] otherwise.
	Check(ctx context.Context, repo entities.Repo) error
}
