package values

import "uuid"

type (
	ApplicationID = uuid.UUID
	ServiceID     = uuid.UUID
	SecretID      = uuid.UUID

	UserID       = uuid.UUID
	CeremonyID   = uuid.UUID
	CredentialID = uuid.UUID

	RepoID = uuid.UUID
)
