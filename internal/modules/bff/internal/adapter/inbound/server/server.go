package server

import (
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/labstack/echo/v5"

	kernelports "github.com/LiquidCats/hive/internal/kernel/ports"
	"github.com/LiquidCats/hive/internal/kernel/values"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/entities"
	domainerrors "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/errors"
	domainvalues "github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
	"github.com/LiquidCats/hive/pkg/logs"
)

const (
	sessionCookie = "session"
	userKey       = "user"
)

var (
	_ ServerInterface = (*Server)(nil)

	errBadRequest = errors.New("bad request")

	// publicPaths skip RequireSession; every other route needs a session. Keep in sync with `security: []` in api.yaml.
	// Logout is public so an expired session can still clear its cookie.
	publicPaths = map[string]bool{
		"/webauthn/register/start":  true,
		"/webauthn/register/finish": true,
		"/webauthn/login/start":     true,
		"/webauthn/login/finish":    true,
		"/auth/logout":              true,
	}
)

type Server struct {
	auth       ports.Auth
	users      ports.Users
	repos      ports.Repos
	log        kernelports.Logger[any]
	sessionTTL time.Duration
}

func NewServer(
	auth ports.Auth,
	users ports.Users,
	repos ports.Repos,
	log kernelports.Logger[any],
	sessionTTL time.Duration,
) *Server {
	return &Server{auth: auth, users: users, repos: repos, log: log, sessionTTL: sessionTTL}
}

// RequireSession is the router-wide middleware (echo Use): it runs after routing, so ctx.Path() is the route pattern.
// Routes are protected unless listed in publicPaths; handlers read the user with currentUser.
func (s *Server) RequireSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx *echo.Context) error {
		if publicPaths[ctx.Path()] {
			return next(ctx)
		}

		cookie, err := ctx.Cookie(sessionCookie)
		if err != nil {
			return s.fail(ctx, domainerrors.ErrUnauthenticated)
		}

		user, err := s.auth.Session(ctx.Request().Context(), cookie.Value)
		if err != nil {
			return s.fail(ctx, err)
		}

		// Users only read. User management is the owner's, checked by the Users use case.
		if ctx.Request().Method != http.MethodGet && user.Role < domainvalues.RoleAdmin {
			return s.fail(ctx, domainerrors.ErrForbidden)
		}

		ctx.Set(userKey, user)

		return next(ctx)
	}
}

// CreateChallenge implements [ServerInterface].
func (s *Server) CreateChallenge(ctx *echo.Context) error {
	var req UsernameRequest
	if err := ctx.Bind(&req); err != nil {
		return s.fail(ctx, errBadRequest)
	}

	creation, err := s.auth.BeginRegistration(ctx.Request().Context(), values.Username(req.Username))
	if err != nil {
		return s.fail(ctx, err)
	}

	return ctx.JSON(http.StatusOK, creation)
}

// CreateChallengeResponse implements [ServerInterface].
func (s *Server) CreateChallengeResponse(ctx *echo.Context) error {
	resp, err := protocol.ParseCredentialCreationResponseBody(ctx.Request().Body)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	token, err := s.auth.FinishRegistration(ctx.Request().Context(), resp)

	switch {
	case errors.Is(err, domainerrors.ErrApprovalPending):
		return ctx.JSON(http.StatusOK, RegistrationResult{Status: PendingApproval})
	case err != nil:
		return s.fail(ctx, err)
	}

	s.setSessionCookie(ctx, token, int(s.sessionTTL.Seconds()))

	return ctx.JSON(http.StatusOK, RegistrationResult{Status: Active})
}

// LoginChallenge implements [ServerInterface].
func (s *Server) LoginChallenge(ctx *echo.Context) error {
	assertion, err := s.auth.BeginLogin(ctx.Request().Context())
	if err != nil {
		return s.fail(ctx, err)
	}

	return ctx.JSON(http.StatusOK, assertion)
}

