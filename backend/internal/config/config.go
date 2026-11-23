// Package config reads the API's settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// Config holds the API's settings. See .env.example for local values.
type Config struct {
	// DatabaseURL is the Postgres connection string. Required.
	DatabaseURL string
	// Port is the TCP port the API listens on. Defaults to 8080.
	Port string
	// FrontendOrigin is the one browser origin allowed to call the API across
	// origins with cookies, e.g. the Vite dev server at http://localhost:5173.
	// Empty means no CORS headers at all, which is right for production, where
	// Caddy serves the app and the API from the same domain.
	FrontendOrigin string
	// MigrateOnStart applies pending migrations before the API starts
	// listening. On in production; locally `make migrate` does it instead.
	MigrateOnStart bool
}

// Load reads the settings through getenv (os.Getenv in main, a map in tests).
// Errors never include DATABASE_URL, because it holds the database password.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL:    getenv("DATABASE_URL"),
		Port:           getenv("PORT"),
		FrontendOrigin: getenv("FRONTEND_ORIGIN"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is not set (see .env.example)")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if n, err := strconv.Atoi(cfg.Port); err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT must be a number from 1 to 65535, got %q", cfg.Port)
	}

	if cfg.FrontendOrigin != "" && !isOrigin(cfg.FrontendOrigin) {
		return Config{}, fmt.Errorf("FRONTEND_ORIGIN must be an http(s) origin with no path or trailing slash, like http://localhost:5173; got %q", cfg.FrontendOrigin)
	}

	if v := getenv("MIGRATE_ON_START"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("MIGRATE_ON_START must be true or false, got %q", v)
		}
		cfg.MigrateOnStart = b
	}

	return cfg, nil
}

// isOrigin reports whether s is exactly scheme://host[:port], the form a
// browser sends in the Origin header, so it can be compared as a plain string.
func isOrigin(s string) bool {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return u.Scheme+"://"+u.Host == s
}
