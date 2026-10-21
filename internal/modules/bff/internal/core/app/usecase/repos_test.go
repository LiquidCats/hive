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

type ReposSuite struct {
	suite.Suite

	store  *mocks.MockRepoRepository
	remote *mocks.MockGitRemote
	repos  *app.Repos
	admin  *entities.User
	user   *entities.User
}

func TestReposSuite(t *testing.T) {
	suite.Run(t, new(ReposSuite))
}

func (s *ReposSuite) SetupTest() {
	s.store = mocks.NewMockRepoRepository(s.T())
	s.remote = mocks.NewMockGitRemote(s.T())
	s.repos = app.NewRepos(s.store, s.remote)
	s.admin = &entities.User{ID: uuid.NewV7(), Role: values.RoleAdmin}
	s.user = &entities.User{ID: uuid.NewV7(), Role: values.RoleUser}
}

// Refused before any store or remote call; the mocks fail on any call.
func (s *ReposSuite) TestForbidden() {
	in := entities.RepoInput{URL: "https://h/r"}
	reads := map[string]func(ctx context.Context, actor *entities.User) error{
		"ListRepos": func(ctx context.Context, actor *entities.User) error {
			_, _, err := s.repos.ListRepos(ctx, actor, values.Cursor{})

			return err
		},
		"GetRepo": func(ctx context.Context, actor *entities.User) error {
			_, err := s.repos.GetRepo(ctx, actor, uuid.NewV7())

			return err
		},
	}
	writes := map[string]func(ctx context.Context, actor *entities.User) error{
		"CreateRepo": func(ctx context.Context, actor *entities.User) error {
			_, err := s.repos.CreateRepo(ctx, actor, in)

			return err
		},
		"UpdateRepo": func(ctx context.Context, actor *entities.User) error {
			_, err := s.repos.UpdateRepo(ctx, actor, uuid.NewV7(), in)

			return err
		},
		"DeleteRepo": func(ctx context.Context, actor *entities.User) error {
			return s.repos.DeleteRepo(ctx, actor, uuid.NewV7())
		},
	}

	for name, call := range reads {
		s.Run(name+"/no actor", func() {
			s.ErrorIs(call(s.T().Context(), nil), domainerrors.ErrForbidden)
		})
	}

	for name, call := range writes {
		for actorName, actor := range map[string]*entities.User{"no actor": nil, "user": s.user} {
			s.Run(name+"/"+actorName, func() {
				s.ErrorIs(call(s.T().Context(), actor), domainerrors.ErrForbidden)
			})
		}
	}
}

