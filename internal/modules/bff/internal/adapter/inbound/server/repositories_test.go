package server_test

import (
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/inbound/server"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
)

const repoURL = "https://git.test/org/apps.git"

func TestRepositories_Access(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		as         string
		method     string
		path       string // {repo} becomes the stored repository's ID
		body       any
		checks     bool // the remote is checked, and passes
		wantStatus int
		wantCode   string
	}{
		{name: "user lists", as: "user", method: http.MethodGet, path: "/repositories", wantStatus: http.StatusOK},
		{name: "user gets", as: "user", method: http.MethodGet, path: "/repositories/{repo}", wantStatus: http.StatusOK},
		{name: "user cannot create", as: "user", method: http.MethodPost, path: "/repositories",
			body: map[string]any{"url": "https://git.test/other"}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user cannot update", as: "user", method: http.MethodPut, path: "/repositories/{repo}",
			body: map[string]any{"url": repoURL}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user cannot delete", as: "user", method: http.MethodDelete, path: "/repositories/{repo}",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "admin creates", as: "admin", method: http.MethodPost, path: "/repositories",
			body: map[string]any{"url": "https://git.test/other"}, checks: true, wantStatus: http.StatusCreated},
		{name: "admin updates", as: "admin", method: http.MethodPut, path: "/repositories/{repo}",
			body: map[string]any{"url": repoURL, "username": "u"}, checks: true, wantStatus: http.StatusOK},
		{name: "admin deletes", as: "admin", method: http.MethodDelete, path: "/repositories/{repo}",
			wantStatus: http.StatusNoContent},
		{name: "owner creates", as: "owner", method: http.MethodPost, path: "/repositories",
			body: map[string]any{"url": "https://git.test/other"}, checks: true, wantStatus: http.StatusCreated},

		{name: "url taken", as: "admin", method: http.MethodPost, path: "/repositories",
			body: map[string]any{"url": repoURL, "username": "u", "password": "p"}, checks: true,
			wantStatus: http.StatusConflict, wantCode: "repo_url_taken"},
		{name: "file url", as: "admin", method: http.MethodPost, path: "/repositories",
			body: map[string]any{"url": "file:///etc"}, wantStatus: http.StatusBadRequest, wantCode: "invalid_repo_url"},
		{name: "token in url", as: "admin", method: http.MethodPost, path: "/repositories",
			body:       map[string]any{"url": "https://u:token@git.test/r"},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_repo_url"},
		{name: "ssh without key", as: "admin", method: http.MethodPost, path: "/repositories",
			body:       map[string]any{"url": "git@git.test:org/r.git"},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_repo_credentials"},
		{name: "long name", as: "admin", method: http.MethodPost, path: "/repositories",
			body:       map[string]any{"url": "https://git.test/r", "name": strings.Repeat("n", 256)},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_repo_name"},
		{name: "url change keeping password", as: "admin", method: http.MethodPut, path: "/repositories/{repo}",
			body:       map[string]any{"url": "https://evil.test/r", "username": "u"},
			wantStatus: http.StatusBadRequest, wantCode: "repo_credentials_required"},
		{name: "malformed body", as: "admin", method: http.MethodPost, path: "/repositories", body: "{",
			wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "malformed update body", as: "admin", method: http.MethodPut, path: "/repositories/{repo}", body: "{",
			wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "malformed id", as: "user", method: http.MethodGet, path: "/repositories/not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "malformed update id", as: "admin", method: http.MethodPut, path: "/repositories/not-a-uuid",
			body: map[string]any{"url": repoURL}, wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "malformed delete id", as: "admin", method: http.MethodDelete, path: "/repositories/not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "unknown repo", as: "user", method: http.MethodGet, path: "/repositories/" + uuid.NewV7().String(),
			wantStatus: http.StatusNotFound, wantCode: "repo_not_found"},
		{name: "delete unknown repo", as: "admin", method: http.MethodDelete,
			path: "/repositories/" + uuid.NewV7().String(), wantStatus: http.StatusNotFound, wantCode: "repo_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newHive(t)
			stored := h.createRepo(t, entities.Repo{URL: repoURL, Username: "u", Password: "secret"})

			if tt.checks {
				h.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(nil).Once()
			}

			path := strings.ReplaceAll(tt.path, "{repo}", stored.ID.String())
			rec := h.do(t, tt.as, tt.method, path, tt.body)

			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())

			if tt.wantCode != "" {
				var body server.Error
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body.Code)
			}
		})
	}
}

