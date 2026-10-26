package sqlite

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/rotisserie/eris"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	dblib "github.com/LiquidCats/libraries/db/v2/sqlite"
)

// keySize is the AES-256 key length.
const keySize = 32

type Repository struct {
	queryGetter *dblib.QueriesTxManager[db.Queries]
	// aead seals repository secrets at rest.
	aead cipher.AEAD
}

func NewRepository(qg *dblib.QueriesTxManager[db.Queries], aead cipher.AEAD) *Repository {
	return &Repository{
		queryGetter: qg,
		aead:        aead,
	}
}

// NewCipher returns the AES-256-GCM cipher for [NewRepository] from a 32-byte key.
// ponytail: single key, no rotation; prefix sealed values with a key ID when rotation is needed.
func NewCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, eris.New("encryption key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, eris.Wrap(err, "new aes cipher")
	}

	aead, err := cipher.NewGCMWithRandomNonce(block)

	return aead, eris.Wrap(err, "new gcm")
}
