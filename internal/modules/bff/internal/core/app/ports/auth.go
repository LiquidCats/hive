package ports

import (
	"context"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainvalues "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

// Auth is the inbound port: passkey registration, login and sessions.
type Auth interface {
	BeginRegistration(ctx context.Context, username values.Username) (*protocol.CredentialCreation, error)
	// FinishRegistration creates the user and returns a session token;
	// [errors.ErrApprovalPending] (user created, no session) if the user awaits approval.
	FinishRegistration(ctx context.Context, resp *protocol.ParsedCredentialCreationData) (string, error)
	// BeginLogin starts a usernameless (discoverable passkey) login.
	BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, error)
	// FinishLogin returns a session token; [errors.ErrApprovalPending] if the user is not approved.
	FinishLogin(ctx context.Context, resp *protocol.ParsedCredentialAssertionData) (string, error)
	// Session returns the user signed in with the token;
	// [errors.ErrUnauthenticated] if it is unknown, expired or the user is disabled.
	Session(ctx context.Context, token string) (*entities.User, error)
	// Logout ends the session; unknown tokens are ignored.
	Logout(ctx context.Context, token string) error
}

// Users is the inbound port: user management. Every method returns [errors.ErrForbidden]
// unless actor, the signed-in user, is the owner.
type Users interface {
	// ListUsers returns a page of users after the given ID (zero for the first page), oldest first,
	// and the ID to pass for the next page; zero on the last page.
	ListUsers(ctx context.Context, actor *entities.User, after values.UserID) ([]entities.User, values.UserID, error)
	// EnableUser approves a pending user or re-enables a disabled one, as a user;
	// [errors.ErrUserNotFound] if missing.
	EnableUser(ctx context.Context, actor *entities.User, id values.UserID) error
	// DisableUser disables the user, demotes them to user and ends their sessions;
	// [errors.ErrCannotManageSelf] if actor is the user, [errors.ErrUserNotFound] if missing.
	DisableUser(ctx context.Context, actor *entities.User, id values.UserID) error
	// SetRole makes an active user an admin or a user; ownership cannot be granted ([domainvalues.ErrInvalidRole]).
	// [errors.ErrCannotManageSelf] if actor is the user, [errors.ErrUserNotFound] if missing,
	// [errors.ErrUserDisabled] if pending or disabled.
	SetRole(ctx context.Context, actor *entities.User, id values.UserID, role domainvalues.Role) error
}

// Repos is the inbound port: git repositories the system deploys from. Every signed-in user may read;
// writes need an admin. Methods return [errors.ErrForbidden] otherwise, before any lookup.
type Repos interface {
	// ListRepos returns a page of repositories after the given ID (zero for the first page), oldest first,
	// and the ID to pass for the next page; zero on the last page.
	ListRepos(ctx context.Context, actor *entities.User, after values.RepoID) ([]entities.Repo, values.RepoID, error)
	// GetRepo returns the repository; [errors.ErrRepoNotFound] if missing.
	GetRepo(ctx context.Context, actor *entities.User, id values.RepoID) (*entities.Repo, error)
	// CreateRepo validates the input, checks the remote is reachable with the credentials and stores it.
	// Errors: validation ([errors.ErrInvalidRepoURL], [errors.ErrInvalidRepoName], [errors.ErrInvalidRepoCredentials]),
	// [GitRemote] check errors, [errors.ErrRepoURLTaken].
	CreateRepo(ctx context.Context, actor *entities.User, in entities.RepoInput) (*entities.Repo, error)
	// UpdateRepo replaces the repository, keeping secrets the input omits; same errors as CreateRepo plus
	// [errors.ErrRepoNotFound] and [errors.ErrRepoCredentialsRequired].
	UpdateRepo(ctx context.Context, actor *entities.User, id values.RepoID, in entities.RepoInput) (*entities.Repo, error)
	// DeleteRepo deletes the repository; [errors.ErrRepoNotFound] if missing.
	DeleteRepo(ctx context.Context, actor *entities.User, id values.RepoID) error
}
