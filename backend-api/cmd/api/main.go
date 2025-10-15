package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"giddyup/api/internal/config"
	"giddyup/api/internal/database"
	"giddyup/api/internal/logger"
	"giddyup/api/internal/router"
	"giddyup/api/internal/services"
)

func main() {
	// Initialize file logging first
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir = "logs" // Default to logs/ directory
	}

	if err := logger.InitializeFileLogging(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logger.CloseLogFile()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	logger.Info("=== GiddyUp API Starting ===")
	logger.Info("Environment: %s", cfg.Server.Env)
	logger.Info("Server Port: %d", cfg.Server.Port)
	logger.Info("Log Level: %s", os.Getenv("LOG_LEVEL"))

	// Connect to database
	logger.Info("Connecting to database...")
	logger.Info("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Error("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("✅ Database connection established")
	logger.Info("✅ Search path set to: racing, public")

	// Initialize auto-update service
	autoUpdateEnabled := os.Getenv("AUTO_UPDATE_ON_STARTUP") == "true"
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/home/smonaghan/GiddyUp/data" // Default data directory
	}

	if autoUpdateEnabled {
		logger.Info("🔄 Auto-update service enabled")
		logger.Info("   Data directory: %s", dataDir)
		autoUpdate := services.NewAutoUpdateService(db.DB, true, dataDir)

		// Start auto-update in background (non-blocking)
		// This will find the last date in the database and backfill to yesterday
		autoUpdate.RunInBackground()
	} else {
		logger.Info("Auto-update service disabled (set AUTO_UPDATE_ON_STARTUP=true to enable)")
	}

	// Setup router
	logger.Info("Initializing router and handlers...")
	r := router.Setup(db, cfg.CORS.Origins)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server on port %d...", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	logger.Info("✅ GiddyUp API is running on http://localhost:%d", cfg.Server.Port)
	logger.Info("Health check: http://localhost:%d/health", cfg.Server.Port)
	logger.Info("API endpoints: http://localhost:%d/api/v1/*", cfg.Server.Port)
	logger.Info("=====================================")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Warn("Shutdown signal received, gracefully stopping server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown: %v", err)
	}

	logger.Info("✅ Server exited cleanly")
}
