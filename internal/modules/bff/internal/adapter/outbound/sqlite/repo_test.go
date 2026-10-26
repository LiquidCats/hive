package sqlite_test

import (
	"bytes"
	"crypto/cipher"
	"database/sql"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
)

func TestRepo_CRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo := newRepo(t)
	want := entities.Repo{
		ID:            uuid.NewV7(),
		Name:          "apps",
		URL:           "git@github.com:org/apps.git",
		SSHPrivateKey: "key",
		Insecure:      true,
	}

	created, err := repo.CreateRepo(ctx, want)
	require.NoError(t, err)
	assert.False(t, created.CreatedAt.IsZero())
	assertSameRepo(t, want, *created)

	got, err := repo.GetRepoByID(ctx, want.ID)
	require.NoError(t, err)
	assertSameRepo(t, want, *got)

	want.URL, want.Username, want.Password, want.SSHPrivateKey = "https://h/apps", "u", "token", ""
	updated, err := repo.UpdateRepo(ctx, want)
	require.NoError(t, err)
	assertSameRepo(t, want, *updated)

	require.NoError(t, repo.DeleteRepoByID(ctx, want.ID))
	_, err = repo.GetRepoByID(ctx, want.ID)
	require.ErrorIs(t, err, domainerrors.ErrRepoNotFound)
}

