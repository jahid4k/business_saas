// BusinessSAAS Backend — Entry Point
// cmd/server/main.go
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
	"github.com/mridha/businesssaas/internal/business"
	"github.com/mridha/businesssaas/internal/config"
	"github.com/mridha/businesssaas/internal/database"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	"github.com/mridha/businesssaas/pkg/response"
)

func main() {
	// ----------------------------------------------------------
	// 1. Configuration (must come first — logger depends on env)
	// ----------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		// Pre-config fallback: plain slog before tint is ready
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// ----------------------------------------------------------
	// 2. Logger (set up immediately after config)
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

	slog.Info("PostgreSQL connected")

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

	slog.Info("Redis connected")

	// ----------------------------------------------------------
	// 5. Repositories
	// ----------------------------------------------------------
	authRepo := auth.NewRepository()
	userRepo := user.NewRepository()
	businessRepo := business.NewRepository()
	auditRepo := audit.NewNoopRepository()
	taskRepo := task.NewRepository()

	// ----------------------------------------------------------
	// 6. Services
	// ----------------------------------------------------------
	auditSvc := audit.NewService(auditRepo)
	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(authRepo)
	businessSvc := business.NewService(businessRepo)
	taskSvc := task.NewService(taskRepo)

	_ = auditSvc // used in Phase 1-B

	// ----------------------------------------------------------
	// 7. Handlers
	// ----------------------------------------------------------
	authHandler := auth.NewHandler(authSvc)
	userHandler := user.NewHandler(userSvc)
	businessHandler := business.NewHandler(businessSvc)
	taskHandler := task.NewHandler(taskSvc)

	// ----------------------------------------------------------
	// 8. Fiber app
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

		ServerHeader:  "", // never leak server software
		StrictRouting: true,
		CaseSensitive: true,
	})

	// ----------------------------------------------------------
	// 9. Global middleware
	// ----------------------------------------------------------
	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())

	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.CORS.AllowedOrigins[0], "*"},
		AllowMethods:     []string{"GET,POST,PATCH,DELETE,OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// ----------------------------------------------------------
	// 10. Routes
	// ----------------------------------------------------------
	api := app.Group("/api/v1")

	registerSystemRoutes(api, pgPool, redisClient)

	auth.RegisterRoutes(api, authHandler)
	user.RegisterRoutes(api, userHandler)
	business.RegisterRoutes(api, businessHandler)
	task.RegisterRoutes(api, taskHandler)
	registerMemberRoutes(api)

	// 404 fallback — must be the very last handler
	app.Use(func(c fiber.Ctx) error {
		return response.NotFound(c,
			"ROUTE_NOT_FOUND",
			fmt.Sprintf("route %s %s not found", c.Method(), c.Path()),
		)
	})

	slog.Info("all routes registered")

	// ----------------------------------------------------------
	// 11. Graceful startup + shutdown
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
	slog.Info("shutdown signal received, draining requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("server stopped cleanly")
}

// setupLogger configures the global slog logger.
//
// Development → tint colored handler, DEBUG level, human-readable timestamps.
// Production  → JSON handler, INFO level, machine-readable for log aggregators.
//
// Both environments apply a ReplaceAttr that redacts sensitive keys so that
// accidentally logging a token or password is never exposed in any environment.
func setupLogger(isDev bool) {
	// Sensitive keys that must never appear in logs as plain values.
	sensitiveKeys := map[string]bool{
		"password":      true,
		"token":         true,
		"secret":        true,
		"refresh_token": true,
		"access_token":  true,
		"reset_token":   true,
		"api_key":       true,
	}

	replaceAttr := func(_ []string, a slog.Attr) slog.Attr {
		if sensitiveKeys[a.Key] {
			return slog.String(a.Key, "[REDACTED]")
		}
		return a
	}

	if isDev {
		slog.SetDefault(slog.New(
			tint.NewHandler(os.Stdout, &tint.Options{
				Level:       slog.LevelDebug,
				TimeFormat:  time.TimeOnly, // "15:04:05" — clean for dev terminal
				AddSource:   false,         // flip to true to see file:line in logs
				ReplaceAttr: replaceAttr,
			}),
		))
		return
	}

	// Production: structured JSON, INFO and above only.
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: replaceAttr,
		}),
	))
}

// registerSystemRoutes wires /api/v1/health and /api/v1/hello.
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

// registerMemberRoutes wires member/role/permission management routes.
// Full implementation comes in Phase 1-B.
func registerMemberRoutes(router fiber.Router) {
	members := router.Group("/members",
		middleware.RequireAuth(),
		middleware.RequireBusiness(),
	)
	members.Get("/", func(c fiber.Ctx) error { return response.NotImplemented(c) })
	members.Post("/:userId/role", func(c fiber.Ctx) error { return response.NotImplemented(c) })

	roles := router.Group("/roles", middleware.RequireAuth())
	roles.Get("/", func(c fiber.Ctx) error { return response.NotImplemented(c) })

	perms := router.Group("/permissions", middleware.RequireAuth())
	perms.Get("/", func(c fiber.Ctx) error { return response.NotImplemented(c) })
}

// healthStatus returns a human-readable status string.
func healthStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "degraded"
}

// healthMessage returns a human-readable health message.
func healthMessage(ok bool) string {
	if ok {
		return "Service healthy"
	}
	return "Service degraded"
}
