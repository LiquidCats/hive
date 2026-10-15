package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
	"github.com/LiquidCats/hive/tests/mocks/kernelmocks"
)

const sessionTTL = time.Hour

var errBoom = errors.New("boom")

// tokenHash is what the session repository must receive for the raw token "token".
var tokenHash = func() string {
	sum := sha256.Sum256([]byte("token"))

	return hex.EncodeToString(sum[:])
}()

type AuthSuite struct {
	suite.Suite

	users       *mocks.MockUserRepository
	credentials *mocks.MockCredentialRepository
	ceremonies  *mocks.MockCeremonyRepository
	sessions    *mocks.MockSessionRepository
	clock       *kernelmocks.MockClock

	now  time.Time
	auth *app.Auth
}

func TestAuthSuite(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupTest() {
	s.users = mocks.NewMockUserRepository(s.T())
	s.credentials = mocks.NewMockCredentialRepository(s.T())
	s.ceremonies = mocks.NewMockCeremonyRepository(s.T())
	s.sessions = mocks.NewMockSessionRepository(s.T())
	s.clock = kernelmocks.NewMockClock(s.T())
	s.now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	wn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Hive",
		RPID:          "hive.test",
		RPOrigins:     []string{"https://hive.test"},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true},
			Registration: webauthn.TimeoutConfig{Enforce: true},
		},
	})
	s.Require().NoError(err)

	s.auth = app.NewAuth(wn, s.users, s.credentials, s.ceremonies, s.sessions, s.clock, sessionTTL)
}

func (s *AuthSuite) TestBeginRegistration_InvalidUsername() {
	_, err := s.auth.BeginRegistration(s.T().Context(), "a b")

	s.ErrorIs(err, domainerrors.ErrInvalidUsername)
}

func (s *AuthSuite) TestBeginRegistration_UsernameTaken() {
	s.users.EXPECT().GetUserByName(mock.Anything, kernelvalues.Username("alice")).
		Return(&entities.User{}, nil).Once()

	_, err := s.auth.BeginRegistration(s.T().Context(), " Alice ")

	s.ErrorIs(err, domainerrors.ErrUsernameTaken)
}

func (s *AuthSuite) TestBeginRegistration_RepositoryError() {
	s.users.EXPECT().GetUserByName(mock.Anything, kernelvalues.Username("alice")).Return(nil, errBoom).Once()

	_, err := s.auth.BeginRegistration(s.T().Context(), "alice")

	s.Require().ErrorIs(err, errBoom)
	s.Require().NotErrorIs(err, domainerrors.ErrUsernameTaken)
}

func (s *AuthSuite) TestBeginRegistration_CreateCeremonyError() {
	s.users.EXPECT().GetUserByName(mock.Anything, kernelvalues.Username("alice")).
		Return(nil, domainerrors.ErrUserNotFound).Once()
	s.ceremonies.EXPECT().CreateCeremony(mock.Anything, mock.Anything).Return(errBoom).Once()

	_, err := s.auth.BeginRegistration(s.T().Context(), "alice")

	s.ErrorIs(err, errBoom)
}

func (s *AuthSuite) TestBeginRegistration_Success() {
	s.users.EXPECT().GetUserByName(mock.Anything, kernelvalues.Username("alice")).
		Return(nil, domainerrors.ErrUserNotFound).Once()

	var stored entities.Ceremony
	s.ceremonies.EXPECT().CreateCeremony(mock.Anything, mock.MatchedBy(func(c entities.Ceremony) bool {
		return c.Type == values.CeremonyTypeRegistration && c.Username == "alice"
	})).RunAndReturn(func(_ context.Context, c entities.Ceremony) error {
		stored = c

		return nil
	}).Once()

	creation, err := s.auth.BeginRegistration(s.T().Context(), " ALICE ")

	s.Require().NoError(err)
	s.Equal("alice", creation.Response.User.Name)
	s.Equal(creation.Response.Challenge.String(), stored.Session.Challenge)
	s.Len(stored.Session.UserID, len(kernelvalues.UserID{}))
}

func (s *AuthSuite) TestBeginLogin_CreateCeremonyError() {
	s.ceremonies.EXPECT().CreateCeremony(mock.Anything, mock.Anything).Return(errBoom).Once()

	_, err := s.auth.BeginLogin(s.T().Context())

	s.ErrorIs(err, errBoom)
}

func (s *AuthSuite) TestBeginLogin_Success() {
	s.ceremonies.EXPECT().CreateCeremony(mock.Anything, mock.MatchedBy(func(c entities.Ceremony) bool {
		return c.Type == values.CeremonyTypeLogin && c.Username == "" && c.Session.Challenge != ""
	})).Return(nil).Once()

	assertion, err := s.auth.BeginLogin(s.T().Context())

	s.Require().NoError(err)
	s.NotEmpty(assertion.Response.Challenge)
}

func (s *AuthSuite) TestFinishRegistration_TakeCeremonyError() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").Return(nil, domainerrors.ErrCeremonyNotFound).Once()

	_, err := s.auth.FinishRegistration(s.T().Context(), creationData())

	s.ErrorIs(err, domainerrors.ErrCeremonyNotFound)
}

