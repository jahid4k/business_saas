// Package config loads and validates all application configuration
// from environment variables. It is the single source of truth for
// every configurable value in the backend.
//
// Usage:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration values.
// Values are loaded once at startup and treated as immutable.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	CORS     CORSConfig
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name string
	Env  string // "development" | "staging" | "production"
	Port string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// JWTConfig holds token signing settings.
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
}

// Load reads environment variables (and optionally a .env file)
// and returns a validated Config.
//
// In Docker Compose, all env vars are injected directly.
// For local development outside Docker, a .env file is used.
// The .env file is silently ignored if it does not exist.
func Load() (*Config, error) {
	// Attempt to load .env — silently ignore if absent (Docker sets vars directly)
	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file found, using environment variables directly")
	}

	cfg := &Config{}

	// ----------------------------------------------------------
	// App
	// ----------------------------------------------------------
	cfg.App = AppConfig{
		Name: getEnv("APP_NAME", "BusinessSAAS"),
		Env:  getEnv("APP_ENV", "development"),
		Port: getEnv("APP_PORT", "8080"),
	}

	// ----------------------------------------------------------
	// Database
	// ----------------------------------------------------------
	maxOpenConns, err := getEnvInt("DATABASE_MAX_OPEN_CONNS", 25)
	if err != nil {
		return nil, fmt.Errorf("config: DATABASE_MAX_OPEN_CONNS: %w", err)
	}

	maxIdleConns, err := getEnvInt("DATABASE_MAX_IDLE_CONNS", 5)
	if err != nil {
		return nil, fmt.Errorf("config: DATABASE_MAX_IDLE_CONNS: %w", err)
	}

	connMaxLifetimeSecs, err := getEnvInt("DATABASE_CONN_MAX_LIFETIME", 300)
	if err != nil {
		return nil, fmt.Errorf("config: DATABASE_CONN_MAX_LIFETIME: %w", err)
	}

	cfg.Database = DatabaseConfig{
		Host:            getEnv("DATABASE_HOST", "localhost"),
		Port:            getEnv("DATABASE_PORT", "5432"),
		User:            getEnvRequired("DATABASE_USER"),
		Password:        getEnvRequired("DATABASE_PASSWORD"),
		Name:            getEnvRequired("DATABASE_NAME"),
		SSLMode:         getEnv("DATABASE_SSLMODE", "disable"),
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: time.Duration(connMaxLifetimeSecs) * time.Second,
	}

	// ----------------------------------------------------------
	// Redis
	// ----------------------------------------------------------
	redisDB, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return nil, fmt.Errorf("config: REDIS_DB: %w", err)
	}

	cfg.Redis = RedisConfig{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnv("REDIS_PORT", "6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       redisDB,
	}

	// ----------------------------------------------------------
	// JWT
	// ----------------------------------------------------------
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET is required")
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("config: JWT_SECRET must be at least 32 characters")
	}

	accessTTL, err := getEnvDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("config: JWT_ACCESS_TOKEN_TTL: %w", err)
	}

	refreshTTL, err := getEnvDuration("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("config: JWT_REFRESH_TOKEN_TTL: %w", err)
	}

	cfg.JWT = JWTConfig{
		Secret:          jwtSecret,
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
	}

	// ----------------------------------------------------------
	// CORS
	// ----------------------------------------------------------
	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	origins := splitAndTrim(originsRaw, ",")

	cfg.CORS = CORSConfig{
		AllowedOrigins: origins,
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string for pgx.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// Addr returns the Redis address in "host:port" format.
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// IsDevelopment returns true when running in development mode.
func (c *AppConfig) IsDevelopment() bool {
	return c.Env == "development"
}

// IsProduction returns true when running in production mode.
func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// ----------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------

// getEnv returns the value of the environment variable named by key,
// or the fallback if the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvRequired returns the value of a required environment variable.
// If missing or empty, it panics with a clear message rather than returning
// an error, because missing required config is always a fatal startup error.
func getEnvRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		// We panic here intentionally — missing required config is a
		// programming/deployment error, not a runtime error.
		panic(fmt.Sprintf("config: required environment variable %q is not set", key))
	}
	return v
}

// getEnvInt returns the integer value of an environment variable,
// or the fallback if unset or empty.
func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("must be an integer, got %q", v)
	}
	return n, nil
}

// getEnvDuration parses a duration string (e.g. "15m", "7d") from an
// environment variable. It supports the "d" suffix for days in addition
// to the standard Go duration formats.
func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}

	// Handle "d" suffix for days (not supported natively by time.ParseDuration)
	if strings.HasSuffix(v, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(v, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", v, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", v, err)
	}
	return d, nil
}

// splitAndTrim splits a string by sep and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
