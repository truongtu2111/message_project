package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/insider/insider-messaging/internal/api"
	"github.com/insider/insider-messaging/internal/db"
	"github.com/insider/insider-messaging/pkg/config"
	"github.com/insider/insider-messaging/pkg/logger"
)

// @title Insider Messaging API
// @version 1.0
// @description A messaging service API for sending messages via webhooks
// @host localhost:8080
// @BasePath /
func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log := logger.New().WithComponent("main")

	log.Info("Starting Insider Messaging Service", "version", "v0.1.0")

	// Initialize database connection (optional for development)
	var database *db.DB
	if cfg.DatabaseURL != "" {
		var err error
		database, err = db.New(cfg.DatabaseURL)
		if err != nil {
			log.Warn("Failed to connect to database, running without database", "error", err)
		} else {
			defer database.Close()
			log.Info("Connected to database successfully")

			// Run database migrations
			if err := database.RunMigrations(); err != nil {
				log.Error("Failed to run database migrations", "error", err)
				os.Exit(1)
			}
			log.Info("Database migrations completed successfully")
		}
	} else {
		log.Info("No database URL configured, running without database")
	}

	// Create HTTP server
	server := api.NewServer(log)

	// Create HTTP server instance
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: server,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("Server exited")
}