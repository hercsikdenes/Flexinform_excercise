package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	GORM *gorm.DB
	SQL  *sql.DB
}

func Open(ctx context.Context, dsn string) (*Database, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("obtain SQL database: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Database{GORM: db, SQL: sqlDB}, nil
}

func (database *Database) Ping(ctx context.Context) error {
	if err := database.SQL.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func (database *Database) Close() error {
	if err := database.SQL.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

func Migrate(ctx context.Context, db *sql.DB, directory string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, directory); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}
	return nil
}
