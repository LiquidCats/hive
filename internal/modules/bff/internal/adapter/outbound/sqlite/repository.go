package sqlite

import (
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	dblib "github.com/LiquidCats/libraries/db/v2/sqlite"
)

type Repository struct {
	queryGetter *dblib.QueriesTxManager[db.Queries]
}

func NewRepository(qg *dblib.QueriesTxManager[db.Queries]) *Repository {
	return &Repository{
		queryGetter: qg,
	}
}
