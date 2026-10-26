package app_test

import (
	"context"
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
)

type UsersSuite struct {
	suite.Suite

	repo  *mocks.MockUserRepository
	users *app.Users
	owner *entities.User
}

func TestUsersSuite(t *testing.T) {
	suite.Run(t, new(UsersSuite))
}

func (s *UsersSuite) SetupTest() {
	s.repo = mocks.NewMockUserRepository(s.T())
	s.users = app.NewUsers(s.repo)
	s.owner = &entities.User{ID: uuid.NewV7(), Name: "owner", Role: values.RoleOwner}
}

// Every method refuses anyone but the owner before touching the repository; the mock fails on any call.
func (s *UsersSuite) TestForbidden() {
	actors := map[string]*entities.User{
		"no actor": nil,
		"user":     {ID: uuid.NewV7(), Role: values.RoleUser},
		"admin":    {ID: uuid.NewV7(), Role: values.RoleAdmin},
	}

	calls := map[string]func(ctx context.Context, actor *entities.User) error{
		"ListUsers": func(ctx context.Context, actor *entities.User) error {
			_, _, err := s.users.ListUsers(ctx, actor, values.Cursor{})

			return err
		},
		"EnableUser": func(ctx context.Context, actor *entities.User) error {
			return s.users.EnableUser(ctx, actor, uuid.NewV7())
		},
		"DisableUser": func(ctx context.Context, actor *entities.User) error {
			return s.users.DisableUser(ctx, actor, uuid.NewV7())
		},
		"SetRole": func(ctx context.Context, actor *entities.User) error {
			return s.users.SetRole(ctx, actor, uuid.NewV7(), values.RoleAdmin)
		},
	}

	for actorName, actor := range actors {
		for callName, call := range calls {
			s.Run(callName+"/"+actorName, func() {
				s.ErrorIs(call(s.T().Context(), actor), domainerrors.ErrForbidden)
			})
		}
	}
}

func (s *UsersSuite) TestListUsers_Pages() {
	pivot := uuid.NewV7()
	tests := []struct {
		name     string
		cursor   values.Cursor
		stored   int // rows the store returns, oldest first
		wantLen  int
		wantNext bool
		wantPrev bool
	}{
		{name: "first page, empty", stored: 0},
		{name: "first page, partial", stored: 3, wantLen: 3},
		{name: "first page, exactly full", stored: app.PageSize, wantLen: app.PageSize},
		{name: "first page, more follow", stored: app.PageSize + 1, wantLen: app.PageSize, wantNext: true},
		{name: "after, last page", cursor: values.Cursor{ID: pivot}, stored: 3, wantLen: 3, wantPrev: true},
		{
			name: "after, in the middle", cursor: values.Cursor{ID: pivot}, stored: app.PageSize + 1,
			wantLen: app.PageSize, wantNext: true, wantPrev: true,
		},
		{name: "before, first page", cursor: values.Cursor{ID: pivot, Before: true}, stored: 3, wantLen: 3, wantNext: true},
		{
			name: "before, in the middle", cursor: values.Cursor{ID: pivot, Before: true}, stored: app.PageSize + 1,
			wantLen: app.PageSize, wantNext: true, wantPrev: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			stored := make([]entities.User, tt.stored)
			for i := range stored {
				stored[i] = entities.User{ID: uuid.NewV7()}
			}

			// Asks for one extra row to learn whether the page beyond exists.
			if tt.cursor.Before {
				s.repo.EXPECT().ListUsersBefore(mock.Anything, pivot, app.PageSize+1).Return(stored, nil).Once()
			} else {
				s.repo.EXPECT().ListUsers(mock.Anything, tt.cursor.ID, app.PageSize+1).Return(stored, nil).Once()
			}

			got, cursors, err := s.users.ListUsers(s.T().Context(), s.owner, tt.cursor)

			s.Require().NoError(err)
			s.Require().Len(got, tt.wantLen)

			if tt.wantNext {
				s.Equal(&values.Cursor{ID: got[len(got)-1].ID}, cursors.Next)
			} else {
				s.Nil(cursors.Next)
			}

			if tt.wantPrev {
				s.Equal(&values.Cursor{ID: got[0].ID, Before: true}, cursors.Prev)
			} else {
				s.Nil(cursors.Prev)
			}
		})
	}
}

func (s *UsersSuite) TestListUsers_Error() {
	s.repo.EXPECT().ListUsers(mock.Anything, uuid.UUID{}, app.PageSize+1).Return(nil, errBoom).Once()

	_, _, err := s.users.ListUsers(s.T().Context(), s.owner, values.Cursor{})

	s.ErrorIs(err, errBoom)
}

func (s *UsersSuite) TestEnableUser_Success() {
	id := uuid.NewV7()
	s.repo.EXPECT().EnableUserByID(mock.Anything, id).Return(nil).Once()

	s.NoError(s.users.EnableUser(s.T().Context(), s.owner, id))
}

func (s *UsersSuite) TestEnableUser_NotFound() {
	id := uuid.NewV7()
	s.repo.EXPECT().EnableUserByID(mock.Anything, id).Return(domainerrors.ErrUserNotFound).Once()

	s.ErrorIs(s.users.EnableUser(s.T().Context(), s.owner, id), domainerrors.ErrUserNotFound)
}

func (s *UsersSuite) TestDisableUser_Self() {
	s.ErrorIs(s.users.DisableUser(s.T().Context(), s.owner, s.owner.ID), domainerrors.ErrCannotManageSelf)
}

func (s *UsersSuite) TestDisableUser_Success() {
	id := uuid.NewV7()
	s.repo.EXPECT().DisableUserByID(mock.Anything, id).Return(nil).Once()

	s.NoError(s.users.DisableUser(s.T().Context(), s.owner, id))
}

func (s *UsersSuite) TestDisableUser_Error() {
	id := uuid.NewV7()
	s.repo.EXPECT().DisableUserByID(mock.Anything, id).Return(errBoom).Once()

	s.ErrorIs(s.users.DisableUser(s.T().Context(), s.owner, id), errBoom)
}

func (s *UsersSuite) TestSetRole_Success() {
	for _, role := range []values.Role{values.RoleAdmin, values.RoleUser} {
		s.Run(role.String(), func() {
			id := uuid.NewV7()
			s.repo.EXPECT().SetUserRoleByID(mock.Anything, id, role).Return(nil).Once()

			s.NoError(s.users.SetRole(s.T().Context(), s.owner, id, role))
		})
	}
}

func (s *UsersSuite) TestSetRole_Self() {
	s.ErrorIs(s.users.SetRole(s.T().Context(), s.owner, s.owner.ID, values.RoleUser), domainerrors.ErrCannotManageSelf)
}

func (s *UsersSuite) TestSetRole_Owner() {
	s.ErrorIs(s.users.SetRole(s.T().Context(), s.owner, uuid.NewV7(), values.RoleOwner), values.ErrInvalidRole)
}

func (s *UsersSuite) TestSetRole_RepositoryErrors() {
	for _, want := range []error{domainerrors.ErrUserNotFound, domainerrors.ErrUserDisabled, errBoom} {
		s.Run(want.Error(), func() {
			id := uuid.NewV7()
			s.repo.EXPECT().SetUserRoleByID(mock.Anything, id, values.RoleAdmin).Return(want).Once()

			s.ErrorIs(s.users.SetRole(s.T().Context(), s.owner, id, values.RoleAdmin), want)
		})
	}
}
