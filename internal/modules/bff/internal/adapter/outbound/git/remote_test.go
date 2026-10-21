package git_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/git"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

func TestRemote_Check(t *testing.T) {
	t.Parallel()

	status := func(code int) *httptest.Server {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		t.Cleanup(srv.Close)

		return srv
	}

	// An empty repository advertises no refs: the service line, then flush packets.
	empty := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		_, _ = w.Write([]byte("001e# service=git-upload-pack\n00000000"))
	}))
	t.Cleanup(empty.Close)

	unauthorized, forbidden, missing := status(http.StatusUnauthorized), status(http.StatusForbidden),
		status(http.StatusNotFound)
	key := sshKey(t)

	tests := []struct {
		name    string
		repo    entities.Repo
		wantErr error
	}{
		{name: "empty repository is reachable", repo: entities.Repo{URL: empty.URL + "/r.git", Insecure: true}},
		{name: "closed port", repo: entities.Repo{URL: "https://" + closedAddr(t) + "/r.git"},
			wantErr: domainerrors.ErrRepoUnreachable},
		{name: "self-signed without insecure", repo: entities.Repo{URL: unauthorized.URL + "/r.git"},
			wantErr: domainerrors.ErrRepoUnreachable},
		{name: "401", repo: entities.Repo{URL: unauthorized.URL + "/r.git", Insecure: true},
			wantErr: domainerrors.ErrRepoAccessDenied},
		{name: "403 with credentials", repo: entities.Repo{URL: forbidden.URL + "/r.git", Username: "u", Password: "p",
			Insecure: true}, wantErr: domainerrors.ErrRepoAccessDenied},
		{name: "404", repo: entities.Repo{URL: missing.URL + "/r.git", Insecure: true},
			wantErr: domainerrors.ErrRepoAccessDenied},
		{name: "http is refused", repo: entities.Repo{URL: "http://127.0.0.1/r.git"},
			wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "file is refused", repo: entities.Repo{URL: "/etc"}, wantErr: domainerrors.ErrInvalidRepoURL},
		{name: "bad ssh key", repo: entities.Repo{URL: "ssh://git@127.0.0.1:1/r.git", SSHPrivateKey: "not a key"},
			wantErr: domainerrors.ErrInvalidRepoCredentials},
		{name: "ssh closed port", repo: entities.Repo{URL: "git@" + closedHost(t) + ":r.git", SSHPrivateKey: key,
			Insecure: true}, wantErr: domainerrors.ErrRepoUnreachable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := git.Remote{}.Check(t.Context(), tt.repo)

			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// Without insecure, host keys come from known_hosts; a missing file fails the check.
func TestRemote_Check_KnownHostsMissing(t *testing.T) {
	t.Setenv("SSH_KNOWN_HOSTS", filepath.Join(t.TempDir(), "missing"))

	err := git.Remote{}.Check(t.Context(), entities.Repo{URL: "ssh://git@127.0.0.1:1/r.git", SSHPrivateKey: sshKey(t)})

	assert.ErrorIs(t, err, domainerrors.ErrRepoUnreachable)
}

// closedAddr is a local address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, l.Close())

	return l.Addr().String()
}

// closedHost is closedAddr in scp-like host:port: form.
func closedHost(t *testing.T) string {
	t.Helper()

	host, port, err := net.SplitHostPort(closedAddr(t))
	require.NoError(t, err)

	return host + ":" + port
}

func sshKey(t *testing.T) string {
	t.Helper()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(private, "")
	require.NoError(t, err)

	return string(pem.EncodeToMemory(block))
}