func TestRepositories_CheckFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err      error
		wantCode string
		wantMsg  string
	}{
		{err: domainerrors.ErrRepoUnreachable, wantCode: "repo_unreachable", wantMsg: "Could not connect to the repository"},
		{err: domainerrors.ErrRepoAccessDenied, wantCode: "repo_access_denied",
			wantMsg: "Repository not found or credentials rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			t.Parallel()

			h := newHive(t)
			// The git error names internal hosts; it goes to the log, not the client.
			h.remote.EXPECT().Check(mock.Anything, mock.Anything).
				Return(fmt.Errorf("%w: dial tcp 10.0.0.7:443: connection refused", tt.err)).Once()

			rec := h.do(t, "admin", http.MethodPost, "/repositories", map[string]any{"url": repoURL})

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.JSONEq(t, fmt.Sprintf(`{"code":%q,"message":%q}`, tt.wantCode, tt.wantMsg), rec.Body.String())
		})
	}
}

func TestRepositories_SecretsNeverReturned(t *testing.T) {
	t.Parallel()

	h := newHive(t)
	h.remote.EXPECT().Check(mock.Anything, mock.Anything).Return(nil).Times(2)

	rec := h.do(t, "admin", http.MethodPost, "/repositories", map[string]any{
		"name": "apps", "url": repoURL, "username": "u", "password": "hunter2", "insecure": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "hunter2")

	var created server.Repository
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "apps", created.Name)
	assert.Equal(t, "u", created.Username)
	assert.True(t, created.Insecure)
	assert.True(t, created.HasPassword)
	assert.False(t, created.HasSshPrivateKey)

	// Omitting the password on update keeps it.
	rec = h.do(t, "admin", http.MethodPut, "/repositories/"+created.Id, map[string]any{"url": repoURL, "username": "u"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	for _, path := range []string{"/repositories", "/repositories/" + created.Id} {
		rec = h.do(t, "user", http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "hunter2", path)
		assert.NotContains(t, rec.Body.String(), `"password"`, path)
		assert.Contains(t, rec.Body.String(), `"hasPassword":true`, path)
	}
}

func TestListRepositories_Pagination(t *testing.T) {
	t.Parallel()

	h := newHive(t)

	for i := range 12 {
		h.createRepo(t, entities.Repo{URL: fmt.Sprintf("https://git.test/r%02d", i)})
	}

	page := h.reposPage(t, "/repositories")
	require.Len(t, page.Repositories, 10)
	require.NotNil(t, page.NextCursor)
	assert.Equal(t, "https://git.test/r00", page.Repositories[0].Url)
	assert.Equal(t, page.Repositories[9].Id, *page.NextCursor)

	last := h.reposPage(t, "/repositories?cursor="+*page.NextCursor)
	require.Len(t, last.Repositories, 2)
	assert.Nil(t, last.NextCursor)
	assert.Equal(t, "https://git.test/r11", last.Repositories[1].Url)

	rec := h.do(t, "user", http.MethodGet, "/repositories?cursor=not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func (h *hive) createRepo(t *testing.T, repo entities.Repo) *entities.Repo {
	t.Helper()

	repo.ID = uuid.NewV7()
	stored, err := h.repo.CreateRepo(t.Context(), repo)
	require.NoError(t, err)

	return stored
}

func (h *hive) reposPage(t *testing.T, path string) server.RepositoriesPage {
	t.Helper()

	rec := h.do(t, "user", http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var page server.RepositoriesPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))

	return page
}

// testCipher uses a fixed key; tests never depend on its value.
func testCipher(t *testing.T) cipher.AEAD {
	t.Helper()

	aead, err := sqlite.NewCipher([]byte(strings.Repeat("k", 32)))
	require.NoError(t, err)

	return aead
}