// LoginChallengeResponse implements [ServerInterface].
func (s *Server) LoginChallengeResponse(ctx *echo.Context) error {
	resp, err := protocol.ParseCredentialRequestResponseBody(ctx.Request().Body)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	token, err := s.auth.FinishLogin(ctx.Request().Context(), resp)
	if err != nil {
		return s.fail(ctx, err)
	}

	s.setSessionCookie(ctx, token, int(s.sessionTTL.Seconds()))

	return ctx.NoContent(http.StatusOK)
}

// GetSession implements [ServerInterface].
func (s *Server) GetSession(ctx *echo.Context) error {
	user := currentUser(ctx)

	return ctx.JSON(http.StatusOK, SessionUser{
		Id:       user.ID.String(),
		Username: user.Name.String(),
		Role:     Role(user.Role.String()),
	})
}

// Logout implements [ServerInterface].
func (s *Server) Logout(ctx *echo.Context) error {
	if cookie, err := ctx.Cookie(sessionCookie); err == nil {
		if err = s.auth.Logout(ctx.Request().Context(), cookie.Value); err != nil {
			return s.fail(ctx, err)
		}
	}

	s.setSessionCookie(ctx, "", -1)

	return ctx.NoContent(http.StatusNoContent)
}

// ListUsers implements [ServerInterface].
func (s *Server) ListUsers(ctx *echo.Context, params ListUsersParams) error {
	var after values.UserID

	if params.Cursor != nil {
		var err error
		if after, err = uuid.Parse(*params.Cursor); err != nil {
			return s.fail(ctx, errBadRequest)
		}
	}

	users, next, err := s.users.ListUsers(ctx.Request().Context(), currentUser(ctx), after)
	if err != nil {
		return s.fail(ctx, err)
	}

	resp := UsersPage{Users: make([]User, len(users))}
	if next != (values.UserID{}) {
		cursor := next.String()
		resp.NextCursor = &cursor
	}

	for i, user := range users {
		resp.Users[i] = User{
			Id:        user.ID.String(),
			Username:  user.Name.String(),
			Role:      Role(user.Role.String()),
			Disabled:  user.Disabled,
			CreatedAt: user.CreatedAt,
		}
	}

	return ctx.JSON(http.StatusOK, resp)
}

