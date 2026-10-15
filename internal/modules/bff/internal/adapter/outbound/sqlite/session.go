package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/rotisserie/eris"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

var _ ports.SessionRepository = (*Repository)(nil)

// CreateSession implements [ports.SessionRepository].
func (r *Repository) CreateSession(ctx context.Context, session entities.Session) error {
	queries := r.queryGetter.GetQueries(ctx)

	err := queries.DeleteExpiredSessions(ctx)
	if err != nil {
		return eris.Wrap(err, "delete expired sessions")
	}

	err = queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: session.TokenHash,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt.Unix(),
	})

	return eris.Wrap(err, "create session")
}

// GetSessionUser implements [ports.SessionRepository].
func (r *Repository) GetSessionUser(ctx context.Context, tokenHash string) (*entities.User, time.Time, error) {
	row, err := r.queryGetter.
		GetQueries(ctx).
		GetSessionUser(ctx, tokenHash)
	if err != nil {
		if eris.Is(err, sql.ErrNoRows) {
			return nil, time.Time{}, domainerrors.ErrUnauthenticated
		}

		return nil, time.Time{}, eris.Wrap(err, "get session user")
	}

	user := toUser(row.User)

	return &user, time.Unix(row.ExpiresAt, 0), nil
}

// DeleteSession implements [ports.SessionRepository].
func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	return eris.Wrap(r.queryGetter.GetQueries(ctx).DeleteSession(ctx, tokenHash), "delete session")
}
