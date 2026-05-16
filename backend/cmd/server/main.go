// backend/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"

	"github.com/redis/go-redis/v9"

	"github.com/mridha/businesssaas/internal/audit"
	"github.com/mridha/businesssaas/internal/auth"
	"github.com/mridha/businesssaas/internal/authz"
	"github.com/mridha/businesssaas/internal/business"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/database"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/response"
)

func main() {
	// ----------------------------------------------------------
	// 1. Config
	// ----------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// ----------------------------------------------------------
	// 2. Logger
	// ----------------------------------------------------------
	setupLogger(cfg.App.IsDevelopment())
	slog.Info("starting BusinessSAAS backend",
		slog.String("env", cfg.App.Env),
		slog.String("port", cfg.App.Port),
	)

	// ----------------------------------------------------------
	// 3. PostgreSQL
	// ----------------------------------------------------------
	ctx := context.Background()

	pgPool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", slog.Any("error", err))
		os.Exit(1)
	}
	defer pgPool.Close()

	// ----------------------------------------------------------
	// 4. Redis
	// ----------------------------------------------------------
	redisClient, err := database.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		slog.Error("failed to connect to Redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			slog.Warn("redis close error", slog.Any("error", closeErr))
		}
	}()

	// ----------------------------------------------------------
	// 5. JWT + middleware — built once, injected everywhere
	// ----------------------------------------------------------
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	requireAuth := middleware.RequireAuth(jwtManager)
	authRateLimit := middleware.NewAuthRateLimit(redisClient)

	// ----------------------------------------------------------
	// 6. Repositories — all receive pgPool
	// ----------------------------------------------------------
	authRepo := auth.NewRepository(pgPool)
	userRepo := user.NewRepository(pgPool)
	authzRepo := authz.NewRepository(pgPool)
	businessRepo := business.NewRepository(pgPool)
	auditRepo := audit.NewNoopRepository()
	taskRepo := task.NewRepository(pgPool)

	// ----------------------------------------------------------
	// 7. Services
	// ----------------------------------------------------------
	_ = audit.NewService(auditRepo) // wired in audit domain

	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(authRepo, userRepo, jwtManager, cfg.JWT)
	authzSvc := authz.NewService(authzRepo, redisClient)
	businessSvc := business.NewService(businessRepo, authzRepo, jwtManager)
	taskSvc := task.NewService(taskRepo)

	// ----------------------------------------------------------
	// 8. Handlers
	// ----------------------------------------------------------
	authHandler := auth.NewHandler(authSvc)
	userHandler := user.NewHandler(userSvc)
	authzHandler := authz.NewHandler(authzSvc)
	businessHandler := business.NewHandler(businessSvc)
	taskHandler := task.NewHandler(taskSvc)

	// ----------------------------------------------------------
	// 9. Fiber
	// ----------------------------------------------------------
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			slog.Error("unhandled error",
				slog.Any("error", err),
				slog.String("path", c.Path()),
				slog.String("method", c.Method()),
			)
			return response.InternalServerError(c)
		},
		ServerHeader:  "",
		StrictRouting: true,
		CaseSensitive: true,
	})

	// ----------------------------------------------------------
	// 10. Global middleware
	// ----------------------------------------------------------
	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// ----------------------------------------------------------
	// 11. Routes
	// ----------------------------------------------------------
	api := app.Group("/api/v1")

	registerSystemRoutes(api, pgPool, redisClient)

	auth.RegisterRoutesWithRateLimit(api, authHandler, requireAuth, authRateLimit)
	user.RegisterRoutes(api, userHandler, requireAuth)
	business.RegisterRoutes(api, businessHandler, requireAuth)
	authz.RegisterRoutes(api, authzHandler, requireAuth, authzSvc)
	task.RegisterRoutes(api, taskHandler, requireAuth, authzSvc)

	// 404 fallback — must be last
	app.Use(func(c fiber.Ctx) error {
		return response.NotFound(c,
			"ROUTE_NOT_FOUND",
			fmt.Sprintf("route %s %s not found", c.Method(), c.Path()),
		)
	})

	slog.Info("all routes registered")

	// ----------------------------------------------------------
	// 12. Start + graceful shutdown
	// ----------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.App.Port
		slog.Info("server listening", slog.String("addr", addr))
		if listenErr := app.Listen(addr, fiber.ListenConfig{
			DisableStartupMessage: !cfg.App.IsDevelopment(),
		}); listenErr != nil {
			slog.Error("server error", slog.Any("error", listenErr))
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}

// ----------------------------------------------------------
// System routes
// ----------------------------------------------------------

func registerSystemRoutes(router fiber.Router, pgPool *pgxpool.Pool, redisClient *redis.Client) {
	router.Get("/health", func(c fiber.Ctx) error {
		ctx := context.Background()

		pgStatus := "ok"
		if err := database.Ping(ctx, pgPool); err != nil {
			slog.Error("health: postgres ping failed", slog.Any("error", err))
			pgStatus = "unreachable"
		}

		redisStatus := "ok"
		if err := database.PingRedis(ctx, redisClient); err != nil {
			slog.Error("health: redis ping failed", slog.Any("error", err))
			redisStatus = "unreachable"
		}

		healthy := pgStatus == "ok" && redisStatus == "ok"
		httpStatus := fiber.StatusOK
		if !healthy {
			httpStatus = fiber.StatusServiceUnavailable
		}

		return c.Status(httpStatus).JSON(fiber.Map{
			"success": healthy,
			"data": fiber.Map{
				"status":   healthStatus(healthy),
				"postgres": pgStatus,
				"redis":    redisStatus,
			},
			"message":    healthMessage(healthy),
			"request_id": c.Locals("request_id"),
		})
	})

	router.Get("/hello", func(c fiber.Ctx) error {
		return response.OK(c, nil, "Hello from Go backend")
	})
}

// ----------------------------------------------------------
// Logger
// ----------------------------------------------------------

func setupLogger(isDev bool) {
	sensitiveKeys := map[string]bool{
		"password": true, "token": true, "secret": true,
		"refresh_token": true, "access_token": true,
		"reset_token": true, "api_key": true,
	}
	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		if sensitiveKeys[a.Key] {
			return slog.String(a.Key, "[REDACTED]")
		}
		return a
	}

	if isDev {
		slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			Level:       slog.LevelDebug,
			TimeFormat:  time.TimeOnly,
			ReplaceAttr: replaceAttr,
		})))
		return
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceAttr,
	})))
}

func healthStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}

func healthMessage(ok bool) string {
	if ok {
		return "Service healthy"
	}
	return "Service degraded"
}
