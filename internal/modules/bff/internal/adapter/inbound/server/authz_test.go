package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kernelvalues "github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/inbound/server"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	"github.com/LiquidCats/hive/internal/modules/bff/tests/mocks"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
)

// writePath is a test-only write route outside user management.
const writePath = "/things"

// hive is a server whose users sign in with the cookie named after their role; pending awaits approval.
type hive struct {
	router *echo.Echo
	repo   *sqlite.Repository
	remote *mocks.MockGitRemote
	ids    map[string]kernelvalues.UserID
}

func TestRoleAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		as         string
		method     string
		path       string // {pending}, {user} and {owner} become user IDs
		body       any
		wantStatus int
		wantCode   string
	}{
		{name: "user cannot list users", as: "user", method: http.MethodGet, path: "/users",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "admin cannot list users", as: "admin", method: http.MethodGet, path: "/users",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "user reads session", as: "user", method: http.MethodGet, path: "/auth/session",
			wantStatus: http.StatusOK},
		{name: "owner reads users", as: "owner", method: http.MethodGet, path: "/users", wantStatus: http.StatusOK},

		{name: "user cannot write", as: "user", method: http.MethodPost, path: writePath,
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "admin writes", as: "admin", method: http.MethodPost, path: writePath, wantStatus: http.StatusNoContent},
		{name: "owner writes", as: "owner", method: http.MethodPost, path: writePath, wantStatus: http.StatusNoContent},

		{name: "user cannot approve", as: "user", method: http.MethodPost, path: "/users/{pending}/enable",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "admin cannot approve", as: "admin", method: http.MethodPost, path: "/users/{pending}/enable",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "owner approves", as: "owner", method: http.MethodPost, path: "/users/{pending}/enable",
			wantStatus: http.StatusNoContent},

		{name: "admin cannot disable", as: "admin", method: http.MethodPost, path: "/users/{user}/disable",
			wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "owner disables", as: "owner", method: http.MethodPost, path: "/users/{user}/disable",
			wantStatus: http.StatusNoContent},

		{name: "user cannot set role", as: "user", method: http.MethodPost, path: "/users/{user}/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "admin cannot set role", as: "admin", method: http.MethodPost, path: "/users/{user}/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusForbidden, wantCode: "forbidden"},
		{name: "owner sets role", as: "owner", method: http.MethodPost, path: "/users/{user}/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusNoContent},
		{name: "pending user cannot get a role", as: "owner", method: http.MethodPost, path: "/users/{pending}/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusConflict, wantCode: "user_disabled"},
		{name: "owner cannot change self", as: "owner", method: http.MethodPost, path: "/users/{owner}/role",
			body: map[string]string{"role": "user"}, wantStatus: http.StatusBadRequest, wantCode: "cannot_manage_self"},
		{name: "owner cannot disable self", as: "owner", method: http.MethodPost, path: "/users/{owner}/disable",
			wantStatus: http.StatusBadRequest, wantCode: "cannot_manage_self"},
		{name: "owner is not assignable", as: "owner", method: http.MethodPost, path: "/users/{user}/role",
			body: map[string]string{"role": "owner"}, wantStatus: http.StatusBadRequest, wantCode: "invalid_role"},
		{name: "unknown role", as: "owner", method: http.MethodPost, path: "/users/{user}/role",
			body: map[string]string{"role": "root"}, wantStatus: http.StatusBadRequest, wantCode: "invalid_role"},
		{name: "malformed body", as: "owner", method: http.MethodPost, path: "/users/{user}/role",
			body: "{", wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "malformed id", as: "owner", method: http.MethodPost, path: "/users/not-a-uuid/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusBadRequest, wantCode: "bad_request"},
		{name: "unknown user", as: "owner", method: http.MethodPost, path: "/users/" + uuid.NewV7().String() + "/role",
			body: map[string]string{"role": "admin"}, wantStatus: http.StatusNotFound, wantCode: "user_not_found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newHive(t)

			rec := h.do(t, tt.as, tt.method, h.expand(tt.path), tt.body)

			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())

			if tt.wantCode != "" {
				var body server.Error
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tt.wantCode, body.Code)
			}
		})
	}
}

