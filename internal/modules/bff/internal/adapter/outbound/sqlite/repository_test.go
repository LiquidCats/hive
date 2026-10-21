package sqlite_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"testing"
	"time"
	"uuid"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/suite"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
)

type RepositorySuite struct {
	suite.Suite

	conn    *sql.DB
	queries *queries.Queries
	repo    *sqlite.Repository
}

func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositorySuite))
}

func (s *RepositorySuite) SetupTest() {
	ctx := s.T().Context()

	// :memory: is per connection; Connect defaults to a single one.
	conn, err := db.Connect(ctx, db.WithDatabase(":memory:"))
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = conn.Close() })
	s.Require().NoError(db.MigrateUp(ctx, conn, migrations.Migrations))

	s.conn = conn
	s.queries = queries.New(conn)
	s.repo = sqlite.NewRepository(db.NewQueriesTxManager(db.NewTxManager(conn), s.queries), testCipher(s.T()))
}

func (s *RepositorySuite) TestCreateUser_FirstActiveLaterDisabled() {
	first := s.createUser("alice")
	s.False(first.Disabled)
	s.Equal(kernelvalues.Username("alice"), first.Name)

	second := s.createUser("bob")
	s.True(second.Disabled)
}

func (s *RepositorySuite) TestCreateUser_DuplicateName() {
	s.createUser("alice")

	_, err := s.repo.CreateUser(s.T().Context(), entities.User{ID: uuid.NewV7(), Name: "alice"})

	s.ErrorIs(err, domainerrors.ErrUsernameTaken)
}

func (s *RepositorySuite) TestCreateUser_CredentialsRoundTrip() {
	id := uuid.NewV7()
	credential := webauthn.Credential{ID: []byte("cred-1"), PublicKey: []byte("key")}

	_, err := s.repo.CreateUser(s.T().Context(), entities.User{
		ID:          id,
		Name:        "alice",
		Credentials: []webauthn.Credential{credential},
	})
	s.Require().NoError(err)

	got, err := s.repo.GetUserByID(s.T().Context(), id)
	s.Require().NoError(err)
	s.Require().Len(got.Credentials, 1)
	s.Equal(credential.ID, got.Credentials[0].ID)
	s.Equal(credential.PublicKey, got.Credentials[0].PublicKey)
}

func (s *RepositorySuite) TestGetUserByName() {
	want := s.createUser("alice")

	got, err := s.repo.GetUserByName(s.T().Context(), "alice")

	s.Require().NoError(err)
	s.Equal(want.ID, got.ID)
}

func (s *RepositorySuite) TestListUsers() {
	alice := s.createUser("alice")
	bob := s.createUser("bob")

	users, err := s.repo.ListUsers(s.T().Context(), uuid.UUID{}, 10)

	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Equal(alice.ID, users[0].ID)
	s.Equal(bob.ID, users[1].ID)
}

func (s *RepositorySuite) TestMissingUser() {
	ctx := s.T().Context()
	id := uuid.NewV7()

	_, err := s.repo.GetUserByID(ctx, id)
	s.Require().ErrorIs(err, domainerrors.ErrUserNotFound)

	_, err = s.repo.GetUserByName(ctx, "nobody")
	s.Require().ErrorIs(err, domainerrors.ErrUserNotFound)

	s.Require().ErrorIs(s.repo.EnableUserByID(ctx, id), domainerrors.ErrUserNotFound)
	s.Require().ErrorIs(s.repo.DisableUserByID(ctx, id), domainerrors.ErrUserNotFound)
}

func (s *RepositorySuite) TestEnableUserByID() {
	s.createUser("alice")
	bob := s.createUser("bob")

	s.Require().NoError(s.repo.EnableUserByID(s.T().Context(), bob.ID))

	got, err := s.repo.GetUserByID(s.T().Context(), bob.ID)
	s.Require().NoError(err)
	s.False(got.Disabled)
}

func (s *RepositorySuite) TestDisableUserByID_DeletesSessions() {
	ctx := s.T().Context()
	alice := s.createUser("alice")
	s.Require().NoError(s.repo.CreateSession(ctx, entities.Session{
		TokenHash: "hash",
		UserID:    alice.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	}))

	s.Require().NoError(s.repo.DisableUserByID(ctx, alice.ID))

	got, err := s.repo.GetUserByID(ctx, alice.ID)
	s.Require().NoError(err)
	s.True(got.Disabled)

	_, _, err = s.repo.GetSessionUser(ctx, "hash")
	s.ErrorIs(err, domainerrors.ErrUnauthenticated)
}

func (s *RepositorySuite) TestUpdateCredential() {
	ctx := s.T().Context()
	id := uuid.NewV7()
	credential := webauthn.Credential{ID: []byte("cred-1")}
	_, err := s.repo.CreateUser(ctx, entities.User{ID: id, Name: "alice", Credentials: []webauthn.Credential{credential}})
	s.Require().NoError(err)

	credential.Authenticator.SignCount = 42
	s.Require().NoError(s.repo.UpdateCredential(ctx, credential))

	got, err := s.repo.GetUserByID(ctx, id)
	s.Require().NoError(err)
	s.Require().Len(got.Credentials, 1)
	s.Equal(uint32(42), got.Credentials[0].Authenticator.SignCount)
}

