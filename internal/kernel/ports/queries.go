package ports

import (
	"context"
)

type QueryGetter[T any] interface {
	GetQueries(ctx context.Context) *T
}