func (s *AuthSuite) TestFinishRegistration_WrongCeremonyType() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").
		Return(&entities.Ceremony{Type: values.CeremonyTypeLogin}, nil).Once()

	_, err := s.auth.FinishRegistration(s.T().Context(), creationData())

	s.ErrorIs(err, domainerrors.ErrCeremonyNotFound)
}

func (s *AuthSuite) TestFinishRegistration_SessionUserIDNotUUID() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").Return(&entities.Ceremony{
		Type:    values.CeremonyTypeRegistration,
		Session: webauthn.SessionData{Challenge: "challenge", UserID: []byte("short")},
	}, nil).Once()

	_, err := s.auth.FinishRegistration(s.T().Context(), creationData())

	s.Require().Error(err)
	s.NotErrorIs(err, domainerrors.ErrVerification)
}

func (s *AuthSuite) TestFinishRegistration_VerificationFails() {
	id := uuid.NewV7()
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").Return(&entities.Ceremony{
		Type:     values.CeremonyTypeRegistration,
		Username: "alice",
		Session:  webauthn.SessionData{Challenge: "other", UserID: id[:]},
	}, nil).Once()

	_, err := s.auth.FinishRegistration(s.T().Context(), creationData())

	s.ErrorIs(err, domainerrors.ErrVerification)
}

func (s *AuthSuite) TestFinishLogin_TakeCeremonyError() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").Return(nil, errBoom).Once()

	_, err := s.auth.FinishLogin(s.T().Context(), assertionData())

	s.ErrorIs(err, errBoom)
}

func (s *AuthSuite) TestFinishLogin_WrongCeremonyType() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").
		Return(&entities.Ceremony{Type: values.CeremonyTypeRegistration}, nil).Once()

	_, err := s.auth.FinishLogin(s.T().Context(), assertionData())

	s.ErrorIs(err, domainerrors.ErrCeremonyNotFound)
}

func (s *AuthSuite) TestFinishLogin_VerificationFails() {
	s.ceremonies.EXPECT().TakeCeremony(mock.Anything, "challenge").Return(&entities.Ceremony{
		Type:    values.CeremonyTypeLogin,
		Session: webauthn.SessionData{Challenge: "other"},
	}, nil).Once()

	_, err := s.auth.FinishLogin(s.T().Context(), assertionData())

	s.ErrorIs(err, domainerrors.ErrVerification)
}

func (s *AuthSuite) TestSession_RepositoryError() {
	s.sessions.EXPECT().GetSessionUser(mock.Anything, tokenHash).
		Return(nil, time.Time{}, domainerrors.ErrUnauthenticated).Once()

	_, err := s.auth.Session(s.T().Context(), "token")

	s.ErrorIs(err, domainerrors.ErrUnauthenticated)
}

func (s *AuthSuite) TestSession_DisabledUser() {
	s.sessions.EXPECT().GetSessionUser(mock.Anything, tokenHash).
		Return(&entities.User{Disabled: true}, s.now.Add(time.Minute), nil).Once()

	_, err := s.auth.Session(s.T().Context(), "token")

	s.ErrorIs(err, domainerrors.ErrUnauthenticated)
}

func (s *AuthSuite) TestSession_Expired() {
	s.sessions.EXPECT().GetSessionUser(mock.Anything, tokenHash).
		Return(&entities.User{}, s.now, nil).Once()
	s.clock.EXPECT().Now().Return(s.now).Once()

	_, err := s.auth.Session(s.T().Context(), "token")

	s.ErrorIs(err, domainerrors.ErrUnauthenticated)
}

func (s *AuthSuite) TestSession_Valid() {
	want := &entities.User{ID: uuid.NewV7(), Name: "alice"}
	s.sessions.EXPECT().GetSessionUser(mock.Anything, tokenHash).
		Return(want, s.now.Add(time.Second), nil).Once()
	s.clock.EXPECT().Now().Return(s.now).Once()

	got, err := s.auth.Session(s.T().Context(), "token")

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *AuthSuite) TestLogout_HashesToken() {
	s.sessions.EXPECT().DeleteSession(mock.Anything, tokenHash).Return(nil).Once()

	s.NoError(s.auth.Logout(s.T().Context(), "token"))
}

func (s *AuthSuite) TestLogout_RepositoryError() {
	s.sessions.EXPECT().DeleteSession(mock.Anything, tokenHash).Return(errBoom).Once()

	s.ErrorIs(s.auth.Logout(s.T().Context(), "token"), errBoom)
}

func creationData() *protocol.ParsedCredentialCreationData {
	var data protocol.ParsedCredentialCreationData
	data.Response.CollectedClientData.Challenge = "challenge"

	return &data
}

func assertionData() *protocol.ParsedCredentialAssertionData {
	var data protocol.ParsedCredentialAssertionData
	data.Response.CollectedClientData.Challenge = "challenge"

	return &data
}
