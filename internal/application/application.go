package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"carservice/internal/config"
	"carservice/internal/database"
	httpapi "carservice/internal/http"
	"carservice/internal/repository"
	"carservice/internal/seed"
	"carservice/internal/service"
)

const (
	startupTimeout    = 30 * time.Second
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func Run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), startupTimeout)
	defer startupCancel()

	db, err := database.Open(startupCtx, cfg.DatabaseDSN)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer closeDatabase(db, logger)

	if err := migrateDatabase(startupCtx, db, cfg); err != nil {
		return err
	}

	server := initServer(cfg, db, logger)

	err = serveUntilShutdown(server, cfg.ShutdownTimeout, logger)
	return err
}

func migrateDatabase(ctx context.Context, db *database.Database, cfg config.Config) error {
	if err := database.Migrate(ctx, db.SQL, cfg.MigrationsDir); err != nil {
		return err
	}
	if err := seed.New(db.GORM, cfg.DataDir).Ensure(ctx); err != nil {
		return fmt.Errorf("ensure initial seed data: %w", err)
	}
	return nil
}

func initServer(cfg config.Config, db *database.Database, logger *log.Logger) *http.Server {
	clientRepository := repository.NewGORMClientRepository(db.GORM)
	clientService := service.NewClientService(clientRepository)
	router := httpapi.NewRouter(clientService, db, logger)

	return &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func serveUntilShutdown(server *http.Server, shutdownTimeout time.Duration, logger *log.Logger) error {
	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("HTTP server listening on %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	select {
	case serveErr := <-serverErrors:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", serveErr)
	case <-signalCtx.Done():
		logger.Printf("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful HTTP shutdown: %w", err)
	}
	logger.Printf("HTTP server stopped cleanly")
	return nil
}

func closeDatabase(db *database.Database, logger *log.Logger) {
	if err := db.Close(); err != nil {
		logger.Printf("database close failed: %v", err)
	}
}
