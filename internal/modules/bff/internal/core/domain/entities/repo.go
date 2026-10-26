package entities

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/LiquidCats/hive/internal/kernel/values"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

const (
	maxRepoNameLen = 255
	maxRepoURLLen  = 2048
)

// scpLike is git's user@host:path form, SSH with the user from the URL.
var scpLike = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9][A-Za-z0-9.-]*:\S+$`)

// Repo is a git repository the system deploys from. Password and SSHPrivateKey are plaintext in memory,
// empty when unset; they never leave the server.
type Repo struct {
	ID            values.RepoID
	Name          string
	URL           string
	Username      string
	Password      string
	SSHPrivateKey string
	// Insecure skips TLS verification for https and host key verification for ssh.
	Insecure  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RepoInput is a create or update request. A nil secret keeps the stored one, "" clears it.
type RepoInput struct {
	Name          string
	URL           string
	Username      string
	Password      *string
	SSHPrivateKey *string
	Insecure      bool
}

// ApplyTo returns r with the input applied. [errors.ErrRepoCredentialsRequired] if the URL changes while
// stored secrets would be kept, so they cannot be redirected to another host.
func (in RepoInput) ApplyTo(r Repo) (Repo, error) {
	keepsSecret := (in.Password == nil && r.Password != "") || (in.SSHPrivateKey == nil && r.SSHPrivateKey != "")
	if keepsSecret && in.URL != r.URL {
		return Repo{}, domainerrors.ErrRepoCredentialsRequired
	}

	r.Name, r.URL, r.Username, r.Insecure = in.Name, in.URL, in.Username, in.Insecure

	if in.Password != nil {
		r.Password = *in.Password
	}

	if in.SSHPrivateKey != nil {
		r.SSHPrivateKey = *in.SSHPrivateKey
	}

	return r, nil
}

// Validate checks the name and URL and that the credentials fit the URL: https takes an optional username and
// password, ssh takes a private key with the user in the URL. Only https and ssh are allowed: file:// would read
// server files, http:// would send credentials in clear. URLs carry no secrets since they are returned to clients.
func (r Repo) Validate() error {
	if len(r.Name) > maxRepoNameLen || strings.ContainsFunc(r.Name, unicode.IsControl) {
		return domainerrors.ErrInvalidRepoName
	}

	if len(r.URL) > maxRepoURLLen || strings.ContainsFunc(r.URL, unicode.IsControl) {
		return domainerrors.ErrInvalidRepoURL
	}

	if scpLike.MatchString(r.URL) {
		return r.validateSSHCredentials()
	}

	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" || u.Opaque != "" {
		return domainerrors.ErrInvalidRepoURL
	}

	switch u.Scheme {
	case "https":
		if u.User != nil {
			return domainerrors.ErrInvalidRepoURL
		}

		if r.SSHPrivateKey != "" || (r.Password != "" && r.Username == "") {
			return domainerrors.ErrInvalidRepoCredentials
		}

		return nil
	case "ssh":
		if _, hasPassword := u.User.Password(); hasPassword {
			return domainerrors.ErrInvalidRepoURL
		}

		return r.validateSSHCredentials()
	default:
		return domainerrors.ErrInvalidRepoURL
	}
}

func (r Repo) validateSSHCredentials() error {
	if r.SSHPrivateKey == "" || r.Username != "" || r.Password != "" {
		return domainerrors.ErrInvalidRepoCredentials
	}

	return nil
}
