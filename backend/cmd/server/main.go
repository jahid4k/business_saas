// backend/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
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
	"github.com/mridha/businesssaas/internal/config"
	crmdeals "github.com/mridha/businesssaas/internal/crm/deals"
	crmleads "github.com/mridha/businesssaas/internal/crm/leads"
	crmpipeline "github.com/mridha/businesssaas/internal/crm/pipeline"
	crmreports "github.com/mridha/businesssaas/internal/crm/reports"
	"github.com/mridha/businesssaas/internal/database"
	"github.com/mridha/businesssaas/internal/middleware"
	"github.com/mridha/businesssaas/internal/organizations"
	"github.com/mridha/businesssaas/internal/platform/contacts"
	"github.com/mridha/businesssaas/internal/platform/engagement"
	"github.com/mridha/businesssaas/internal/security"
	"github.com/mridha/businesssaas/internal/task"
	"github.com/mridha/businesssaas/internal/user"
	jwtpkg "github.com/mridha/businesssaas/pkg/jwt"
	"github.com/mridha/businesssaas/pkg/response"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// 2. Logger
	setupLogger(cfg.App.IsDevelopment())
	slog.Info("starting BusinessSAAS backend",
		slog.String("env", cfg.App.Env),
		slog.String("port", cfg.App.Port),
	)

	// 3. PostgreSQL
	ctx := context.Background()
	pgPool, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", slog.Any("error", err))
		os.Exit(1)
	}
	defer pgPool.Close()

	// 4. Redis
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

	// 5. JWT + core middleware
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	requireAuth := middleware.RequireAuth(jwtManager)
	authRateLimit := middleware.NewAuthRateLimit(redisClient)

	// ----------------------------------------------------------------
	// 6. Repositories
	// ----------------------------------------------------------------
	authRepo := auth.NewRepository(pgPool)
	userRepo := user.NewRepository(pgPool)
	authzRepo := authz.NewRepository(pgPool)
	businessRepo := organizations.NewRepository(pgPool)
	auditRepo := audit.NewRepository(pgPool)
	securityRepo := security.NewRepository(pgPool)
	taskRepo := task.NewRepository(pgPool)

	// Platform
	contactsRepo := contacts.NewRepository(pgPool)
	engagementRepo := engagement.NewRepository(pgPool)

	// CRM
	pipelineRepo := crmpipeline.NewRepository(pgPool)
	dealsRepo := crmdeals.NewRepository(pgPool)
	leadsRepo := crmleads.NewRepository(pgPool)
	reportsRepo := crmreports.NewRepository(pgPool)

	// ----------------------------------------------------------------
	// 7. Services
	//
	// Dependency order matters:
	//   pipelineSvc  — no CRM deps
	//   dealsSvc     — needs pipelineSvc
	//   contactsSvc  — no CRM deps
	//   engagementSvc — no CRM deps
	//   leadsSvc     — needs contactsSvc (ContactCreator) + dealsSvc (DealCreator)
	//   reportsSvc   — needs dealsSvc + leadsSvc + engagementSvc
	// ----------------------------------------------------------------
	auditSvc := audit.NewService(auditRepo)

	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(authRepo, userRepo, jwtManager, cfg.JWT, auditSvc)
	authzSvc := authz.NewService(authzRepo, redisClient)
	businessSvc := organizations.NewService(businessRepo, authzRepo, jwtManager)
	securitySvc := security.NewService(securityRepo)
	taskSvc := task.NewService(taskRepo, auditSvc)

	// Platform
	contactsSvc := contacts.NewService(contactsRepo)
	engagementSvc := engagement.NewService(engagementRepo)

	// CRM — wire in dependency order
	pipelineSvc := crmpipeline.NewService(pipelineRepo)
	dealsSvc := crmdeals.NewService(dealsRepo, pipelineSvc)
	// leadsSvc receives contactsSvc and dealsSvc through narrow interfaces
	// (ContactCreator and DealCreator) defined in the leads package.
	// This breaks the import cycle: leads does not import deals or contacts directly.
	leadsSvc := crmleads.NewService(leadsRepo, contactsSvc, dealsSvc)
	reportsSvc := crmreports.NewService(reportsRepo, dealsSvc, leadsSvc, engagementSvc)

	// ----------------------------------------------------------------
	// 8. Handlers
	// ----------------------------------------------------------------
	authHandler := auth.NewHandler(authSvc)
	userHandler := user.NewHandler(userSvc)
	authzHandler := authz.NewHandler(authzSvc)
	businessHandler := organizations.NewHandler(businessSvc)
	securityHandler := security.NewHandler(securitySvc)
	taskHandler := task.NewHandler(taskSvc)

	// Platform
	contactsHandler := contacts.NewHandler(contactsSvc)
	// engagement handler is bound to "crm" — records are tagged module="crm".
	// When HRM arrives: engagement.NewHandler(engagementSvc, "hrm")
	engagementHandler := engagement.NewHandler(engagementSvc, "crm")

	// CRM
	pipelineHandler := crmpipeline.NewHandler(pipelineSvc)
	dealsHandler := crmdeals.NewHandler(dealsSvc)
	leadsHandler := crmleads.NewHandler(leadsSvc)
	reportsHandler := crmreports.NewHandler(reportsSvc)

	// ----------------------------------------------------------------
	// 9. Fiber
	// ----------------------------------------------------------------
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

	// ----------------------------------------------------------------
	// 10. Global middleware
	// ----------------------------------------------------------------
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

	// ----------------------------------------------------------------
	// 11. Routes
	// ----------------------------------------------------------------
	api := app.Group("/api/v1")
	registerSystemRoutes(api, app, pgPool, redisClient, cfg)

	auth.RegisterRoutesWithRateLimit(api, authHandler, requireAuth, authRateLimit)
	user.RegisterRoutes(api, userHandler, requireAuth)
	organizations.RegisterRoutes(api, businessHandler, requireAuth)

	// Shared middleware factories
	requireBusiness := middleware.RequireBusiness()
	requireOrgParam := middleware.RequireOrganizationParam("orgId")

	// permFn builds a permission-checking middleware for a named permission key.
	permFn := func(perm string) fiber.Handler {
		return middleware.RequirePermission(authzSvc, perm)
	}

	authz.RegisterRoutes(api, authzHandler, permFn, requireAuth, requireBusiness, requireOrgParam)
	security.RegisterRoutes(api, securityHandler, permFn, requireAuth, requireOrgParam)
	task.RegisterRoutes(api, taskHandler, permFn, requireAuth, requireOrgParam)

	// CRM tenant isolation guard — validates :orgId param against JWT business_id.
	// All CRM and platform routes share this middleware; it must run after requireAuth.
	requireOrgMatch := middleware.RequireOrganizationParam("orgId")

	// Platform layer (shared across all future modules)
	contacts.RegisterRoutes(api, contactsHandler, permFn, requireAuth, requireOrgMatch)
	engagement.RegisterRoutes(api, engagementHandler, permFn, requireAuth, requireOrgMatch)

	// CRM domain routes
	crmleads.RegisterRoutes(api, leadsHandler, permFn, requireAuth, requireOrgMatch)
	crmpipeline.RegisterRoutes(api, pipelineHandler, permFn, requireAuth, requireOrgMatch)
	crmdeals.RegisterRoutes(api, dealsHandler, permFn, requireAuth, requireOrgMatch)
	crmreports.RegisterRoutes(api, reportsHandler, permFn, requireAuth, requireOrgMatch)

	// 404 fallback — must be last
	app.Use(func(c fiber.Ctx) error {
		return response.NotFound(c,
			"ROUTE_NOT_FOUND",
			fmt.Sprintf("route %s %s not found", c.Method(), c.Path()),
		)
	})

	slog.Info("all routes registered")

	// ----------------------------------------------------------------
	// 12. Start + graceful shutdown
	// ----------------------------------------------------------------
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

