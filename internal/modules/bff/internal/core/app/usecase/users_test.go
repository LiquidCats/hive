package app_test

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
)

type UsersSuite struct {
	suite.Suite

	repo  *mocks.MockUserRepository
	users *app.Users
}

func TestUsersSuite(t *testing.T) {
	suite.Run(t, new(UsersSuite))
}

func (s *UsersSuite) SetupTest() {
	s.repo = mocks.NewMockUserRepository(s.T())
	s.users = app.NewUsers(s.repo)
}

func (s *UsersSuite) TestListUsers_Success() {
	want := []entities.User{{ID: uuid.NewV7(), Name: "alice"}}
	s.repo.EXPECT().ListUsers(mock.Anything).Return(want, nil).Once()

	got, err := s.users.ListUsers(s.T().Context())

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *UsersSuite) TestListUsers_Error() {
	s.repo.EXPECT().ListUsers(mock.Anything).Return(nil, errBoom).Once()

	_, err := s.users.ListUsers(s.T().Context())

	s.ErrorIs(err, errBoom)
}

func (s *UsersSuite) TestEnableUser_Success() {
	id := uuid.NewV7()
	s.repo.EXPECT().EnableUserByID(mock.Anything, id).Return(nil).Once()

	s.NoError(s.users.EnableUser(s.T().Context(), id))
}

func (s *UsersSuite) TestEnableUser_NotFound() {
	id := uuid.NewV7()
	s.repo.EXPECT().EnableUserByID(mock.Anything, id).Return(domainerrors.ErrUserNotFound).Once()

	s.ErrorIs(s.users.EnableUser(s.T().Context(), id), domainerrors.ErrUserNotFound)
}

func (s *UsersSuite) TestDisableUser_Self() {
	id := uuid.NewV7()

	s.ErrorIs(s.users.DisableUser(s.T().Context(), id, id), domainerrors.ErrCannotDisableSelf)
}

func (s *UsersSuite) TestDisableUser_Success() {
	id := uuid.NewV7()
	s.repo.EXPECT().DisableUserByID(mock.Anything, id).Return(nil).Once()

	s.NoError(s.users.DisableUser(s.T().Context(), uuid.NewV7(), id))
}

func (s *UsersSuite) TestDisableUser_Error() {
	id := uuid.NewV7()
	s.repo.EXPECT().DisableUserByID(mock.Anything, id).Return(errBoom).Once()

	s.ErrorIs(s.users.DisableUser(s.T().Context(), uuid.NewV7(), id), errBoom)
}
