package app

import (
	"context"

	"github.com/rotisserie/eris"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

var _ ports.Users = (*Users)(nil)

type Users struct {
	userRepo ports.UserRepository
}

func NewUsers(userRepo ports.UserRepository) *Users {
	return &Users{userRepo: userRepo}
}

// ListUsers implements [ports.Users].
func (u *Users) ListUsers(ctx context.Context) ([]entities.User, error) {
	users, err := u.userRepo.ListUsers(ctx)

	return users, eris.Wrap(err, "list users")
}

// EnableUser implements [ports.Users].
func (u *Users) EnableUser(ctx context.Context, id kernelvalues.UserID) error {
	return eris.Wrap(u.userRepo.EnableUserByID(ctx, id), "enable user")
}

// DisableUser implements [ports.Users].
func (u *Users) DisableUser(ctx context.Context, actor, id kernelvalues.UserID) error {
	// Otherwise the last active user could lock everyone out.
	if actor == id {
		return domainerrors.ErrCannotDisableSelf
	}

	return eris.Wrap(u.userRepo.DisableUserByID(ctx, id), "disable user")
}
