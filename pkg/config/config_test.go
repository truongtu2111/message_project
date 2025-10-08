package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear environment variables to test defaults
	envVars := []string{
		"DB_URL", "REDIS_URL", "WEBHOOK_URL",
		"BATCH_SIZE", "INTERVAL", "AUTOSTART",
		"PORT", "MAX_RETRIES", "BACKOFF_MIN", "BACKOFF_MAX", "REDIS_TTL",
	}

	// Store original values
	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}

	// Restore original values after test
	defer func() {
		for _, envVar := range envVars {
			if val, exists := originalValues[envVar]; exists && val != "" {
				os.Setenv(envVar, val)
			}
		}
	}()

	cfg := Load()

	// Test default values
	assert.Equal(t, "postgres://user:password@localhost/insider_messaging?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, "redis://localhost:6379", cfg.RedisURL)
	assert.Equal(t, "http://localhost:8081/webhook", cfg.WebhookURL)
	assert.Equal(t, 2, cfg.BatchSize)
	assert.Equal(t, 2*time.Minute, cfg.Interval)
	assert.Equal(t, false, cfg.AutoStart)
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.BackoffMin)
	assert.Equal(t, 30*time.Second, cfg.BackoffMax)
	assert.Equal(t, 24*time.Hour, cfg.RedisTTL)
}

func TestLoad_CustomValues(t *testing.T) {
	// Set custom environment variables
	testEnvVars := map[string]string{
		"DB_URL":      "postgres://custom:pass@custom-host/custom-db?sslmode=disable",
		"REDIS_URL":   "redis://custom-redis:6380",
		"WEBHOOK_URL": "https://example.com/webhook",
		"BATCH_SIZE":  "10",
		"INTERVAL":    "5m",
		"AUTOSTART":   "true",
		"PORT":        "9090",
		"MAX_RETRIES": "10",
		"BACKOFF_MIN": "2s",
		"BACKOFF_MAX": "60s",
		"REDIS_TTL":   "48h",
	}

	// Store original values
	originalValues := make(map[string]string)
	for key, value := range testEnvVars {
		originalValues[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	// Restore original values after test
	defer func() {
		for key := range testEnvVars {
			if val, exists := originalValues[key]; exists && val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	cfg := Load()

	// Test custom values
	assert.Equal(t, "postgres://custom:pass@custom-host/custom-db?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, "redis://custom-redis:6380", cfg.RedisURL)
	assert.Equal(t, "https://example.com/webhook", cfg.WebhookURL)
	assert.Equal(t, 10, cfg.BatchSize)
	assert.Equal(t, 5*time.Minute, cfg.Interval)
	assert.Equal(t, true, cfg.AutoStart)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, 10, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.BackoffMin)
	assert.Equal(t, 60*time.Second, cfg.BackoffMax)
	assert.Equal(t, 48*time.Hour, cfg.RedisTTL)
}