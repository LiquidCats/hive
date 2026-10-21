package app

import (
	"context"
	"uuid"

	"github.com/rotisserie/eris"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

var _ ports.Users = (*Users)(nil)

// PageSize is the most items a list returns at once.
const PageSize = 10

type Users struct {
	userRepo ports.UserRepository
}

func NewUsers(userRepo ports.UserRepository) *Users {
	return &Users{userRepo: userRepo}
}

// ListUsers implements [ports.Users].
func (u *Users) ListUsers(
	ctx context.Context,
	actor *entities.User,
	after kernelvalues.UserID,
) ([]entities.User, kernelvalues.UserID, error) {
	if err := authorize(actor); err != nil {
		return nil, kernelvalues.UserID{}, err
	}

	users, err := u.userRepo.ListUsers(ctx, after, PageSize+1)
	if err != nil {
		return nil, kernelvalues.UserID{}, eris.Wrap(err, "list users")
	}

	users, next := nextPage(users, func(user entities.User) kernelvalues.UserID { return user.ID })

	return users, next, nil
}

// EnableUser implements [ports.Users].
func (u *Users) EnableUser(ctx context.Context, actor *entities.User, id kernelvalues.UserID) error {
	if err := authorize(actor); err != nil {
		return err
	}

	return eris.Wrap(u.userRepo.EnableUserByID(ctx, id), "enable user")
}

// DisableUser implements [ports.Users].
func (u *Users) DisableUser(ctx context.Context, actor *entities.User, id kernelvalues.UserID) error {
	if err := authorize(actor); err != nil {
		return err
	}

	// Otherwise the owner could lock everyone out.
	if actor.ID == id {
		return domainerrors.ErrCannotManageSelf
	}

	return eris.Wrap(u.userRepo.DisableUserByID(ctx, id), "disable user")
}

// SetRole implements [ports.Users].
func (u *Users) SetRole(ctx context.Context, actor *entities.User, id kernelvalues.UserID, role values.Role) error {
	if err := authorize(actor); err != nil {
		return err
	}

	// Demoting themselves would leave nobody to manage users.
	if actor.ID == id {
		return domainerrors.ErrCannotManageSelf
	}

	// There is exactly one owner, the first user.
	if role == values.RoleOwner {
		return values.ErrInvalidRole
	}

	return eris.Wrap(u.userRepo.SetUserRoleByID(ctx, id, role), "set user role")
}

// authorize lets only the owner manage users. Checked here, not only in the server, so every inbound adapter gets it;
// before any lookup, so other roles cannot probe which users exist.
func authorize(actor *entities.User) error {
	if actor == nil || actor.Role != values.RoleOwner {
		return domainerrors.ErrForbidden
	}

	return nil
}

// nextPage trims items, fetched as PageSize+1 so the extra one tells whether another page follows,
// and returns the cursor for the next page; zero on the last page.
func nextPage[T any](items []T, id func(T) uuid.UUID) ([]T, uuid.UUID) {
	if len(items) <= PageSize {
		return items, uuid.UUID{}
	}

	items = items[:PageSize]

	return items, id(items[PageSize-1])
}
