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

var _ ports.Repos = (*Repos)(nil)

type Repos struct {
	store  ports.RepoRepository
	remote ports.GitRemote
}

func NewRepos(store ports.RepoRepository, remote ports.GitRemote) *Repos {
	return &Repos{store: store, remote: remote}
}

// ListRepos implements [ports.Repos].
func (r *Repos) ListRepos(
	ctx context.Context,
	actor *entities.User,
	cursor values.Cursor,
) ([]entities.Repo, values.Cursors, error) {
	if err := authorizeRepos(actor, false); err != nil {
		return nil, values.Cursors{}, err
	}

	repos, cursors, err := page(
		ctx, cursor,
		func(repo entities.Repo) kernelvalues.RepoID { return repo.ID },
		r.store.ListRepos, r.store.ListReposBefore,
	)

	return repos, cursors, eris.Wrap(err, "list repos")
}

// GetRepo implements [ports.Repos].
func (r *Repos) GetRepo(ctx context.Context, actor *entities.User, id kernelvalues.RepoID) (*entities.Repo, error) {
	if err := authorizeRepos(actor, false); err != nil {
		return nil, err
	}

	repo, err := r.store.GetRepoByID(ctx, id)

	return repo, eris.Wrap(err, "get repo")
}

// CreateRepo implements [ports.Repos].
func (r *Repos) CreateRepo(ctx context.Context, actor *entities.User, in entities.RepoInput) (*entities.Repo, error) {
	if err := authorizeRepos(actor, true); err != nil {
		return nil, err
	}

	repo, err := in.ApplyTo(entities.Repo{ID: uuid.NewV7()})
	if err != nil {
		return nil, err
	}

	if err = r.check(ctx, repo); err != nil {
		return nil, err
	}

	stored, err := r.store.CreateRepo(ctx, repo)

	return stored, eris.Wrap(err, "create repo")
}

// UpdateRepo implements [ports.Repos].
func (r *Repos) UpdateRepo(
	ctx context.Context,
	actor *entities.User,
	id kernelvalues.RepoID,
	in entities.RepoInput,
) (*entities.Repo, error) {
	if err := authorizeRepos(actor, true); err != nil {
		return nil, err
	}

	current, err := r.store.GetRepoByID(ctx, id)
	if err != nil {
		return nil, eris.Wrap(err, "get repo")
	}

	repo, err := in.ApplyTo(*current)
	if err != nil {
		return nil, err
	}

	if err = r.check(ctx, repo); err != nil {
		return nil, err
	}

	stored, err := r.store.UpdateRepo(ctx, repo)

	return stored, eris.Wrap(err, "update repo")
}

// DeleteRepo implements [ports.Repos].
func (r *Repos) DeleteRepo(ctx context.Context, actor *entities.User, id kernelvalues.RepoID) error {
	if err := authorizeRepos(actor, true); err != nil {
		return err
	}

	return eris.Wrap(r.store.DeleteRepoByID(ctx, id), "delete repo")
}

// check validates the repository, then connects to it, so unreachable repositories are never stored.
func (r *Repos) check(ctx context.Context, repo entities.Repo) error {
	if err := repo.Validate(); err != nil {
		return err
	}

	return eris.Wrap(r.remote.Check(ctx, repo), "check repo")
}

// authorizeRepos lets every signed-in user read and admins write. The server checks roles too; repeated here
// so every inbound adapter gets it, before any lookup so readers cannot probe through write errors.
func authorizeRepos(actor *entities.User, write bool) error {
	if actor == nil || (write && actor.Role < values.RoleAdmin) {
		return domainerrors.ErrForbidden
	}

	return nil
}
