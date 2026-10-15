package sqlite

import (
	"context"
	"database/sql"
	"encoding/json/v2"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rotisserie/eris"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

var _ ports.CeremonyRepository = (*Repository)(nil)

// CreateCeremony implements [ports.CeremonyRepository].
func (r *Repository) CreateCeremony(ctx context.Context, ceremony entities.Ceremony) error {
	val, err := json.Marshal(ceremony.Session)
	if err != nil {
		return eris.Wrap(err, "marshal session")
	}

	queries := r.queryGetter.GetQueries(ctx)

	err = queries.DeleteStaleCeremonies(ctx)
	if err != nil {
		return eris.Wrap(err, "delete stale ceremonies")
	}

	err = queries.CreateCeremony(ctx, db.CreateCeremonyParams{
		ID:        ceremony.ID,
		Challenge: ceremony.Session.Challenge,
		Type:      ceremony.Type.String(),
		Username:  ceremony.Username.String(),
		Session:   string(val),
	})
	if err != nil {
		return eris.Wrap(err, "create ceremony")
	}

	return nil
}

// TakeCeremony implements [ports.CeremonyRepository].
func (r *Repository) TakeCeremony(ctx context.Context, challenge string) (*entities.Ceremony, error) {
	ceremony, err := r.queryGetter.
		GetQueries(ctx).
		TakeCeremonyByChallenge(ctx, challenge)
	if err != nil {
		if eris.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.ErrCeremonyNotFound
		}

		return nil, eris.Wrap(err, "take ceremony")
	}

	var session webauthn.SessionData

	err = json.Unmarshal([]byte(ceremony.Session), &session)
	if err != nil {
		return nil, eris.Wrap(err, "unmarshal session")
	}

	var ceremonyType values.CeremonyType

	err = ceremonyType.UnmarshalText([]byte(ceremony.Type))
	if err != nil {
		return nil, eris.Wrap(err, "unmarshal ceremony type")
	}

	return &entities.Ceremony{
		ID:       ceremony.ID,
		Type:     ceremonyType,
		Username: kernelvalues.Username(ceremony.Username),
		Session:  session,
	}, nil
}