func (s *RepositorySuite) TestRevokeCredentialByID() {
	ctx := s.T().Context()
	id := uuid.NewV7()
	_, err := s.repo.CreateUser(ctx, entities.User{
		ID:          id,
		Name:        "alice",
		Credentials: []webauthn.Credential{{ID: []byte("cred-1")}, {ID: []byte("cred-2")}},
	})
	s.Require().NoError(err)

	rows, err := s.queries.ListActiveCredentialsByUser(ctx, id)
	s.Require().NoError(err)
	s.Require().Len(rows, 2)

	s.Require().NoError(s.repo.RevokeCredentialByID(ctx, rows[0].ID))

	got, err := s.repo.GetUserByID(ctx, id)
	s.Require().NoError(err)
	s.Require().Len(got.Credentials, 1)
	s.Equal(rows[1].CredentialID, b64(got.Credentials[0].ID))
}

func (s *RepositorySuite) TestSession_CreateGetDelete() {
	ctx := s.T().Context()
	alice := s.createUser("alice")
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)

	s.Require().NoError(s.repo.CreateSession(ctx, entities.Session{TokenHash: "hash", UserID: alice.ID, ExpiresAt: expiresAt}))

	user, gotExpiry, err := s.repo.GetSessionUser(ctx, "hash")
	s.Require().NoError(err)
	s.Equal(alice.ID, user.ID)
	s.True(expiresAt.Equal(gotExpiry), "want %s, got %s", expiresAt, gotExpiry)

	s.Require().NoError(s.repo.DeleteSession(ctx, "hash"))
	_, _, err = s.repo.GetSessionUser(ctx, "hash")
	s.Require().ErrorIs(err, domainerrors.ErrUnauthenticated)

	// Deleting a missing session is not an error.
	s.NoError(s.repo.DeleteSession(ctx, "hash"))
}

func (s *RepositorySuite) TestCeremony_TakeOnce() {
	ctx := s.T().Context()
	userID := uuid.NewV7()
	want := entities.Ceremony{
		ID:       uuid.NewV7(),
		Type:     values.CeremonyTypeRegistration,
		Username: "alice",
		Session:  webauthn.SessionData{Challenge: "challenge", UserID: userID[:]},
	}
	s.Require().NoError(s.repo.CreateCeremony(ctx, want))

	got, err := s.repo.TakeCeremony(ctx, "challenge")
	s.Require().NoError(err)
	s.Equal(want.ID, got.ID)
	s.Equal(want.Type, got.Type)
	s.Equal(want.Username, got.Username)
	s.Equal(want.Session.Challenge, got.Session.Challenge)
	s.Equal(want.Session.UserID, got.Session.UserID)

	_, err = s.repo.TakeCeremony(ctx, "challenge")
	s.ErrorIs(err, domainerrors.ErrCeremonyNotFound)
}

func (s *RepositorySuite) TestClosedConnection() {
	s.Require().NoError(s.conn.Close())

	id := uuid.NewV7()
	calls := map[string]func(ctx context.Context) error{
		"CreateUser": func(ctx context.Context) error {
			_, err := s.repo.CreateUser(ctx, entities.User{ID: id, Name: "alice"})
			return err
		},
		"GetUserByID": func(ctx context.Context) error {
			_, err := s.repo.GetUserByID(ctx, id)
			return err
		},
		"GetUserByName": func(ctx context.Context) error {
			_, err := s.repo.GetUserByName(ctx, "alice")
			return err
		},
		"ListUsers": func(ctx context.Context) error {
			_, err := s.repo.ListUsers(ctx, uuid.UUID{}, 10)
			return err
		},
		"EnableUserByID":  func(ctx context.Context) error { return s.repo.EnableUserByID(ctx, id) },
		"DisableUserByID": func(ctx context.Context) error { return s.repo.DisableUserByID(ctx, id) },
		"UpdateCredential": func(ctx context.Context) error {
			return s.repo.UpdateCredential(ctx, webauthn.Credential{ID: []byte("cred")})
		},
		"RevokeCredentialByID": func(ctx context.Context) error { return s.repo.RevokeCredentialByID(ctx, id) },
		"CreateSession": func(ctx context.Context) error {
			return s.repo.CreateSession(ctx, entities.Session{TokenHash: "hash", UserID: id})
		},
		"GetSessionUser": func(ctx context.Context) error {
			_, _, err := s.repo.GetSessionUser(ctx, "hash")
			return err
		},
		"DeleteSession": func(ctx context.Context) error { return s.repo.DeleteSession(ctx, "hash") },
		"CreateCeremony": func(ctx context.Context) error {
			return s.repo.CreateCeremony(ctx, entities.Ceremony{ID: id, Session: webauthn.SessionData{Challenge: "c"}})
		},
		"TakeCeremony": func(ctx context.Context) error {
			_, err := s.repo.TakeCeremony(ctx, "c")
			return err
		},
	}

	domain := []error{
		domainerrors.ErrUsernameTaken,
		domainerrors.ErrUserNotFound,
		domainerrors.ErrUnauthenticated,
		domainerrors.ErrCeremonyNotFound,
	}

	for name, call := range calls {
		s.Run(name, func() {
			err := call(s.T().Context())

			s.Require().Error(err)
			for _, target := range domain {
				s.NotErrorIs(err, target)
			}
		})
	}
}

func (s *RepositorySuite) createUser(name kernelvalues.Username) *entities.User {
	user, err := s.repo.CreateUser(s.T().Context(), entities.User{ID: uuid.NewV7(), Name: name})
	s.Require().NoError(err)

	return user
}

func b64(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}
