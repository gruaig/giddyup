package database

import (
	"fmt"
	"time"

	"giddyup/api/internal/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type DB struct {
	*sqlx.DB
}

func Connect(cfg *config.Config) (*DB, error) {
	// Add search_path to connection string
	connStr := cfg.ConnectionString() + " search_path=racing,public"

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify search_path is set
	var currentPath string
	if err := db.Get(&currentPath, "SHOW search_path"); err != nil {
		return nil, fmt.Errorf("failed to verify search_path: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) Health() error {
	return db.Ping()
}

func (db *DB) Close() error {
	return db.DB.Close()
}
