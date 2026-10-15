package bff

import (
	"context"
	"net/http"
	"slices"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/LiquidCats/hive/internal/kernel/ports"
	"github.com/LiquidCats/hive/internal/modules/bff/configs"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/inbound/server"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite"
	queries "github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/db"
	"github.com/LiquidCats/hive/internal/modules/bff/internal/adapter/outbound/sqlite/migrations"
	app "github.com/LiquidCats/hive/internal/modules/bff/internal/core/app/usecase"
	"github.com/LiquidCats/hive/pkg/clock"
	"github.com/LiquidCats/hive/pkg/logs"
	db "github.com/LiquidCats/libraries/db/v2/sqlite"
	"github.com/LiquidCats/libraries/graceful"
)

const (
	AppName = "bff"

	maxBodyBytes = 64 << 10 // WebAuthn payloads are a few KB
	ratePerIP    = 10       // requests per second
)

func Run(ctx context.Context, logger ports.Logger[any], cfg configs.Config) {
	wn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.Auth.RPDisplayName,
		RPID:          cfg.Auth.RPID,
		RPOrigins:     slices.Clone(cfg.Auth.RPOrigins),
		// Stored ceremonies carry an expiry; reject responses that arrive after it.
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true},
			Registration: webauthn.TimeoutConfig{Enforce: true},
		},
	})
	if err != nil {
		logger.Error("failed to create webauthn", logs.ErisError(err))

		return
	}

	conn, err := db.Connect(ctx, db.WithDatabase(cfg.DB.Path))
	if err != nil {
		logger.Error("failed to connect to database", logs.ErisError(err))

		return
	}
	defer conn.Close()

	logger.Info("database connection established")

	err = db.MigrateUp(ctx, conn, migrations.Migrations)
	if err != nil {
		logger.Error("failed to migrate database", logs.ErisError(err))

		return
	}
	logger.Info("database migrated successfully")

	queries := queries.New(conn)
	txManager := db.NewTxManager(conn)
	queriesTxManager := db.NewQueriesTxManager(txManager, queries)

	repo := sqlite.NewRepository(queriesTxManager)

	auth := app.NewAuth(wn, repo, repo, repo, repo, clock.New(), cfg.Auth.SessionTTL)
	srv := server.NewServer(auth, app.NewUsers(repo), logger, cfg.Auth.SessionTTL)

	// Rejects cross-origin non-GET browser requests (Sec-Fetch-Site / Origin); SameSite alone lets same-site
	// origins such as sibling subdomains through. The web app origins are trusted for a cross-origin API URL.
	csrf := http.NewCrossOriginProtection()
	for _, origin := range cfg.Auth.RPOrigins {
		if err = csrf.AddTrustedOrigin(origin); err != nil {
			logger.Error("failed to trust origin", logs.ErisError(err))

			return
		}
	}

	router := echo.New()
	// Limit first so unauthenticated floods (the public start endpoints write a row each) are throttled too.
	// ponytail: in-memory, per process, keyed by RemoteAddr; behind a reverse proxy set router.IPExtractor
	// to a trusted one (e.g. echo.ExtractIPFromXFFHeader), or every client shares the proxy's bucket.
	router.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(ratePerIP)), srv.RequireSession)

	server.RegisterHandlers(router, srv)

	runners := []graceful.Runner{
		graceful.Signals,
		graceful.Server(
			csrf.Handler(http.MaxBytesHandler(router, maxBodyBytes)),
			graceful.WithPort(cfg.Http.Port),
			graceful.WithReadTimeout(cfg.Http.ReadTimeout),
			graceful.WithWriteTimeout(cfg.Http.WriteTimeout),
		),
	}

	logger.Info("application starting")
	defer logger.Info("application stopped")

	if err = graceful.WaitContext(ctx, runners...); err != nil {
		logger.Error("failed to wait for signals", logs.ErisError(err))

		return
	}
}
