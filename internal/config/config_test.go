package config

import (
	"strings"
	"testing"
	"time"
)

var configEnvironmentKeys = []string{
	"DATABASE_DSN",
	"SERVER_PORT",
	"DATA_DIR",
	"MIGRATIONS_DIR",
	"SHUTDOWN_TIMEOUT",
}

func TestLoadDefaults(testContext *testing.T) {
	clearConfigEnvironment(testContext)

	configuration, err := Load()
	if err != nil {
		testContext.Fatalf("Load returned an error: %v", err)
	}

	if configuration.DatabaseDSN != defaultDatabaseDSN {
		testContext.Errorf("DatabaseDSN = %q, want %q", configuration.DatabaseDSN, defaultDatabaseDSN)
	}
	if configuration.ServerPort != defaultServerPort {
		testContext.Errorf("ServerPort = %q, want %q", configuration.ServerPort, defaultServerPort)
	}
	if configuration.DataDir != defaultDataDir {
		testContext.Errorf("DataDir = %q, want %q", configuration.DataDir, defaultDataDir)
	}
	if configuration.MigrationsDir != defaultMigrationsDir {
		testContext.Errorf("MigrationsDir = %q, want %q", configuration.MigrationsDir, defaultMigrationsDir)
	}
	if configuration.ShutdownTimeout != defaultShutdown {
		testContext.Errorf("ShutdownTimeout = %s, want %s", configuration.ShutdownTimeout, defaultShutdown)
	}
}

func TestLoadOverridesAndTrimsValues(testContext *testing.T) {
	clearConfigEnvironment(testContext)
	testContext.Setenv("DATABASE_DSN", "  postgres://example/test  ")
	testContext.Setenv("SERVER_PORT", " 42069 ")
	testContext.Setenv("DATA_DIR", " /custom/data ")
	testContext.Setenv("MIGRATIONS_DIR", " /custom/migrations ")
	testContext.Setenv("SHUTDOWN_TIMEOUT", " 1m30s ")

	configuration, err := Load()
	if err != nil {
		testContext.Fatalf("Load returned an error: %v", err)
	}

	if configuration.DatabaseDSN != "postgres://example/test" {
		testContext.Errorf("unexpected DatabaseDSN: %q", configuration.DatabaseDSN)
	}
	if configuration.ServerPort != "42069" {
		testContext.Errorf("unexpected ServerPort: %q", configuration.ServerPort)
	}
	if configuration.DataDir != "/custom/data" {
		testContext.Errorf("unexpected DataDir: %q", configuration.DataDir)
	}
	if configuration.MigrationsDir != "/custom/migrations" {
		testContext.Errorf("unexpected MigrationsDir: %q", configuration.MigrationsDir)
	}
	if configuration.ShutdownTimeout != 90*time.Second {
		testContext.Errorf("ShutdownTimeout = %s, want 1m30s", configuration.ShutdownTimeout)
	}
}

func TestLoadServerPortValidation(testContext *testing.T) {
	for _, port := range []string{"1", "65535"} {
		testContext.Run("valid_"+port, func(testContext *testing.T) {
			clearConfigEnvironment(testContext)
			testContext.Setenv("SERVER_PORT", port)

			configuration, err := Load()
			if err != nil {
				testContext.Fatalf("Load returned an error for port %s: %v", port, err)
			}
			if configuration.ServerPort != port {
				testContext.Errorf("ServerPort = %q, want %q", configuration.ServerPort, port)
			}
		})
	}

	for _, port := range []string{"invalid", "0", "-1", "65536", "1.5"} {
		testContext.Run("invalid_"+port, func(testContext *testing.T) {
			clearConfigEnvironment(testContext)
			testContext.Setenv("SERVER_PORT", port)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "SERVER_PORT") {
				testContext.Fatalf("Load error = %v, want SERVER_PORT validation error", err)
			}
		})
	}
}

func TestLoadShutdownTimeoutValidation(testContext *testing.T) {
	for _, timeout := range []string{"1ns", "250ms", "2m"} {
		testContext.Run("valid_"+timeout, func(testContext *testing.T) {
			clearConfigEnvironment(testContext)
			testContext.Setenv("SHUTDOWN_TIMEOUT", timeout)

			configuration, err := Load()
			if err != nil {
				testContext.Fatalf("Load returned an error for timeout %s: %v", timeout, err)
			}
			want, parseErr := time.ParseDuration(timeout)
			if parseErr != nil {
				testContext.Fatalf("invalid test duration %q: %v", timeout, parseErr)
			}
			if configuration.ShutdownTimeout != want {
				testContext.Errorf("ShutdownTimeout = %s, want %s", configuration.ShutdownTimeout, want)
			}
		})
	}

	for _, timeout := range []string{"invalid", "0s", "-1s"} {
		testContext.Run("invalid_"+timeout, func(testContext *testing.T) {
			clearConfigEnvironment(testContext)
			testContext.Setenv("SHUTDOWN_TIMEOUT", timeout)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
				testContext.Fatalf("Load error = %v, want SHUTDOWN_TIMEOUT validation error", err)
			}
		})
	}
}

func clearConfigEnvironment(testContext *testing.T) {
	testContext.Helper()
	for _, key := range configEnvironmentKeys {
		testContext.Setenv(key, "")
	}
}
