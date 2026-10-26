package git

import (
	"context"
	"errors"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rotisserie/eris"
	"golang.org/x/crypto/ssh"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

var _ ports.GitRemote = Remote{}

const checkTimeout = 15 * time.Second

// Remote checks repositories with an in-memory ls-remote.
// ponytail: checked only when written, no periodic status refresh; add POST /repositories/{id}/check
// when apps depend on live status.
type Remote struct{}

// Check implements [ports.GitRemote].
func (Remote) Check(ctx context.Context, repo entities.Repo) error {
	// The domain validated the URL; go-git parses it again, so refuse anything its parser reads differently.
	endpoint, err := transport.NewEndpoint(repo.URL)
	if err != nil || (endpoint.Protocol != "https" && endpoint.Protocol != "ssh") {
		return domainerrors.ErrInvalidRepoURL
	}

	auth, err := authMethod(endpoint, repo)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{Name: "origin", URLs: []string{repo.URL}})

	_, err = remote.ListContext(ctx, &gogit.ListOptions{Auth: auth, InsecureSkipTLS: repo.Insecure})

	switch {
	case err == nil, errors.Is(err, transport.ErrEmptyRemoteRepository):
		return nil
	case errors.Is(err, transport.ErrAuthenticationRequired),
		errors.Is(err, transport.ErrAuthorizationFailed),
		errors.Is(err, transport.ErrRepositoryNotFound):
		return eris.Wrap(domainerrors.ErrRepoAccessDenied, err.Error())
	default:
		return eris.Wrap(domainerrors.ErrRepoUnreachable, err.Error())
	}
}

func authMethod(endpoint *transport.Endpoint, repo entities.Repo) (transport.AuthMethod, error) {
	if endpoint.Protocol == "https" {
		if repo.Username == "" {
			return nil, nil //nolint:nilnil // anonymous
		}

		return &githttp.BasicAuth{Username: repo.Username, Password: repo.Password}, nil
	}

	user := endpoint.User
	if user == "" {
		user = "git"
	}

	auth, err := gitssh.NewPublicKeys(user, []byte(repo.SSHPrivateKey), "")
	if err != nil {
		return nil, eris.Wrap(domainerrors.ErrInvalidRepoCredentials, err.Error())
	}

	if repo.Insecure {
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // the admin opted out of host key checks
	} else if auth.HostKeyCallback, err = gitssh.NewKnownHostsCallback(); err != nil {
		// Reads SSH_KNOWN_HOSTS, else ~/.ssh/known_hosts and /etc/ssh/ssh_known_hosts.
		return nil, eris.Wrap(domainerrors.ErrRepoUnreachable, err.Error())
	}

	return auth, nil
}