func (s *ReposSuite) TestListRepos_Pages() {
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

			stored := make([]entities.Repo, tt.stored)
			for i := range stored {
				stored[i] = entities.Repo{ID: uuid.NewV7()}
			}

			// Asks for one extra row to learn whether the page beyond exists.
			if tt.cursor.Before {
				s.store.EXPECT().ListReposBefore(mock.Anything, pivot, app.PageSize+1).Return(stored, nil).Once()
			} else {
				s.store.EXPECT().ListRepos(mock.Anything, tt.cursor.ID, app.PageSize+1).Return(stored, nil).Once()
			}

			got, cursors, err := s.repos.ListRepos(s.T().Context(), s.user, tt.cursor)

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

func (s *ReposSuite) TestListRepos_Error() {
	s.store.EXPECT().ListRepos(mock.Anything, uuid.UUID{}, app.PageSize+1).Return(nil, errBoom).Once()

	_, _, err := s.repos.ListRepos(s.T().Context(), s.user, values.Cursor{})

	s.ErrorIs(err, errBoom)
}

func (s *ReposSuite) TestGetRepo() {
	want := &entities.Repo{ID: uuid.NewV7()}
	s.store.EXPECT().GetRepoByID(mock.Anything, want.ID).Return(want, nil).Once()

	got, err := s.repos.GetRepo(s.T().Context(), s.user, want.ID)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *ReposSuite) TestGetRepo_NotFound() {
	id := uuid.NewV7()
	s.store.EXPECT().GetRepoByID(mock.Anything, id).Return(nil, domainerrors.ErrRepoNotFound).Once()

	_, err := s.repos.GetRepo(s.T().Context(), s.user, id)

	s.ErrorIs(err, domainerrors.ErrRepoNotFound)
}

func (s *ReposSuite) TestCreateRepo_Success() {
	in := entities.RepoInput{Name: "apps", URL: "https://h/r", Username: "u", Password: ptr("p")}
	matches := mock.MatchedBy(func(repo entities.Repo) bool {
		return repo.ID != uuid.UUID{} && repo.Name == "apps" && repo.URL == in.URL && repo.Password == "p"
	})

	var checked entities.Repo
	s.remote.EXPECT().Check(mock.Anything, matches).
		Run(func(_ context.Context, repo entities.Repo) { checked = repo }).Return(nil).Once()
	s.store.EXPECT().CreateRepo(mock.Anything, matches).
		RunAndReturn(func(_ context.Context, repo entities.Repo) (*entities.Repo, error) {
			s.Equal(checked, repo, "stores what was checked")

			return &repo, nil
		}).Once()

	got, err := s.repos.CreateRepo(s.T().Context(), s.admin, in)

	s.Require().NoError(err)
	s.Equal(in.URL, got.URL)
}

func (s *ReposSuite) TestCreateRepo_InvalidSkipsRemoteAndStore() {
	for name, in := range map[string]entities.RepoInput{
		"file url":         {URL: "file:///etc/passwd"},
		"ssh without key":  {URL: "git@h:r"},
		"https with a key": {URL: "https://h/r", SSHPrivateKey: ptr("key")},
	} {
		s.Run(name, func() {
			_, err := s.repos.CreateRepo(s.T().Context(), s.admin, in)

			s.Error(err)
		})
	}
}

func (s *ReposSuite) TestCreateRepo_CheckFailsSkipsStore() {
	for _, want := range []error{domainerrors.ErrRepoUnreachable, domainerrors.ErrRepoAccessDenied} {
		s.Run(want.Error(), func() {
			s.SetupTest()
			s.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(want).Once()

			_, err := s.repos.CreateRepo(s.T().Context(), s.admin, entities.RepoInput{URL: "https://h/r"})

			s.ErrorIs(err, want)
		})
	}
}

func (s *ReposSuite) TestCreateRepo_URLTaken() {
	s.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(nil).Once()
	s.store.EXPECT().CreateRepo(mock.Anything, mock.Anything).Return(nil, domainerrors.ErrRepoURLTaken).Once()

	_, err := s.repos.CreateRepo(s.T().Context(), s.admin, entities.RepoInput{URL: "https://h/r"})

	s.ErrorIs(err, domainerrors.ErrRepoURLTaken)
}

func (s *ReposSuite) TestUpdateRepo_KeepsSecrets() {
	current := &entities.Repo{ID: uuid.NewV7(), URL: "https://h/r", Username: "u", Password: "secret"}
	want := entities.Repo{ID: current.ID, Name: "renamed", URL: current.URL, Username: "u", Password: "secret"}

	s.store.EXPECT().GetRepoByID(mock.Anything, current.ID).Return(current, nil).Once()
	s.remote.EXPECT().Check(mock.Anything, want).Return(nil).Once()
	s.store.EXPECT().UpdateRepo(mock.Anything, want).Return(&want, nil).Once()

	got, err := s.repos.UpdateRepo(s.T().Context(), s.admin, current.ID,
		entities.RepoInput{Name: "renamed", URL: current.URL, Username: "u"})

	s.Require().NoError(err)
	s.Equal("secret", got.Password)
}

func (s *ReposSuite) TestUpdateRepo_URLChangeNeedsCredentials() {
	current := &entities.Repo{ID: uuid.NewV7(), URL: "https://h/r", Username: "u", Password: "secret"}
	s.store.EXPECT().GetRepoByID(mock.Anything, current.ID).Return(current, nil).Once()

	_, err := s.repos.UpdateRepo(s.T().Context(), s.admin, current.ID,
		entities.RepoInput{URL: "https://evil/r", Username: "u"})

	s.ErrorIs(err, domainerrors.ErrRepoCredentialsRequired)
}

func (s *ReposSuite) TestUpdateRepo_CheckFailsSkipsStore() {
	current := &entities.Repo{ID: uuid.NewV7(), URL: "https://h/r"}
	s.store.EXPECT().GetRepoByID(mock.Anything, current.ID).Return(current, nil).Once()
	s.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(domainerrors.ErrRepoUnreachable).Once()

	_, err := s.repos.UpdateRepo(s.T().Context(), s.admin, current.ID, entities.RepoInput{URL: "https://h/r"})

	s.ErrorIs(err, domainerrors.ErrRepoUnreachable)
}

func (s *ReposSuite) TestUpdateRepo_StoreErrors() {
	for _, want := range []error{domainerrors.ErrRepoNotFound, domainerrors.ErrRepoURLTaken} {
		s.Run(want.Error(), func() {
			s.SetupTest()

			current := &entities.Repo{ID: uuid.NewV7(), URL: "https://h/r"}
			s.store.EXPECT().GetRepoByID(mock.Anything, current.ID).Return(current, nil).Once()
			s.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(nil).Once()
			s.store.EXPECT().UpdateRepo(mock.Anything, mock.Anything).Return(nil, want).Once()

			_, err := s.repos.UpdateRepo(s.T().Context(), s.admin, current.ID, entities.RepoInput{URL: "https://h/r2"})

			s.ErrorIs(err, want)
		})
	}
}

func (s *ReposSuite) TestUpdateRepo_NotFound() {
	id := uuid.NewV7()
	s.store.EXPECT().GetRepoByID(mock.Anything, id).Return(nil, domainerrors.ErrRepoNotFound).Once()

	_, err := s.repos.UpdateRepo(s.T().Context(), s.admin, id, entities.RepoInput{URL: "https://h/r"})

	s.ErrorIs(err, domainerrors.ErrRepoNotFound)
}

func (s *ReposSuite) TestDeleteRepo() {
	for name, want := range map[string]error{"success": nil, "not found": domainerrors.ErrRepoNotFound} {
		s.Run(name, func() {
			id := uuid.NewV7()
			s.store.EXPECT().DeleteRepoByID(mock.Anything, id).Return(want).Once()

			s.ErrorIs(s.repos.DeleteRepo(s.T().Context(), s.admin, id), want)
		})
	}
}

func ptr(v string) *string { return &v }
