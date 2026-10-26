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
	cursor values.Cursor,
) ([]entities.User, values.Cursors, error) {
	if err := authorize(actor); err != nil {
		return nil, values.Cursors{}, err
	}

	users, cursors, err := page(
		ctx, cursor,
		func(user entities.User) kernelvalues.UserID { return user.ID },
		u.userRepo.ListUsers, u.userRepo.ListUsersBefore,
	)

	return users, cursors, eris.Wrap(err, "list users")
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

// page loads the page at the cursor, fetching one row more than PageSize to learn whether the page
// beyond it exists, and returns the cursors of the neighbouring pages.
// Both cursors point next to an item of this page, so they stay valid if other rows come and go.
func page[T any](
	ctx context.Context,
	cursor values.Cursor,
	id func(T) uuid.UUID,
	after, before func(ctx context.Context, id uuid.UUID, limit int) ([]T, error),
) ([]T, values.Cursors, error) {
	list := after
	if cursor.Before {
		list = before
	}

	items, err := list(ctx, cursor.ID, PageSize+1)
	if err != nil {
		return nil, values.Cursors{}, err
	}

	more := len(items) > PageSize
	if more && cursor.Before {
		items = items[1:] // oldest first: the extra row is the oldest
	} else if more {
		items = items[:PageSize]
	}

	if len(items) == 0 {
		return items, values.Cursors{}, nil
	}

	// Going back, the pivot row is still ahead; going forward, it is still behind (unless this is page one).
	hasPrev, hasNext := cursor.ID != (uuid.UUID{}), more
	if cursor.Before {
		hasPrev, hasNext = more, true
	}

	var cursors values.Cursors

	if hasPrev {
		cursors.Prev = &values.Cursor{ID: id(items[0]), Before: true}
	}

	if hasNext {
		cursors.Next = &values.Cursor{ID: id(items[len(items)-1])}
	}

	return items, cursors, nil
}
