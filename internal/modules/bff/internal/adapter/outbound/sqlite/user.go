package sqlite

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"

	"uuid"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rotisserie/eris"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	domainvalues "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

var _ ports.UserRepository = (*Repository)(nil)

// CreateUser implements [ports.UserRepository].
func (r *Repository) CreateUser(ctx context.Context, user entities.User) (*entities.User, error) {
	var stored entities.User

	err := r.queryGetter.Transactional(ctx, func(ctx context.Context) error {
		queries := r.queryGetter.GetQueries(ctx)

		row, err := queries.CreateUser(ctx, db.CreateUserParams{
			ID:   user.ID,
			Name: user.Name.String(),
		})
		if err != nil {
			// The name check before the ceremony can race a concurrent registration; the UNIQUE index cannot.
			var sqliteErr *sqlite.Error
			if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return domainerrors.ErrUsernameTaken
			}

			return eris.Wrap(err, "create user")
		}

		stored = toUser(row)

		for _, credential := range user.Credentials {
			val, marshalErr := json.Marshal(credential)
			if marshalErr != nil {
				return eris.Wrap(marshalErr, "marshal credential value")
			}

			err = queries.CreateCredential(ctx, db.CreateCredentialParams{
				ID:           uuid.NewV7(),
				UserID:       user.ID,
				CredentialID: credentialKey(credential),
				Credential:   string(val),
			})
			if err != nil {
				return eris.Wrap(err, "create credential")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &stored, nil
}

// DisableUserByID implements [ports.UserRepository].
func (r *Repository) DisableUserByID(ctx context.Context, id values.UserID) error {
	return r.queryGetter.Transactional(ctx, func(ctx context.Context) error {
		queries := r.queryGetter.GetQueries(ctx)

		n, err := queries.DisableUserByID(ctx, id)
		if err != nil {
			return eris.Wrap(err, "disable user by id")
		}

		if n == 0 {
			return domainerrors.ErrUserNotFound
		}

		return eris.Wrap(queries.DeleteSessionsByUser(ctx, id), "delete sessions by user")
	})
}

// EnableUserByID implements [ports.UserRepository].
func (r *Repository) EnableUserByID(ctx context.Context, id values.UserID) error {
	n, err := r.queryGetter.
		GetQueries(ctx).
		EnableUserByID(ctx, id)
	if err != nil {
		return eris.Wrap(err, "enable user by id")
	}

	if n == 0 {
		return domainerrors.ErrUserNotFound
	}

	return nil
}

// SetUserRoleByID implements [ports.UserRepository].
func (r *Repository) SetUserRoleByID(ctx context.Context, id values.UserID, role domainvalues.Role) error {
	queries := r.queryGetter.GetQueries(ctx)

	n, err := queries.SetUserRoleByID(ctx, db.SetUserRoleByIDParams{Role: role.String(), ID: id})
	if err != nil {
		return eris.Wrap(err, "set user role by id")
	}

	if n > 0 {
		return nil
	}

	// The update itself was atomic; this only picks the error.
	_, err = queries.GetUserByID(ctx, id)
	if eris.Is(err, sql.ErrNoRows) {
		return domainerrors.ErrUserNotFound
	}

	if err != nil {
		return eris.Wrap(err, "get user by id")
	}

	return domainerrors.ErrUserDisabled
}

// ListUsers implements [ports.UserRepository].
func (r *Repository) ListUsers(ctx context.Context, after values.UserID, limit int) ([]entities.User, error) {
	rows, err := r.queryGetter.
		GetQueries(ctx).
		ListUsers(ctx, db.ListUsersParams{ID: after, Limit: int64(limit)})
	if err != nil {
		return nil, eris.Wrap(err, "list users")
	}

	users := make([]entities.User, len(rows))
	for i, row := range rows {
		users[i] = toUser(row)
	}

	return users, nil
}

// GetUserByID implements [ports.UserRepository].
func (r *Repository) GetUserByID(ctx context.Context, id values.UserID) (*entities.User, error) {
	queries := r.queryGetter.GetQueries(ctx)

	user, err := queries.GetUserByID(ctx, id)
	if err != nil {
		if eris.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.ErrUserNotFound
		}

		return nil, eris.Wrap(err, "get user by id")
	}

	rows, err := queries.ListActiveCredentialsByUser(ctx, id)
	if err != nil {
		return nil, eris.Wrap(err, "list credentials by user")
	}

	credentials := make([]webauthn.Credential, len(rows))
	for i, row := range rows {
		err = json.Unmarshal([]byte(row.Credential), &credentials[i])
		if err != nil {
			return nil, eris.Wrap(err, "unmarshal credential value")
		}
	}

	result := toUser(user)
	result.Credentials = credentials

	return &result, nil
}

// GetUserByName implements [ports.UserRepository].
func (r *Repository) GetUserByName(ctx context.Context, name values.Username) (*entities.User, error) {
	user, err := r.queryGetter.
		GetQueries(ctx).
		GetUserByName(ctx, name.String())
	if err != nil {
		if eris.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.ErrUserNotFound
		}

		return nil, eris.Wrap(err, "get user by name")
	}

	result := toUser(user)

	return &result, nil
}

func toUser(row db.User) entities.User {
	user := entities.User{
		ID:        row.ID,
		Name:      values.Username(row.Name),
		Disabled:  row.DisabledAt.Valid,
		CreatedAt: row.CreatedAt,
	}

	// The CHECK constraint keeps it valid; an unknown role would stay RoleUser, the least privileged.
	_ = user.Role.UnmarshalText([]byte(row.Role))

	return user
}
