package sqlite

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rotisserie/eris"

	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
)

var _ ports.CredentialRepository = (*Repository)(nil)

// UpdateCredential implements [ports.CredentialRepository].
func (r *Repository) UpdateCredential(ctx context.Context, credential webauthn.Credential) error {
	val, err := json.Marshal(credential)
	if err != nil {
		return eris.Wrap(err, "marshal credential value")
	}

	err = r.queryGetter.
		GetQueries(ctx).
		UpdateCredential(ctx, db.UpdateCredentialParams{
			Credential:   string(val),
			CredentialID: credentialKey(credential),
		})
	if err != nil {
		return eris.Wrap(err, "update credential")
	}

	return nil
}

// RevokeCredentialByID implements [ports.CredentialRepository].
func (r *Repository) RevokeCredentialByID(ctx context.Context, id values.CredentialID) error {
	err := r.queryGetter.
		GetQueries(ctx).
		RevokeCredentialByID(ctx, id)
	if err != nil {
		return eris.Wrap(err, "revoke credential by id")
	}

	return nil
}

func credentialKey(credential webauthn.Credential) string {
	return base64.RawURLEncoding.EncodeToString(credential.ID)
}
