package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/insider/insider-messaging/internal/domain"
	"github.com/insider/insider-messaging/internal/scheduler"
	"github.com/insider/insider-messaging/internal/service"
	"github.com/insider/insider-messaging/pkg/logger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server represents the HTTP server
type Server struct {
	router         *gin.Engine
	logger         *logger.Logger
	messageService service.MessageService
	scheduler      *scheduler.Scheduler
}

// NewServer creates a new HTTP server
func NewServer(log *logger.Logger, messageService service.MessageService, sched *scheduler.Scheduler) *Server {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(log))

	server := &Server{
		router:         router,
		logger:         log.WithComponent("api"),
		messageService: messageService,
		scheduler:      sched,
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
			messages.POST("", s.createMessage)
			messages.GET("", s.getMessages)
			messages.GET("/:id", s.getMessage)
			messages.GET("/sent", s.getSentMessages)
			messages.POST("/retry", s.retryFailedMessages)
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
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/scheduler/start [post]
func (s *Server) startScheduler(c *gin.Context) {
	if s.scheduler == nil {
		s.logger.Error("Scheduler not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	if s.scheduler.IsRunning() {
		s.logger.Warn("Scheduler is already running")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "Scheduler is already running",
			"status": s.scheduler.GetStatus(),
		})
		return
	}

	if err := s.scheduler.Start(c.Request.Context()); err != nil {
		s.logger.Error("Failed to start scheduler", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start scheduler",
			"details": err.Error(),
		})
		return
	}

	s.logger.Info("Scheduler started successfully")
	c.JSON(http.StatusOK, gin.H{
		"message": "Scheduler started successfully",
		"status":  s.scheduler.GetStatus(),
	})
}

// stopScheduler godoc
// @Summary Stop the message scheduler
// @Description Stops the message processing scheduler
// @Tags scheduler
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/scheduler/stop [post]
func (s *Server) stopScheduler(c *gin.Context) {
	if s.scheduler == nil {
		s.logger.Error("Scheduler not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	if !s.scheduler.IsRunning() {
		s.logger.Warn("Scheduler is not running")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "Scheduler is not running",
			"status": s.scheduler.GetStatus(),
		})
		return
	}

	if err := s.scheduler.Stop(); err != nil {
		s.logger.Error("Failed to stop scheduler", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stop scheduler",
			"details": err.Error(),
		})
		return
	}

	s.logger.Info("Scheduler stopped successfully")
	c.JSON(http.StatusOK, gin.H{
		"message": "Scheduler stopped successfully",
		"status":  s.scheduler.GetStatus(),
	})
}

// CreateMessageRequest represents the request body for creating a message
type CreateMessageRequest struct {
	Recipient  string `json:"recipient" binding:"required" example:"user@example.com"`
	Content    string `json:"content" binding:"required" example:"Hello, World!"`
	WebhookURL string `json:"webhook_url" binding:"required" example:"https://example.com/webhook"`
}

