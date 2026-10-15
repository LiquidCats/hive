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
	log        kernelports.Logger[any]
	sessionTTL time.Duration
}

func NewServer(auth ports.Auth, users ports.Users, log kernelports.Logger[any], sessionTTL time.Duration) *Server {
	return &Server{auth: auth, users: users, log: log, sessionTTL: sessionTTL}
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

	return ctx.JSON(http.StatusOK, SessionUser{Id: user.ID.String(), Username: user.Name.String()})
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
func (s *Server) ListUsers(ctx *echo.Context) error {
	users, err := s.users.ListUsers(ctx.Request().Context())
	if err != nil {
		return s.fail(ctx, err)
	}

	resp := make([]User, len(users))
	for i, user := range users {
		resp[i] = User{
			Id:        user.ID.String(),
			Username:  user.Name.String(),
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

	if err = s.users.EnableUser(ctx.Request().Context(), userID); err != nil {
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

	if err = s.users.DisableUser(ctx.Request().Context(), currentUser(ctx).ID, userID); err != nil {
		return s.fail(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
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
	case errors.Is(err, domainerrors.ErrCannotDisableSelf):
		status, code, msg = http.StatusBadRequest, "cannot_disable_self", "You cannot disable your own account"
	case errors.Is(err, domainerrors.ErrUsernameTaken):
		status, code, msg = http.StatusConflict, "username_taken", "Username is already taken"
	case errors.Is(err, domainerrors.ErrUserNotFound):
		status, code, msg = http.StatusNotFound, "user_not_found", "User not found"
	case errors.Is(err, domainerrors.ErrUnauthenticated):
		status, code, msg = http.StatusUnauthorized, "unauthenticated", "Sign in required"
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
