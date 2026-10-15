package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
	"uuid"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rotisserie/eris"

	kernelports "github.com/LiquidCats/hive/internal/kernel/ports"
	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

var _ ports.Auth = (*Auth)(nil)

type Auth struct {
	authn *webauthn.WebAuthn

	userRepo       ports.UserRepository
	credentialRepo ports.CredentialRepository
	ceremonyRepo   ports.CeremonyRepository
	sessionRepo    ports.SessionRepository

	clock      kernelports.Clock
	sessionTTL time.Duration
}

func NewAuth(
	authn *webauthn.WebAuthn,
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	ceremonyRepo ports.CeremonyRepository,
	sessionRepo ports.SessionRepository,
	clock kernelports.Clock,
	sessionTTL time.Duration,
) *Auth {
	return &Auth{
		authn: authn,

		userRepo:       userRepo,
		credentialRepo: credentialRepo,
		ceremonyRepo:   ceremonyRepo,
		sessionRepo:    sessionRepo,

		clock:      clock,
		sessionTTL: sessionTTL,
	}
}

// BeginRegistration implements [ports.Auth].
func (a *Auth) BeginRegistration(
	ctx context.Context,
	username kernelvalues.Username,
) (*protocol.CredentialCreation, error) {
	if !username.IsValid() {
		return nil, domainerrors.ErrInvalidUsername
	}

	username = username.Normalized()

	switch _, err := a.userRepo.GetUserByName(ctx, username); {
	case err == nil:
		return nil, domainerrors.ErrUsernameTaken
	case !eris.Is(err, domainerrors.ErrUserNotFound):
		return nil, eris.Wrap(err, "get user by name")
	}

	user := entities.User{
		ID:   uuid.NewV7(),
		Name: username,
	}

	// The user is only persisted in FinishRegistration; its ID travels in the session.
	// A discoverable credential lets login skip the username.
	// UV required: go-webauthn only checks the UV flag when the session demands it.
	creation, session, err := a.authn.BeginRegistration(
		user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return nil, eris.Wrap(err, "begin registration")
	}

	err = a.ceremonyRepo.CreateCeremony(ctx, entities.Ceremony{
		ID:       uuid.NewV7(),
		Type:     values.CeremonyTypeRegistration,
		Username: username,
		Session:  *session,
	})
	if err != nil {
		return nil, eris.Wrap(err, "save ceremony")
	}

	return creation, nil
}

// FinishRegistration implements [ports.Auth].
func (a *Auth) FinishRegistration(ctx context.Context, resp *protocol.ParsedCredentialCreationData) (string, error) {
	ceremony, err := a.takeCeremony(ctx, resp.Response.CollectedClientData.Challenge, values.CeremonyTypeRegistration)
	if err != nil {
		return "", err
	}

	if len(ceremony.Session.UserID) != len(kernelvalues.UserID{}) {
		return "", eris.New("session user id is not a uuid")
	}

	user := entities.User{
		ID:   kernelvalues.UserID(ceremony.Session.UserID),
		Name: ceremony.Username,
	}

	credential, err := a.authn.CreateCredential(user, ceremony.Session, resp)
	if err != nil {
		return "", eris.Wrap(domainerrors.ErrVerification, err.Error())
	}

	user.Credentials = []webauthn.Credential{*credential}

	stored, err := a.userRepo.CreateUser(ctx, user)
	if err != nil {
		return "", eris.Wrap(err, "create user")
	}

	if stored.Disabled {
		return "", domainerrors.ErrApprovalPending
	}

	return a.newSession(ctx, stored.ID)
}

// BeginLogin implements [ports.Auth].
func (a *Auth) BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, error) {
	assertion, session, err := a.authn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, eris.Wrap(err, "begin login")
	}

	err = a.ceremonyRepo.CreateCeremony(ctx, entities.Ceremony{
		ID:      uuid.NewV7(),
		Type:    values.CeremonyTypeLogin,
		Session: *session,
	})
	if err != nil {
		return nil, eris.Wrap(err, "save ceremony")
	}

	return assertion, nil
}

// FinishLogin implements [ports.Auth].
func (a *Auth) FinishLogin(ctx context.Context, resp *protocol.ParsedCredentialAssertionData) (string, error) {
	ceremony, err := a.takeCeremony(ctx, resp.Response.CollectedClientData.Challenge, values.CeremonyTypeLogin)
	if err != nil {
		return "", err
	}

	var user *entities.User

	// The authenticator names its user via userHandle, which is the user ID set at registration.
	// Disabled users are returned too: their state is only revealed after the signature checks out.
	findUser := func(_, userHandle []byte) (webauthn.User, error) {
		if len(userHandle) != len(kernelvalues.UserID{}) {
			return nil, domainerrors.ErrUserNotFound
		}

		var findErr error

		user, findErr = a.userRepo.GetUserByID(ctx, kernelvalues.UserID(userHandle))
		if findErr != nil {
			return nil, findErr
		}

		return user, nil
	}

	_, credential, err := a.authn.ValidatePasskeyLogin(findUser, ceremony.Session, resp)
	if err != nil {
		return "", eris.Wrap(domainerrors.ErrVerification, err.Error())
	}

	if credential.Authenticator.CloneWarning {
		return "", eris.Wrap(domainerrors.ErrVerification, "sign counter went backwards, authenticator may be cloned")
	}

	// Persist the new sign counter / flags so cloned authenticators are detectable.
	if err = a.credentialRepo.UpdateCredential(ctx, *credential); err != nil {
		return "", eris.Wrap(err, "update credential")
	}

	if user.Disabled {
		return "", domainerrors.ErrApprovalPending
	}

	return a.newSession(ctx, user.ID)
}

// Session implements [ports.Auth].
func (a *Auth) Session(ctx context.Context, token string) (*entities.User, error) {
	user, expiresAt, err := a.sessionRepo.GetSessionUser(ctx, hashToken(token))
	if err != nil {
		return nil, eris.Wrap(err, "get session user")
	}

	// Disabling deletes sessions too; this also covers one created concurrently.
	if user.Disabled || !a.clock.Now().Before(expiresAt) {
		return nil, domainerrors.ErrUnauthenticated
	}

	return user, nil
}

// Logout implements [ports.Auth].
func (a *Auth) Logout(ctx context.Context, token string) error {
	return eris.Wrap(a.sessionRepo.DeleteSession(ctx, hashToken(token)), "delete session")
}

// takeCeremony consumes the ceremony for the challenge the client echoed back; a challenge can only be used once.
func (a *Auth) takeCeremony(
	ctx context.Context,
	challenge string,
	ceremonyType values.CeremonyType,
) (*entities.Ceremony, error) {
	ceremony, err := a.ceremonyRepo.TakeCeremony(ctx, challenge)
	if err != nil {
		return nil, eris.Wrap(err, "take ceremony")
	}

	if ceremony.Type != ceremonyType {
		return nil, domainerrors.ErrCeremonyNotFound
	}

	return ceremony, nil
}

// newSession returns a random token; only its hash is stored, so a database leak yields no usable cookies.
func (a *Auth) newSession(ctx context.Context, userID kernelvalues.UserID) (string, error) {
	token := rand.Text()

	err := a.sessionRepo.CreateSession(ctx, entities.Session{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: a.clock.Now().Add(a.sessionTTL),
	})
	if err != nil {
		return "", eris.Wrap(err, "create session")
	}

	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
