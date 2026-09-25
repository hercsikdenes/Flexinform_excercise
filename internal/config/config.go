package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDatabaseDSN   = "postgres://car_service:car_service_dev_password@db:5432/car_service?sslmode=disable"
	defaultServerPort    = "8080"
	defaultDataDir       = "data"
	defaultMigrationsDir = "migrations"
	defaultShutdown      = 10 * time.Second
)

type Config struct {
	DatabaseDSN     string
	ServerPort      string
	DataDir         string
	MigrationsDir   string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseDSN:   valueOrDefault("DATABASE_DSN", defaultDatabaseDSN),
		ServerPort:    valueOrDefault("SERVER_PORT", defaultServerPort),
		DataDir:       valueOrDefault("DATA_DIR", defaultDataDir),
		MigrationsDir: valueOrDefault("MIGRATIONS_DIR", defaultMigrationsDir),
	}

	port, err := strconv.Atoi(cfg.ServerPort)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("SERVER_PORT must be an integer between 1 and 65535")
	}

	cfg.ShutdownTimeout = defaultShutdown
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		timeout, parseErr := time.ParseDuration(raw)
		if parseErr != nil || timeout <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration")
		}
		cfg.ShutdownTimeout = timeout
	}

	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
