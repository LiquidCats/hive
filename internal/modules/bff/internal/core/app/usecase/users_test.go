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
			_, _, err := s.users.ListUsers(ctx, actor, uuid.UUID{})

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
	tests := []struct {
		name     string
		stored   int // rows the repository returns
		wantLen  int
		wantNext bool
	}{
		{name: "empty", stored: 0, wantLen: 0},
		{name: "partial page", stored: 3, wantLen: 3},
		{name: "exactly one page", stored: app.UsersPageSize, wantLen: app.UsersPageSize},
		{name: "more pages", stored: app.UsersPageSize + 1, wantLen: app.UsersPageSize, wantNext: true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			after := uuid.NewV7()
			stored := make([]entities.User, tt.stored)
			for i := range stored {
				stored[i] = entities.User{ID: uuid.NewV7()}
			}

			// Asks for one extra row to learn whether a next page exists.
			s.repo.EXPECT().ListUsers(mock.Anything, after, app.UsersPageSize+1).Return(stored, nil).Once()

			got, next, err := s.users.ListUsers(s.T().Context(), s.owner, after)

			s.Require().NoError(err)
			s.Len(got, tt.wantLen)

			if tt.wantNext {
				s.Equal(got[len(got)-1].ID, next)
			} else {
				s.Zero(next)
			}
		})
	}
}

func (s *UsersSuite) TestListUsers_Error() {
	s.repo.EXPECT().ListUsers(mock.Anything, uuid.UUID{}, app.UsersPageSize+1).Return(nil, errBoom).Once()

	_, _, err := s.users.ListUsers(s.T().Context(), s.owner, uuid.UUID{})

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