func TestRoleIsExposed(t *testing.T) {
	t.Parallel()

	h := newHive(t)

	for _, role := range []string{"owner", "admin", "user"} {
		rec := h.do(t, role, http.MethodGet, "/auth/session", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var session server.SessionUser
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &session))
		assert.Equal(t, server.Role(role), session.Role)
	}

	rec := h.do(t, "owner", http.MethodPost, h.expand("/users/{user}/role"), map[string]string{"role": "admin"})
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	rec = h.do(t, "owner", http.MethodGet, "/users", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var page server.UsersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))

	roles := map[string]server.Role{}
	for _, user := range page.Users {
		roles[user.Username] = user.Role
	}

	assert.Equal(t, map[string]server.Role{
		"owner":   server.RoleOwner,
		"admin":   server.RoleAdmin,
		"user":    server.RoleAdmin, // promoted above
		"pending": server.RoleUser,
	}, roles)
}

func TestListUsers_Pagination(t *testing.T) {
	t.Parallel()

	h := newHive(t) // 4 users

	for i := range 9 {
		_, err := h.repo.CreateUser(t.Context(), entities.User{
			ID:   uuid.NewV7(),
			Name: kernelvalues.Username(fmt.Sprintf("extra%d", i)),
		})
		require.NoError(t, err)
	}

	page := h.usersPage(t, "/users")
	require.Len(t, page.Users, 10)
	require.NotNil(t, page.NextCursor)
	assert.Equal(t, "owner", page.Users[0].Username)
	assert.Equal(t, page.Users[9].Id, *page.NextCursor)

	last := h.usersPage(t, "/users?cursor="+*page.NextCursor)
	require.Len(t, last.Users, 3)
	assert.Nil(t, last.NextCursor)
	assert.Equal(t, "extra8", last.Users[2].Username)

	rec := h.do(t, "owner", http.MethodGet, "/users?cursor=not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func (h *hive) usersPage(t *testing.T, path string) server.UsersPage {
	t.Helper()

	rec := h.do(t, "owner", http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var page server.UsersPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))

	return page
}

func newHive(t *testing.T) *hive {
	t.Helper()

	ctx := t.Context()

	// :memory: is per connection; Connect defaults to a single one.
	conn, err := db.Connect(ctx, db.WithDatabase(":memory:"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, db.MigrateUp(ctx, conn, migrations.Migrations))

	repo := sqlite.NewRepository(db.NewQueriesTxManager(db.NewTxManager(conn), queries.New(conn)), testCipher(t))
	clock := &fakeClock{now: time.Now()}
	remote := mocks.NewMockGitRemote(t)

	// Sessions are inserted directly, so no WebAuthn ceremony runs and Auth needs no *webauthn.WebAuthn.
	srv := server.NewServer(
		app.NewAuth(nil, repo, repo, repo, repo, clock, sessionTTL),
		app.NewUsers(repo),
		app.NewRepos(repo, remote),
		slog.New(slog.DiscardHandler),
		sessionTTL,
	)

	router := echo.New()
	router.Use(srv.RequireSession)
	server.RegisterHandlers(router, srv)
	router.POST(writePath, func(ctx *echo.Context) error { return ctx.NoContent(http.StatusNoContent) })

	h := &hive{router: router, repo: repo, remote: remote, ids: map[string]kernelvalues.UserID{}}

	// Created in this order, the first one is the owner and the rest are pending users.
	for _, name := range []string{"owner", "admin", "user", "pending"} {
		user, createErr := repo.CreateUser(ctx, entities.User{ID: uuid.NewV7(), Name: kernelvalues.Username(name)})
		require.NoError(t, createErr)
		h.ids[name] = user.ID

		if name == "pending" {
			continue
		}

		if name != "owner" {
			require.NoError(t, repo.EnableUserByID(ctx, user.ID))
		}

		require.NoError(t, repo.CreateSession(ctx, entities.Session{
			TokenHash: hashToken(name),
			UserID:    user.ID,
			ExpiresAt: clock.now.Add(sessionTTL),
		}))
	}

	require.NoError(t, repo.SetUserRoleByID(ctx, h.ids["admin"], values.RoleAdmin))

	return h
}

// expand replaces {name} placeholders with that user's ID.
func (h *hive) expand(path string) string {
	for name, id := range h.ids {
		path = strings.ReplaceAll(path, "{"+name+"}", id.String())
	}

	return path
}

// do sends the request signed in as the named user; a string body is sent raw.
func (h *hive) do(t *testing.T, as, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var raw []byte

	switch b := body.(type) {
	case nil:
	case string:
		raw = []byte(b)
	default:
		var err error
		raw, err = json.Marshal(b)
		require.NoError(t, err)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: as})

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	return rec
}

// hashToken mirrors how Auth stores session tokens.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
