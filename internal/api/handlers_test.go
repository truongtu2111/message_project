package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/insider/insider-messaging/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create server instance
	server := NewServer(testLogger)

	// Create a test request
	req, err := http.NewRequest("GET", "/healthz", nil)
	assert.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	server.router.ServeHTTP(w, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, w.Code)

	// Check the response body
	var response HealthResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "insider-messaging", response.Service)
	assert.Equal(t, "v0.1.0", response.Version)
}

func TestNewServer(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create server instance
	server := NewServer(testLogger)

	// Verify server is not nil
	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.logger)
}

func TestLoggerMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create a new Gin router with the middleware
	router := gin.New()
	router.Use(LoggerMiddleware(testLogger))

	// Add a test route
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create a test request
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check the status code
	assert.Equal(t, http.StatusOK, w.Code)

	// Check the response body
	var response map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "test", response["message"])
}