package ports

import (
	"context"

	"github.com/go-webauthn/webauthn/protocol"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
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

// Users is the inbound port: user management. Any active user may approve or disable others.
type Users interface {
	ListUsers(ctx context.Context) ([]entities.User, error)
	// EnableUser approves a pending user or re-enables a disabled one; [errors.ErrUserNotFound] if missing.
	EnableUser(ctx context.Context, id values.UserID) error
	// DisableUser disables the user and ends their sessions;
	// [errors.ErrCannotDisableSelf] if actor is the user, [errors.ErrUserNotFound] if missing.
	DisableUser(ctx context.Context, actor, id values.UserID) error
}
