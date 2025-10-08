package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/insider/insider-messaging/docs" // Import docs for swagger
	"github.com/insider/insider-messaging/internal/api"
	"github.com/insider/insider-messaging/internal/db"
	"github.com/insider/insider-messaging/internal/repo"
	"github.com/insider/insider-messaging/internal/scheduler"
	"github.com/insider/insider-messaging/internal/service"
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

	// Initialize message repository and service
	var messageRepo repo.MessageRepository
	var messageService service.MessageService

	if database != nil {
		log.Info("Using PostgreSQL database")
		messageRepo = repo.NewMessageRepository(database.DB)
		
		// Try to initialize Redis cache
		redisCache, err := repo.NewRedisCacheRepository(cfg.RedisURL, cfg.RedisTTL)
		if err != nil {
			log.Warn("Failed to connect to Redis, proceeding without cache", "error", err)
			messageService = service.NewMessageService(messageRepo, log.Logger)
		} else {
			log.Info("Redis cache initialized successfully")
			messageService = service.NewMessageServiceWithCache(messageRepo, redisCache, log.Logger)
		}
	} else {
		// Use in-memory repository for development
		log.Info("Using in-memory repository for development")
		messageRepo = repo.NewInMemoryMessageRepository()
		messageService = service.NewMessageService(messageRepo, log.Logger)
	}

	// Initialize scheduler with adapter
	schedulerAdapter := service.NewSchedulerAdapter(messageService)
	schedulerConfig := scheduler.DefaultConfig()
	messageScheduler := scheduler.NewScheduler(schedulerAdapter, log, schedulerConfig)

	// Create HTTP server
	server := api.NewServer(log, messageService, messageScheduler)

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