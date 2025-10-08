package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	logger := New()
	assert.NotNil(t, logger)
	assert.NotNil(t, logger.Logger)
}

func TestNewWithLevel(t *testing.T) {
	logger := NewWithLevel(slog.LevelDebug)
	assert.NotNil(t, logger)
	assert.NotNil(t, logger.Logger)
}

func TestWithComponent(t *testing.T) {
	logger := New()
	componentLogger := logger.WithComponent("test-component")
	
	assert.NotNil(t, componentLogger)
	assert.NotEqual(t, logger, componentLogger) // Should return a new instance
}

func TestWithRequestID(t *testing.T) {
	logger := New()
	requestLogger := logger.WithRequestID("test-request-123")
	
	assert.NotNil(t, requestLogger)
	assert.NotEqual(t, logger, requestLogger) // Should return a new instance
}

func TestLoggerMethods(t *testing.T) {
	logger := New()
	
	// Test that methods don't panic
	assert.NotPanics(t, func() {
		logger.Info("test info message", "key", "value")
	})
	
	assert.NotPanics(t, func() {
		logger.Error("test error message", "error", "test error")
	})
	
	assert.NotPanics(t, func() {
		logger.Debug("test debug message", "debug", true)
	})
	
	assert.NotPanics(t, func() {
		logger.Warn("test warn message", "warning", "test warning")
	})
}

func TestChainedMethods(t *testing.T) {
	logger := New()
	
	// Test chaining methods
	chainedLogger := logger.WithComponent("api").WithRequestID("req-123")
	assert.NotNil(t, chainedLogger)
	
	// Should not panic when logging
	assert.NotPanics(t, func() {
		chainedLogger.Info("chained logger test", "test", true)
	})
}