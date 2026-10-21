package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"

	"github.com/rotisserie/eris"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

var _ ports.RepoRepository = (*Repository)(nil)

const (
	passwordColumn      = "password"
	sshPrivateKeyColumn = "ssh_private_key"
)

// CreateRepo implements [ports.RepoRepository].
func (r *Repository) CreateRepo(ctx context.Context, repo entities.Repo) (*entities.Repo, error) {
	row, err := r.queryGetter.GetQueries(ctx).CreateRepo(ctx, db.CreateRepoParams{
		ID:            repo.ID,
		Name:          repo.Name,
		Url:           repo.URL,
		Username:      repo.Username,
		Password:      r.seal(repo.ID, passwordColumn, repo.Password),
		SshPrivateKey: r.seal(repo.ID, sshPrivateKeyColumn, repo.SSHPrivateKey),
		Insecure:      repo.Insecure,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domainerrors.ErrRepoURLTaken
		}

		return nil, eris.Wrap(err, "create repo")
	}

	return r.toRepo(row)
}

// GetRepoByID implements [ports.RepoRepository].
func (r *Repository) GetRepoByID(ctx context.Context, id values.RepoID) (*entities.Repo, error) {
	row, err := r.queryGetter.GetQueries(ctx).GetRepoByID(ctx, id)
	if err != nil {
		if eris.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.ErrRepoNotFound
		}

		return nil, eris.Wrap(err, "get repo by id")
	}

	return r.toRepo(row)
}

// ListRepos implements [ports.RepoRepository].
func (r *Repository) ListRepos(ctx context.Context, after values.RepoID, limit int) ([]entities.Repo, error) {
	rows, err := r.queryGetter.
		GetQueries(ctx).
		ListRepos(ctx, db.ListReposParams{ID: after, Limit: int64(limit)})
	if err != nil {
		return nil, eris.Wrap(err, "list repos")
	}

	repos := make([]entities.Repo, len(rows))
	for i, row := range rows {
		repo, toErr := r.toRepo(row)
		if toErr != nil {
			return nil, toErr
		}

		repos[i] = *repo
	}

	return repos, nil
}

// ListReposBefore implements [ports.RepoRepository].
func (r *Repository) ListReposBefore(ctx context.Context, before values.RepoID, limit int) ([]entities.Repo, error) {
	rows, err := r.queryGetter.
		GetQueries(ctx).
		ListReposBefore(ctx, db.ListReposBeforeParams{ID: before, Limit: int64(limit)})
	if err != nil {
		return nil, eris.Wrap(err, "list repos before")
	}

	// The query returns nearest first.
	repos := make([]entities.Repo, len(rows))
	for i, row := range rows {
		repo, toErr := r.toRepo(row)
		if toErr != nil {
			return nil, toErr
		}

		repos[len(rows)-1-i] = *repo
	}

	return repos, nil
}

// UpdateRepo implements [ports.RepoRepository].
func (r *Repository) UpdateRepo(ctx context.Context, repo entities.Repo) (*entities.Repo, error) {
	row, err := r.queryGetter.GetQueries(ctx).UpdateRepo(ctx, db.UpdateRepoParams{
		Name:          repo.Name,
		Url:           repo.URL,
		Username:      repo.Username,
		Password:      r.seal(repo.ID, passwordColumn, repo.Password),
		SshPrivateKey: r.seal(repo.ID, sshPrivateKeyColumn, repo.SSHPrivateKey),
		Insecure:      repo.Insecure,
		ID:            repo.ID,
	})
	if err != nil {
		switch {
		case eris.Is(err, sql.ErrNoRows):
			return nil, domainerrors.ErrRepoNotFound
		case isUniqueViolation(err):
			return nil, domainerrors.ErrRepoURLTaken
		}

		return nil, eris.Wrap(err, "update repo")
	}

	return r.toRepo(row)
}

// DeleteRepoByID implements [ports.RepoRepository].
func (r *Repository) DeleteRepoByID(ctx context.Context, id values.RepoID) error {
	n, err := r.queryGetter.GetQueries(ctx).DeleteRepoByID(ctx, id)
	if err != nil {
		return eris.Wrap(err, "delete repo by id")
	}

	if n == 0 {
		return domainerrors.ErrRepoNotFound
	}

	return nil
}

func (r *Repository) toRepo(row db.Repository) (*entities.Repo, error) {
	password, err := r.open(row.ID, passwordColumn, row.Password)
	if err != nil {
		return nil, err
	}

	key, err := r.open(row.ID, sshPrivateKeyColumn, row.SshPrivateKey)
	if err != nil {
		return nil, err
	}

	return &entities.Repo{
		ID:            row.ID,
		Name:          row.Name,
		URL:           row.Url,
		Username:      row.Username,
		Password:      password,
		SSHPrivateKey: key,
		Insecure:      row.Insecure,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

// seal encrypts a secret bound to its row and column, so ciphertext moved elsewhere fails to open; "" stays "".
func (r *Repository) seal(id values.RepoID, column, secret string) string {
	if secret == "" {
		return ""
	}

	return base64.StdEncoding.EncodeToString(r.aead.Seal(nil, nil, []byte(secret), sealedData(id, column)))
}

func (r *Repository) open(id values.RepoID, column, sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", eris.Wrapf(err, "decode %s", column)
	}

	secret, err := r.aead.Open(nil, nil, raw, sealedData(id, column))
	if err != nil {
		return "", eris.Wrapf(err, "open %s", column)
	}

	return string(secret), nil
}

// sealedData is the associated data: id || column.
func sealedData(id values.RepoID, column string) []byte {
	return append(id[:], column...)
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error

	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