func TestRepo_SecretsSealed(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo, conn := newRepoConn(t)
	secrets := entities.Repo{ID: uuid.NewV7(), URL: "https://h/a", Username: "u", Password: "hunter2-password"}
	key := entities.Repo{ID: uuid.NewV7(), URL: "git@h:b", SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----"}
	empty := entities.Repo{ID: uuid.NewV7(), URL: "https://h/c"}

	for _, r := range []entities.Repo{secrets, key, empty} {
		_, err := repo.CreateRepo(ctx, r)
		require.NoError(t, err)
	}

	raw := func(id kernelvalues.RepoID) (string, string) {
		var password, sshKey string
		require.NoError(t, conn.QueryRowContext(ctx,
			"SELECT password, ssh_private_key FROM repositories WHERE id = ?", id).Scan(&password, &sshKey))

		return password, sshKey
	}

	password, _ := raw(secrets.ID)
	assert.NotEmpty(t, password)
	assert.NotContains(t, password, "hunter2")

	_, sshKey := raw(key.ID)
	assert.NotEmpty(t, sshKey)
	assert.NotContains(t, sshKey, "OPENSSH")

	password, sshKey = raw(empty.ID)
	assert.Empty(t, password, "unset secrets stay empty")
	assert.Empty(t, sshKey)

	// Ciphertext is bound to its row: moved to another one it no longer opens.
	sealed, _ := raw(secrets.ID)
	_, err := conn.ExecContext(ctx, "UPDATE repositories SET password = ? WHERE id = ?", sealed, empty.ID)
	require.NoError(t, err)

	_, err = repo.GetRepoByID(ctx, empty.ID)
	require.Error(t, err)

	// ...and to its column.
	_, err = conn.ExecContext(ctx, "UPDATE repositories SET password = '', ssh_private_key = ? WHERE id = ?",
		sealed, secrets.ID)
	require.NoError(t, err)

	_, err = repo.GetRepoByID(ctx, secrets.ID)
	require.Error(t, err)

	_, err = repo.ListRepos(ctx, uuid.UUID{}, 10)
	require.Error(t, err)
}

func TestRepo_URLTaken(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo := newRepo(t)
	first := entities.Repo{ID: uuid.NewV7(), URL: "https://h/a"}
	second := entities.Repo{ID: uuid.NewV7(), URL: "https://h/b"}

	for _, r := range []entities.Repo{first, second} {
		_, err := repo.CreateRepo(ctx, r)
		require.NoError(t, err)
	}

	_, err := repo.CreateRepo(ctx, entities.Repo{ID: uuid.NewV7(), URL: first.URL})
	require.ErrorIs(t, err, domainerrors.ErrRepoURLTaken)

	second.URL = first.URL
	_, err = repo.UpdateRepo(ctx, second)
	require.ErrorIs(t, err, domainerrors.ErrRepoURLTaken)
}

func TestRepo_NotFound(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo := newRepo(t)
	id := uuid.NewV7()

	_, err := repo.GetRepoByID(ctx, id)
	require.ErrorIs(t, err, domainerrors.ErrRepoNotFound)

	_, err = repo.UpdateRepo(ctx, entities.Repo{ID: id, URL: "https://h/r"})
	require.ErrorIs(t, err, domainerrors.ErrRepoNotFound)

	require.ErrorIs(t, repo.DeleteRepoByID(ctx, id), domainerrors.ErrRepoNotFound)
}

func TestListReposBefore(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)

	var ids []kernelvalues.RepoID
	for _, url := range []string{"https://h/e", "https://h/d", "https://h/c"} {
		created, err := repo.CreateRepo(t.Context(), entities.Repo{ID: uuid.NewV7(), URL: url})
		require.NoError(t, err)

		ids = append(ids, created.ID)
	}

	tests := []struct {
		name   string
		before kernelvalues.RepoID
		want   []kernelvalues.RepoID
	}{
		{name: "nearest rows, oldest first", before: ids[2], want: ids[:2]},
		{name: "short page", before: ids[1], want: ids[:1]},
		{name: "nothing before the first", before: ids[0], want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // read only

			repos, err := repo.ListReposBefore(t.Context(), tt.before, 2)
			require.NoError(t, err)

			var got []kernelvalues.RepoID
			for _, r := range repos {
				got = append(got, r.ID)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListRepos_Keyset(t *testing.T) {
	t.Parallel()

	repo := newRepo(t)

	var ids []kernelvalues.RepoID
	for _, url := range []string{"https://h/e", "https://h/d", "https://h/c"} {
		created, err := repo.CreateRepo(t.Context(), entities.Repo{ID: uuid.NewV7(), URL: url})
		require.NoError(t, err)

		ids = append(ids, created.ID)
	}

	tests := []struct {
		name  string
		after kernelvalues.RepoID
		want  []kernelvalues.RepoID
	}{
		{name: "first page", after: uuid.UUID{}, want: ids[:2]},
		{name: "last page is short", after: ids[1], want: ids[2:]},
		{name: "past the end", after: ids[2], want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // read only

			repos, err := repo.ListRepos(t.Context(), tt.after, 2)
			require.NoError(t, err)

			var got []kernelvalues.RepoID
			for _, r := range repos {
				got = append(got, r.ID)
			}

			// Creation order, not URL order.
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewCipher(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, 16, 31, 33} {
		_, err := sqlite.NewCipher(make([]byte, size))
		require.Error(t, err, size)
	}

	_, err := sqlite.NewCipher(make([]byte, 32))
	require.NoError(t, err)
}

func TestRepo_ClosedConnection(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	repo, conn := newRepoConn(t)
	require.NoError(t, conn.Close())

	id := uuid.NewV7()

	_, err := repo.CreateRepo(ctx, entities.Repo{ID: id, URL: "https://h/r"})
	require.Error(t, err)
	require.NotErrorIs(t, err, domainerrors.ErrRepoURLTaken)

	_, err = repo.GetRepoByID(ctx, id)
	require.Error(t, err)
	require.NotErrorIs(t, err, domainerrors.ErrRepoNotFound)

	_, err = repo.ListRepos(ctx, uuid.UUID{}, 10)
	require.Error(t, err)

	_, err = repo.UpdateRepo(ctx, entities.Repo{ID: id, URL: "https://h/r"})
	require.Error(t, err)
	require.NotErrorIs(t, err, domainerrors.ErrRepoNotFound)

	err = repo.DeleteRepoByID(ctx, id)
	require.Error(t, err)
	require.NotErrorIs(t, err, domainerrors.ErrRepoNotFound)
}

func assertSameRepo(t *testing.T, want, got entities.Repo) {
	t.Helper()

	got.CreatedAt, got.UpdatedAt = want.CreatedAt, want.UpdatedAt
	assert.Equal(t, want, got)
}

func newRepoConn(t *testing.T) (*sqlite.Repository, *sql.DB) {
	t.Helper()

	// :memory: is per connection; Connect defaults to a single one.
	conn, err := db.Connect(t.Context(), db.WithDatabase(":memory:"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, db.MigrateUp(t.Context(), conn, migrations.Migrations))

	return sqlite.NewRepository(db.NewQueriesTxManager(db.NewTxManager(conn), queries.New(conn)), testCipher(t)), conn
}

// testCipher uses a fixed key; tests never depend on its value.
func testCipher(t *testing.T) cipher.AEAD {
	t.Helper()

	aead, err := sqlite.NewCipher(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, err)

	return aead
}
