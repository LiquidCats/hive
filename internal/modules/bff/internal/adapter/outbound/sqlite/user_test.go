package sqlite_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
)

func TestCreateUser_Roles(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)
	owner := mustCreateUser(t, repo, "alice")
	later := mustCreateUser(t, repo, "bob")

	assert.Equal(t, values.RoleOwner, owner.Role)
	assert.Equal(t, values.RoleUser, later.Role)

	// Every read path maps the role.
	got, err := repo.GetUserByID(t.Context(), owner.ID)
	require.NoError(t, err)
	assert.Equal(t, values.RoleOwner, got.Role)

	got, err = repo.GetUserByName(t.Context(), "bob")
	require.NoError(t, err)
	assert.Equal(t, values.RoleUser, got.Role)

	users, err := repo.ListUsers(t.Context(), uuid.UUID{}, 10)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, values.RoleOwner, users[0].Role)
	assert.Equal(t, values.RoleUser, users[1].Role)
}

func TestListUsers_Keyset(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)

	var ids []kernelvalues.UserID
	for _, name := range []kernelvalues.Username{"erin", "dave", "carol", "bob", "alice"} {
		ids = append(ids, mustCreateUser(t, repo, name).ID)
	}

	tests := []struct {
		name  string
		after kernelvalues.UserID
		limit int
		want  []kernelvalues.UserID
	}{
		{name: "first page", after: uuid.UUID{}, limit: 2, want: ids[:2]},
		{name: "middle page", after: ids[1], limit: 2, want: ids[2:4]},
		{name: "last page is short", after: ids[3], limit: 2, want: ids[4:]},
		{name: "past the end", after: ids[4], limit: 2, want: nil},
		{name: "unknown cursor still pages by id", after: uuid.NewV7(), limit: 2, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // read only

			users, err := repo.ListUsers(t.Context(), tt.after, tt.limit)
			require.NoError(t, err)

			var got []kernelvalues.UserID
			for _, user := range users {
				got = append(got, user.ID)
			}

			// Registration order, not name order.
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetUserRoleByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		role    values.Role
		missing bool
		pending bool
		wantErr error
		// constraint: the database must reject it (UNIQUE single owner, CHECK on role).
		constraint string
	}{
		{name: "admin", role: values.RoleAdmin},
		{name: "back to user", role: values.RoleUser},
		{name: "missing user", role: values.RoleAdmin, missing: true, wantErr: domainerrors.ErrUserNotFound},
		{name: "pending user", role: values.RoleAdmin, pending: true, wantErr: domainerrors.ErrUserDisabled},
		{name: "second owner", role: values.RoleOwner, constraint: "UNIQUE constraint failed"},
		{name: "unknown role", role: values.Role(99), constraint: "CHECK constraint failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newRepo(t)
			mustCreateUser(t, repo, "alice")
			bob := mustCreateUser(t, repo, "bob")

			if !tt.pending {
				require.NoError(t, repo.EnableUserByID(t.Context(), bob.ID))
			}

			id := bob.ID
			if tt.missing {
				id = uuid.NewV7()
			}

			err := repo.SetUserRoleByID(t.Context(), id, tt.role)

			got, getErr := repo.GetUserByID(t.Context(), bob.ID)
			require.NoError(t, getErr)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.constraint != "":
				require.ErrorContains(t, err, tt.constraint)
				assert.Equal(t, values.RoleUser, got.Role, "role must be unchanged")
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.role, got.Role)
			}
		})
	}
}

func TestMigrationRoles_BackfillsOwner(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	conn, err := db.Connect(ctx, db.WithDatabase(":memory:"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// An install from before roles: only the first migration ran.
	v1 := fstest.MapFS{}
	for _, name := range []string{"000001_init.up.sql", "000001_init.down.sql"} {
		data, readErr := fs.ReadFile(migrations.Migrations, name)
		require.NoError(t, readErr)
		v1[name] = &fstest.MapFile{Data: data}
	}
	require.NoError(t, db.MigrateUp(ctx, conn, v1))

	// The oldest user is disabled, so the oldest active one becomes the owner.
	_, err = conn.ExecContext(ctx, `INSERT INTO users (id, name, created_at, disabled_at) VALUES
		(?, 'zed', '2026-01-01 00:00:00', '2026-01-05 00:00:00'),
		(?, 'alice', '2026-01-02 00:00:00', NULL),
		(?, 'bob', '2026-01-03 00:00:00', NULL)`,
		uuid.NewV7(), uuid.NewV7(), uuid.NewV7())
	require.NoError(t, err)

	require.NoError(t, db.MigrateUp(ctx, conn, migrations.Migrations))

	repo := sqlite.NewRepository(db.NewQueriesTxManager(db.NewTxManager(conn), queries.New(conn)), testCipher(t))
	roles := map[kernelvalues.Username]values.Role{}

	users, err := repo.ListUsers(ctx, uuid.UUID{}, 10)
	require.NoError(t, err)

	for _, user := range users {
		roles[user.Name] = user.Role
	}

	assert.Equal(t, map[kernelvalues.Username]values.Role{
		"zed":   values.RoleUser,
		"alice": values.RoleOwner,
		"bob":   values.RoleUser,
	}, roles)
}

func TestDisableUserByID_Demotes(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo := newRepo(t)
	mustCreateUser(t, repo, "alice")
	bob := mustCreateUser(t, repo, "bob")
	require.NoError(t, repo.EnableUserByID(ctx, bob.ID))
	require.NoError(t, repo.SetUserRoleByID(ctx, bob.ID, values.RoleAdmin))

	require.NoError(t, repo.DisableUserByID(ctx, bob.ID))
	require.NoError(t, repo.EnableUserByID(ctx, bob.ID))

	got, err := repo.GetUserByID(ctx, bob.ID)
	require.NoError(t, err)
	assert.False(t, got.Disabled)
	assert.Equal(t, values.RoleUser, got.Role, "a re-enabled admin comes back as a user")
}

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()

	repo, _ := newRepoConn(t)

	return repo
}

func mustCreateUser(t *testing.T, repo *sqlite.Repository, name kernelvalues.Username) *entities.User {
	t.Helper()

	user, err := repo.CreateUser(t.Context(), entities.User{ID: uuid.NewV7(), Name: name})
	require.NoError(t, err)

	return user
}
