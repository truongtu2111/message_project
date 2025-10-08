package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	// Database configuration
	DatabaseURL string

	// Redis configuration (optional)
	RedisURL string

	// Webhook configuration
	WebhookURL string

	// Scheduler configuration
	Interval  time.Duration
	BatchSize int
	AutoStart bool

	// Server configuration
	Port string

	// Retry configuration
	MaxRetries int
	BackoffMin time.Duration
	BackoffMax time.Duration

	// Redis TTL for cached data
	RedisTTL time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DB_URL", "postgres://user:password@localhost/insider_messaging?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		WebhookURL:  getEnv("WEBHOOK_URL", "http://localhost:8081/webhook"),
		Interval:    getDurationEnv("INTERVAL", 2*time.Minute),
		BatchSize:   getIntEnv("BATCH_SIZE", 2),
		AutoStart:   getBoolEnv("AUTOSTART", false),
		Port:        getEnv("PORT", "8080"),
		MaxRetries:  getIntEnv("MAX_RETRIES", 3),
		BackoffMin:  getDurationEnv("BACKOFF_MIN", 1*time.Second),
		BackoffMax:  getDurationEnv("BACKOFF_MAX", 30*time.Second),
		RedisTTL:    getDurationEnv("REDIS_TTL", 24*time.Hour),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getDurationEnv gets a duration environment variable with a default value
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