// EnableUser implements [ServerInterface].
func (s *Server) EnableUser(ctx *echo.Context, id UserID) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	if err = s.users.EnableUser(ctx.Request().Context(), currentUser(ctx), userID); err != nil {
		return s.fail(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
}

// DisableUser implements [ServerInterface].
func (s *Server) DisableUser(ctx *echo.Context, id UserID) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	if err = s.users.DisableUser(ctx.Request().Context(), currentUser(ctx), userID); err != nil {
		return s.fail(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
}

// SetUserRole implements [ServerInterface].
func (s *Server) SetUserRole(ctx *echo.Context, id UserID) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	var req RoleRequest
	if err = ctx.Bind(&req); err != nil {
		return s.fail(ctx, errBadRequest)
	}

	var role domainvalues.Role
	if err = role.UnmarshalText([]byte(req.Role)); err != nil {
		return s.fail(ctx, err)
	}

	if err = s.users.SetRole(ctx.Request().Context(), currentUser(ctx), userID, role); err != nil {
		return s.fail(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
}

// ListRepositories implements [ServerInterface].
func (s *Server) ListRepositories(ctx *echo.Context, params ListRepositoriesParams) error {
	var after values.RepoID

	if params.Cursor != nil {
		var err error
		if after, err = uuid.Parse(*params.Cursor); err != nil {
			return s.fail(ctx, errBadRequest)
		}
	}

	repos, next, err := s.repos.ListRepos(ctx.Request().Context(), currentUser(ctx), after)
	if err != nil {
		return s.fail(ctx, err)
	}

	resp := RepositoriesPage{Repositories: make([]Repository, len(repos))}
	if next != (values.RepoID{}) {
		cursor := next.String()
		resp.NextCursor = &cursor
	}

	for i, repo := range repos {
		resp.Repositories[i] = toRepository(repo)
	}

	return ctx.JSON(http.StatusOK, resp)
}

// CreateRepository implements [ServerInterface].
func (s *Server) CreateRepository(ctx *echo.Context) error {
	var req RepositoryInput
	if err := ctx.Bind(&req); err != nil {
		return s.fail(ctx, errBadRequest)
	}

	repo, err := s.repos.CreateRepo(ctx.Request().Context(), currentUser(ctx), toRepoInput(req))
	if err != nil {
		return s.fail(ctx, err)
	}

	return ctx.JSON(http.StatusCreated, toRepository(*repo))
}

// GetRepository implements [ServerInterface].
func (s *Server) GetRepository(ctx *echo.Context, id RepositoryID) error {
	repoID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	repo, err := s.repos.GetRepo(ctx.Request().Context(), currentUser(ctx), repoID)
	if err != nil {
		return s.fail(ctx, err)
	}

	return ctx.JSON(http.StatusOK, toRepository(*repo))
}

// UpdateRepository implements [ServerInterface].
func (s *Server) UpdateRepository(ctx *echo.Context, id RepositoryID) error {
	repoID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	var req RepositoryInput
	if err = ctx.Bind(&req); err != nil {
		return s.fail(ctx, errBadRequest)
	}

	repo, err := s.repos.UpdateRepo(ctx.Request().Context(), currentUser(ctx), repoID, toRepoInput(req))
	if err != nil {
		return s.fail(ctx, err)
	}

	return ctx.JSON(http.StatusOK, toRepository(*repo))
}

// DeleteRepository implements [ServerInterface].
func (s *Server) DeleteRepository(ctx *echo.Context, id RepositoryID) error {
	repoID, err := uuid.Parse(id)
	if err != nil {
		return s.fail(ctx, errBadRequest)
	}

	if err = s.repos.DeleteRepo(ctx.Request().Context(), currentUser(ctx), repoID); err != nil {
		return s.fail(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
}

func toRepoInput(req RepositoryInput) entities.RepoInput {
	return entities.RepoInput{
		Name:          deref(req.Name),
		URL:           req.Url,
		Username:      deref(req.Username),
		Password:      req.Password,
		SSHPrivateKey: req.SshPrivateKey,
		Insecure:      deref(req.Insecure),
	}
}

// toRepository maps the repository without its secrets.
func toRepository(repo entities.Repo) Repository {
	return Repository{
		Id:               repo.ID.String(),
		Name:             repo.Name,
		Url:              repo.URL,
		Username:         repo.Username,
		Insecure:         repo.Insecure,
		HasPassword:      repo.Password != "",
		HasSshPrivateKey: repo.SSHPrivateKey != "",
		CreatedAt:        repo.CreatedAt,
		UpdatedAt:        repo.UpdatedAt,
	}
}

func deref[T any](v *T) T {
	if v == nil {
		var zero T

		return zero
	}

	return *v
}

// currentUser is the user RequireSession stored; nil on public routes.
func currentUser(ctx *echo.Context) *entities.User {
	user, _ := ctx.Get(userKey).(*entities.User)

	return user
}

// setSessionCookie sets the session token; maxAge -1 deletes the cookie.
func (s *Server) setSessionCookie(ctx *echo.Context, token string, maxAge int) {
	ctx.SetCookie(&http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) fail(ctx *echo.Context, err error) error {
	status, code, msg := http.StatusInternalServerError, "internal_error", "Something went wrong"

	switch {
	case errors.Is(err, errBadRequest):
		status, code, msg = http.StatusBadRequest, "bad_request", "Malformed request"
	case errors.Is(err, domainerrors.ErrInvalidUsername):
		status, code, msg = http.StatusBadRequest, "invalid_username",
			"Username must be 3 to 64 letters, digits, '.', '_' or '-', starting with a letter or digit"
	case errors.Is(err, domainerrors.ErrCeremonyNotFound):
		status, code, msg = http.StatusBadRequest, "ceremony_not_found", "Passkey request expired, try again"
	case errors.Is(err, domainvalues.ErrInvalidRole):
		status, code, msg = http.StatusBadRequest, "invalid_role", "Role must be admin or user"
	case errors.Is(err, domainerrors.ErrCannotManageSelf):
		status, code, msg = http.StatusBadRequest, "cannot_manage_self", "You cannot change your own account"
	case errors.Is(err, domainerrors.ErrUsernameTaken):
		status, code, msg = http.StatusConflict, "username_taken", "Username is already taken"
	case errors.Is(err, domainerrors.ErrUserDisabled):
		status, code, msg = http.StatusConflict, "user_disabled", "Approve or enable the user first"
	case errors.Is(err, domainerrors.ErrUserNotFound):
		status, code, msg = http.StatusNotFound, "user_not_found", "User not found"
	case errors.Is(err, domainerrors.ErrInvalidRepoName):
		status, code, msg = http.StatusBadRequest, "invalid_repo_name", "Name must be at most 255 characters"
	case errors.Is(err, domainerrors.ErrInvalidRepoURL):
		status, code, msg = http.StatusBadRequest, "invalid_repo_url",
			"URL must be https://host/path, ssh://[user@]host[:port]/path or user@host:path, without a password"
	case errors.Is(err, domainerrors.ErrInvalidRepoCredentials):
		status, code, msg = http.StatusBadRequest, "invalid_repo_credentials",
			"https URLs take a username and password, ssh URLs take a valid unencrypted SSH private key"
	case errors.Is(err, domainerrors.ErrRepoCredentialsRequired):
		status, code, msg = http.StatusBadRequest, "repo_credentials_required",
			"Enter the credentials again when changing the URL"
	case errors.Is(err, domainerrors.ErrRepoAccessDenied):
		status, code, msg = http.StatusBadRequest, "repo_access_denied", "Repository not found or credentials rejected"
		// Details stay in logs: they would turn the check into a probe of the server's network.
		s.log.Warn("repository access denied", logs.ErisError(err))
	case errors.Is(err, domainerrors.ErrRepoUnreachable):
		status, code, msg = http.StatusBadRequest, "repo_unreachable", "Could not connect to the repository"
		s.log.Warn("repository unreachable", logs.ErisError(err))
	case errors.Is(err, domainerrors.ErrRepoURLTaken):
		status, code, msg = http.StatusConflict, "repo_url_taken", "A repository with this URL already exists"
	case errors.Is(err, domainerrors.ErrRepoNotFound):
		status, code, msg = http.StatusNotFound, "repo_not_found", "Repository not found"
	case errors.Is(err, domainerrors.ErrUnauthenticated):
		status, code, msg = http.StatusUnauthorized, "unauthenticated", "Sign in required"
	case errors.Is(err, domainerrors.ErrForbidden):
		status, code, msg = http.StatusForbidden, "forbidden", "You do not have permission to do that"
	case errors.Is(err, domainerrors.ErrApprovalPending):
		status, code, msg = http.StatusForbidden, "approval_pending", "Account is awaiting approval or disabled"
	case errors.Is(err, domainerrors.ErrVerification):
		status, code, msg = http.StatusUnauthorized, "verification_failed", "Passkey not recognized"
		// Details stay in logs: go-webauthn messages describe what the server checked.
		s.log.Warn("passkey verification failed", logs.ErisError(err))
	default:
		s.log.Error("request failed", logs.ErisError(err))
	}

	return ctx.JSON(status, Error{Code: code, Message: msg})
}