// MessageResponse represents a message in API responses
type MessageResponse struct {
	ID         int64   `json:"id" example:"1"`
	Recipient  string  `json:"recipient" example:"user@example.com"`
	Content    string  `json:"content" example:"Hello, World!"`
	Status     string  `json:"status" example:"sent"`
	WebhookURL string  `json:"webhook_url" example:"https://example.com/webhook"`
	CreatedAt  string  `json:"created_at" example:"2023-01-01T00:00:00Z"`
	SentAt     *string `json:"sent_at,omitempty" example:"2023-01-01T00:01:00Z"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data  []MessageResponse `json:"data"`
	Total int               `json:"total" example:"100"`
	Page  int               `json:"page" example:"1"`
	Limit int               `json:"limit" example:"10"`
}

// RetryResponse represents the response for retry operations
type RetryResponse struct {
	Message string `json:"message" example:"Retry operation completed"`
	Count   int    `json:"count" example:"5"`
}

// createMessage godoc
// @Summary Create a new message
// @Description Creates a new message to be sent
// @Tags messages
// @Accept json
// @Produce json
// @Param message body CreateMessageRequest true "Message data"
// @Success 201 {object} MessageResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/messages [post]
func (s *Server) createMessage(c *gin.Context) {
	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid request body", "error", err)

		// Check for specific validation errors
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "Recipient") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Recipient is required"})
			return
		}
		if strings.Contains(errorMsg, "Content") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
			return
		}
		if strings.Contains(errorMsg, "WebhookURL") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook URL is required"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	message, err := s.messageService.CreateMessage(c.Request.Context(), &domain.CreateMessageRequest{
		Recipient:  req.Recipient,
		Content:    req.Content,
		WebhookURL: req.WebhookURL,
		MaxRetries: 3, // Default max retries
	})
	if err != nil {
		s.logger.Error("Failed to create message", "error", err, "recipient", req.Recipient)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	s.logger.Info("Message created successfully", "message_id", message.ID, "recipient", req.Recipient)
	c.JSON(http.StatusCreated, message)
}

// getMessages godoc
// @Summary Get messages
// @Description Retrieves a list of messages with pagination
// @Tags messages
// @Accept json
// @Produce json
// @Param offset query int false "Offset for pagination" default(0)
// @Param limit query int false "Items per page" default(50)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/messages [get]
func (s *Server) getMessages(c *gin.Context) {
	// Parse pagination parameters
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	messages, total, err := s.messageService.GetSentMessages(c.Request.Context(), offset, limit)
	if err != nil {
		s.logger.Error("Failed to get messages", "error", err, "offset", offset, "limit", limit)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages"})
		return
	}

	s.logger.Info("Messages retrieved successfully", "count", len(messages), "total", total, "offset", offset)
	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
	})
}

// getMessage godoc
// @Summary Get a specific message
// @Description Retrieves a specific message by ID
// @Tags messages
// @Accept json
// @Produce json
// @Param id path int true "Message ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/messages/{id} [get]
func (s *Server) getMessage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.logger.Error("Invalid message ID", "id", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	message, err := s.messageService.GetMessage(c.Request.Context(), id)
	if err != nil {
		s.logger.Error("Failed to get message", "message_id", id, "error", err)
		if err == domain.ErrMessageNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get message"})
		}
		return
	}

	s.logger.Info("Message retrieved successfully", "message_id", id)
	c.JSON(http.StatusOK, message)
}

// getSentMessages godoc
// @Summary Get sent messages
// @Description Retrieves a list of sent messages with pagination
// @Tags messages
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} PaginatedResponse
// @Router /api/v1/messages/sent [get]
func (s *Server) getSentMessages(c *gin.Context) {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	messages, total, err := s.messageService.GetSentMessages(c.Request.Context(), offset, limit)
	if err != nil {
		s.logger.Error("Failed to get sent messages", "error", err, "offset", offset, "limit", limit)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sent messages"})
		return
	}

	var responseMessages []MessageResponse
	for _, message := range messages {
		response := MessageResponse{
			ID:         message.ID,
			Recipient:  message.Recipient,
			Content:    message.Content,
			Status:     string(message.Status),
			WebhookURL: message.WebhookURL,
			CreatedAt:  message.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		if message.SentAt != nil {
			sentAt := message.SentAt.Format("2006-01-02T15:04:05Z")
			response.SentAt = &sentAt
		}

		responseMessages = append(responseMessages, response)
	}

	response := PaginatedResponse{
		Data:  responseMessages,
		Total: total,
		Page:  page,
		Limit: limit,
	}

	s.logger.Info("Sent messages retrieved successfully", "count", len(messages), "total", total, "page", page)
	c.JSON(http.StatusOK, response)
}

// RetryRequest represents the request body for retrying failed messages
type RetryRequest struct {
	BatchSize int `json:"batch_size,omitempty"`
}

// retryFailedMessages godoc
// @Summary Retry failed messages
// @Description Retries all failed messages
// @Tags messages
// @Accept json
// @Produce json
// @Param retry body RetryRequest false "Retry parameters"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/messages/retry [post]
func (s *Server) retryFailedMessages(c *gin.Context) {
	var req RetryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	count, err := s.messageService.RetryFailedMessages(c.Request.Context(), batchSize)
	if err != nil {
		s.logger.Error("Failed to retry failed messages", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retry failed messages"})
		return
	}

	s.logger.Info("Failed messages retry completed", "count", count)
	c.JSON(http.StatusOK, gin.H{"retried_count": count})
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
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