func registerSystemRoutes(
	router fiber.Router,
	app *fiber.App,
	pgPool *pgxpool.Pool,
	redisClient *redis.Client,
	cfg *config.Config,
) {
	router.Get("/health", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

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

	router.Get("/routes", func(c fiber.Ctx) error {
		if cfg.App.IsProduction() {
			return response.NotFound(c, "ROUTE_NOT_FOUND", "not found")
		}

		type routeInfo struct {
			Method string `json:"method"`
			Path   string `json:"path"`
		}

		grouped := make(map[string][]routeInfo)
		for _, r := range app.GetRoutes(true) {
			if r.Method == fiber.MethodHead {
				continue
			}
			if r.Path == "/*" || r.Path == "" {
				continue
			}
			group := routeGroup(r.Path)
			grouped[group] = append(grouped[group], routeInfo{Method: r.Method, Path: r.Path})
		}

		type groupEntry struct {
			Group  string      `json:"group"`
			Count  int         `json:"count"`
			Routes []routeInfo `json:"routes"`
		}

		groupKeys := make([]string, 0, len(grouped))
		for k := range grouped {
			groupKeys = append(groupKeys, k)
		}
		sort.Strings(groupKeys)

		total := 0
		result := make([]groupEntry, 0, len(groupKeys))
		for _, k := range groupKeys {
			routes := grouped[k]
			sort.Slice(routes, func(i, j int) bool {
				if routes[i].Path != routes[j].Path {
					return routes[i].Path < routes[j].Path
				}
				return routes[i].Method < routes[j].Method
			})
			total += len(routes)
			result = append(result, groupEntry{Group: k, Count: len(routes), Routes: routes})
		}

		return response.OK(c, fiber.Map{"total": total, "groups": result}, "Route table")
	})
}

func routeGroup(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 {
		seg := parts[2]
		switch seg {
		case "health", "hello", "routes":
			return "system"
		default:
			return seg
		}
	}
	return "other"
}

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
			Level: slog.LevelDebug, TimeFormat: time.TimeOnly, ReplaceAttr: replaceAttr,
		})))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, ReplaceAttr: replaceAttr,
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
