package entities_test

import (
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

func TestRepo_Validate(t *testing.T) {
	t.Parallel()

	const key = "-----BEGIN OPENSSH PRIVATE KEY-----"

	tests := []struct {
		name    string
		repo    entities.Repo
		wantErr error
	}{
		{name: "public https", repo: entities.Repo{URL: "https://github.com/argoproj/argocd-example-apps.git"}},
		{name: "https with port", repo: entities.Repo{URL: "https://git.example.com:8443/org/repo"}},
		{name: "https username only", repo: entities.Repo{URL: "https://h/r", Username: "token"}},
		{name: "https username and password", repo: entities.Repo{URL: "https://h/r", Username: "u", Password: "p"}},
		{name: "ssh url", repo: entities.Repo{URL: "ssh://git@github.com/org/repo.git", SSHPrivateKey: key}},
		{name: "ssh url with port, no user", repo: entities.Repo{URL: "ssh://host:2222/repo", SSHPrivateKey: key}},
		{name: "scp-like", repo: entities.Repo{URL: "git@github.com:org/repo.git", SSHPrivateKey: key}},
		{name: "name at limit", repo: entities.Repo{URL: "https://h/r", Name: strings.Repeat("n", 255)}},

		{name: "empty url", repo: entities.Repo{}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "file url", repo: entities.Repo{URL: "file:///etc/passwd"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "local path", repo: entities.Repo{URL: "/srv/git/repo"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "http", repo: entities.Repo{URL: "http://h/r"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "git protocol", repo: entities.Repo{URL: "git://h/r"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "https userinfo", repo: entities.Repo{URL: "https://u:token@h/r"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "https user only", repo: entities.Repo{URL: "https://u@h/r"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "ssh password", repo: entities.Repo{URL: "ssh://git:pw@h/r", SSHPrivateKey: key},
			wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "no host", repo: entities.Repo{URL: "https:///r"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "scp-like without user", repo: entities.Repo{URL: "github.com:org/repo.git", SSHPrivateKey: key},
			wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "control character", repo: entities.Repo{URL: "https://h/r\n"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "url too long", repo: entities.Repo{URL: "https://h/" + strings.Repeat("r", 2048)},
			wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "name too long", repo: entities.Repo{URL: "https://h/r", Name: strings.Repeat("n", 256)},
			wantErr: domainerrors.ErrInvalidRepoName},

		{name: "https with ssh key", repo: entities.Repo{URL: "https://h/r", SSHPrivateKey: key},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
		{name: "https password without username", repo: entities.Repo{URL: "https://h/r", Password: "p"},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
		{name: "ssh without key", repo: entities.Repo{URL: "ssh://git@h/r"},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
		{name: "ssh with username", repo: entities.Repo{URL: "ssh://git@h/r", Username: "u", SSHPrivateKey: key},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
		{name: "scp-like with password", repo: entities.Repo{URL: "git@h:r", Password: "p", SSHPrivateKey: key},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.repo.Validate()

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestRepoInput_ApplyTo(t *testing.T) {
	t.Parallel()

	stored := entities.Repo{
		ID:       uuid.NewV7(),
		Name:     "old",
		URL:      "https://h/r",
		Username: "u",
		Password: "secret",
	}

	tests := []struct {
		name    string
		in      entities.RepoInput
		want    entities.Repo
		wantErr error
	}{
		{
			name: "omitted secret is kept",
			in:   entities.RepoInput{Name: "new", URL: "https://h/r", Username: "u2", Insecure: true},
			want: entities.Repo{ID: stored.ID, Name: "new", URL: "https://h/r", Username: "u2", Password: "secret",
				Insecure: true},
		},
		{
			name: "empty secret clears",
			in:   entities.RepoInput{URL: "https://h/r", Password: ptr("")},
			want: entities.Repo{ID: stored.ID, URL: "https://h/r"},
		},
		{
			name: "new secret replaces",
			in:   entities.RepoInput{URL: "https://h/r", Username: "u", Password: ptr("other")},
			want: entities.Repo{ID: stored.ID, URL: "https://h/r", Username: "u", Password: "other"},
		},
		{
			name: "url change with new secret",
			in:   entities.RepoInput{URL: "https://other/r", Username: "u", Password: ptr("p")},
			want: entities.Repo{ID: stored.ID, URL: "https://other/r", Username: "u", Password: "p"},
		},
		{
			name:    "url change keeping secret",
			in:      entities.RepoInput{URL: "https://evil/r", Username: "u"},
			wantErr: domainerrors.ErrRepoCredentialsRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.in.ApplyTo(stored)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepoInput_ApplyTo_KeepsSSHKeyOnlyForSameURL(t *testing.T) {
	t.Parallel()

	stored := entities.Repo{URL: "git@h:r", SSHPrivateKey: "key"}

	got, err := entities.RepoInput{URL: "git@h:r"}.ApplyTo(stored)
	require.NoError(t, err)
	assert.Equal(t, "key", got.SSHPrivateKey)

	_, err = entities.RepoInput{URL: "git@evil:r"}.ApplyTo(stored)
	require.ErrorIs(t, err, domainerrors.ErrRepoCredentialsRequired)
}

func ptr(s string) *string { return &s }
