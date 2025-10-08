package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/insider/insider-messaging/pkg/logger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	logger *logger.Logger
}

// NewServer creates a new HTTP server
func NewServer(log *logger.Logger) *Server {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	
	// Add middleware
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(log))

	server := &Server{
		router: router,
		logger: log.WithComponent("api"),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all the routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/healthz", s.healthCheck)

	// Swagger documentation
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Scheduler routes (to be implemented)
		scheduler := v1.Group("/scheduler")
		{
			scheduler.POST("/start", s.startScheduler)
			scheduler.POST("/stop", s.stopScheduler)
		}

		// Messages routes (to be implemented)
		messages := v1.Group("/messages")
		{
			messages.GET("/sent", s.getSentMessages)
		}
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Service string `json:"service" example:"insider-messaging"`
	Version string `json:"version" example:"v0.1.0"`
}

// healthCheck godoc
// @Summary Health check endpoint
// @Description Returns the health status of the service
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /healthz [get]
func (s *Server) healthCheck(c *gin.Context) {
	response := HealthResponse{
		Status:  "ok",
		Service: "insider-messaging",
		Version: "v0.1.0",
	}

	s.logger.Info("Health check requested")
	c.JSON(http.StatusOK, response)
}

// startScheduler godoc
// @Summary Start the message scheduler
// @Description Starts the message processing scheduler
// @Tags scheduler
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/scheduler/start [post]
func (s *Server) startScheduler(c *gin.Context) {
	// TODO: Implement scheduler start logic
	s.logger.Info("Scheduler start requested")
	c.JSON(http.StatusOK, gin.H{
		"message": "Scheduler start endpoint - not implemented yet",
		"running": false,
	})
}

// stopScheduler godoc
// @Summary Stop the message scheduler
// @Description Stops the message processing scheduler
// @Tags scheduler
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/scheduler/stop [post]
func (s *Server) stopScheduler(c *gin.Context) {
	// TODO: Implement scheduler stop logic
	s.logger.Info("Scheduler stop requested")
	c.JSON(http.StatusOK, gin.H{
		"message": "Scheduler stop endpoint - not implemented yet",
		"running": false,
	})
}

// getSentMessages godoc
// @Summary Get sent messages
// @Description Retrieves a list of sent messages with pagination
// @Tags messages
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/messages/sent [get]
func (s *Server) getSentMessages(c *gin.Context) {
	// TODO: Implement get sent messages logic
	s.logger.Info("Get sent messages requested")
	c.JSON(http.StatusOK, gin.H{
		"message": "Get sent messages endpoint - not implemented yet",
		"data":    []interface{}{},
		"total":   0,
	})
}

// Start starts the HTTP server
func (s *Server) Start(port string) error {
	s.logger.Info("Starting HTTP server", "port", port)
	return s.router.Run(":" + port)
}

// LoggerMiddleware creates a Gin middleware for structured logging
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log request details
		log.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ip", c.ClientIP(),
		)
	}
}