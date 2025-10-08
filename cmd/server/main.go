// Package main Insider Messaging Service
// @title Insider Messaging API
// @version 1.0
// @description A messaging service that processes and sends messages through webhooks with scheduling capabilities.
// @host localhost:8080
// @BasePath /
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/insider/insider-messaging/internal/api"
	"github.com/insider/insider-messaging/pkg/config"
	"github.com/insider/insider-messaging/pkg/logger"
	_ "github.com/insider/insider-messaging/docs" // Import for swagger docs
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	appLogger := logger.New().WithComponent("main")

	appLogger.Info("Starting Insider Messaging Service",
		"port", cfg.Port,
		"batch_size", cfg.BatchSize,
		"interval", cfg.Interval,
		"auto_start", cfg.AutoStart,
	)

	// Initialize HTTP server
	server := api.NewServer(appLogger)

	// Start server in a goroutine
	go func() {
		appLogger.Info("HTTP server starting", "port", cfg.Port)
		if err := server.Start(cfg.Port); err != nil && err != http.ErrServerClosed {
			appLogger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// TODO: Implement proper graceful shutdown when we have the server instance
	_ = ctx

	appLogger.Info("Server shutdown complete")
}