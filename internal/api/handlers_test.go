package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/insider/insider-messaging/internal/domain"
	"github.com/insider/insider-messaging/internal/scheduler"
	"github.com/insider/insider-messaging/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// createTestServerWithMock creates a test server with a provided mock service
func createTestServerWithMock(mockService *MockMessageService) *Server {
	testLogger := logger.New()
	mockScheduler := scheduler.NewScheduler(nil, testLogger, scheduler.DefaultConfig())
	return NewServer(testLogger, mockService, mockScheduler)
}

// MockMessageService is a mock implementation of MessageService for testing
type MockMessageService struct {
	mock.Mock
}

func (m *MockMessageService) CreateMessage(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageService) ProcessUnsentMessages(ctx context.Context, batchSize int) (int, error) {
	args := m.Called(ctx, batchSize)
	return args.Int(0), args.Error(1)
}

func (m *MockMessageService) GetMessage(ctx context.Context, messageID int64) (*domain.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageService) GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Message), args.Int(1), args.Error(2)
}

func (m *MockMessageService) RetryFailedMessages(ctx context.Context, batchSize int) (int, error) {
	args := m.Called(ctx, batchSize)
	return args.Int(0), args.Error(1)
}

func (m *MockMessageService) ProcessPendingMessages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create mock service
	mockService := new(MockMessageService)

	// Create a mock scheduler
	mockScheduler := scheduler.NewScheduler(nil, testLogger, scheduler.DefaultConfig())

	// Create server instance
	server := NewServer(testLogger, mockService, mockScheduler)

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
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create mock service
	mockService := new(MockMessageService)

	// Create a mock scheduler
	mockScheduler := scheduler.NewScheduler(nil, testLogger, scheduler.DefaultConfig())

	// Create server instance
	server := NewServer(testLogger, mockService, mockScheduler)

	// Verify server is not nil
	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.messageService)
}

func TestLoggerMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test logger
	testLogger := logger.New()

	// Create mock service
	mockService := new(MockMessageService)

	// Create a mock scheduler
	mockScheduler := scheduler.NewScheduler(nil, testLogger, scheduler.DefaultConfig())

	// Create server instance
	server := NewServer(testLogger, mockService, mockScheduler)

	// Create a test route
	server.router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Perform request
	server.router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockMessageService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful message creation",
			requestBody: `{
				"recipient": "test@example.com",
				"content": "Test message",
				"webhook_url": "https://example.com/webhook"
			}`,
			mockSetup: func(m *MockMessageService) {
				message := &domain.Message{
					ID:         1,
					Recipient:  "test@example.com",
					Content:    "Test message",
					WebhookURL: "https://example.com/webhook",
					Status:     domain.MessageStatusPending,
					MaxRetries: 3,
				}
				m.On("CreateMessage", mock.Anything, mock.AnythingOfType("*domain.CreateMessageRequest")).Return(message, nil)
			},
			expectedStatus: 201,
			expectedBody:   `{"id":1,"recipient":"test@example.com","content":"Test message","webhook_url":"https://example.com/webhook","status":"pending","max_retries":3,"retry_count":0,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			mockSetup:      func(m *MockMessageService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request body"}`,
		},
		{
			name: "missing recipient",
			requestBody: `{
				"content": "Test message",
				"webhook_url": "https://example.com/webhook"
			}`,
			mockSetup:      func(m *MockMessageService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Recipient is required"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockMessageService{}
			tt.mockSetup(mockService)

			server := createTestServerWithMock(mockService)

			req, _ := http.NewRequest("POST", "/api/v1/messages", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}

func TestGetMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		mockSetup      func(*MockMessageService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "successful get messages",
			queryParams: "?offset=0&limit=10",
			mockSetup: func(m *MockMessageService) {
				messages := []*domain.Message{
					{
						ID:        1,
						Recipient: "test1@example.com",
						Content:   "Test message 1",
						Status:    domain.MessageStatusSent,
					},
					{
						ID:        2,
						Recipient: "test2@example.com",
						Content:   "Test message 2",
						Status:    domain.MessageStatusPending,
					},
				}
				m.On("GetSentMessages", mock.Anything, 0, 10).Return(messages, 2, nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"messages":[{"id":1,"recipient":"test1@example.com","content":"Test message 1","webhook_url":"","status":"sent","max_retries":0,"retry_count":0,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"},{"id":2,"recipient":"test2@example.com","content":"Test message 2","webhook_url":"","status":"pending","max_retries":0,"retry_count":0,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}],"total":2,"offset":0,"limit":10}`,
		},
		{
			name:        "default pagination",
			queryParams: "",
			mockSetup: func(m *MockMessageService) {
				m.On("GetSentMessages", mock.Anything, 0, 50).Return([]*domain.Message{}, 0, nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"messages":[],"total":0,"offset":0,"limit":50}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockMessageService{}
			tt.mockSetup(mockService)

			server := createTestServerWithMock(mockService)

			req, _ := http.NewRequest("GET", "/api/v1/messages"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}

func TestGetMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		messageID      string
		mockSetup      func(*MockMessageService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:      "successful get message",
			messageID: "1",
			mockSetup: func(m *MockMessageService) {
				message := &domain.Message{
					ID:        1,
					Recipient: "test@example.com",
					Content:   "Test message",
					Status:    domain.MessageStatusSent,
				}
				m.On("GetMessage", mock.Anything, int64(1)).Return(message, nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"id":1,"recipient":"test@example.com","content":"Test message","webhook_url":"","status":"sent","max_retries":0,"retry_count":0,"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`,
		},
		{
			name:           "invalid message ID",
			messageID:      "invalid",
			mockSetup:      func(m *MockMessageService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid message ID"}`,
		},
		{
			name:      "message not found",
			messageID: "999",
			mockSetup: func(m *MockMessageService) {
				m.On("GetMessage", mock.Anything, int64(999)).Return(nil, domain.ErrMessageNotFound)
			},
			expectedStatus: 404,
			expectedBody:   `{"error":"Message not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockMessageService{}
			tt.mockSetup(mockService)

			server := createTestServerWithMock(mockService)

			req, _ := http.NewRequest("GET", "/api/v1/messages/"+tt.messageID, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}

func TestRetryFailedMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    string
		mockSetup      func(*MockMessageService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:        "successful retry",
			requestBody: `{"batch_size": 5}`,
			mockSetup: func(m *MockMessageService) {
				m.On("RetryFailedMessages", mock.Anything, 5).Return(3, nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"retried_count":3}`,
		},
		{
			name:        "default batch size",
			requestBody: `{}`,
			mockSetup: func(m *MockMessageService) {
				m.On("RetryFailedMessages", mock.Anything, 10).Return(0, nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"retried_count":0}`,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{"invalid": json}`,
			mockSetup:      func(m *MockMessageService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request body"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockMessageService{}
			tt.mockSetup(mockService)

			server := createTestServerWithMock(mockService)

			req, _ := http.NewRequest("POST", "/api/v1/messages/retry", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.JSONEq(t, tt.expectedBody, w.Body.String())

			mockService.AssertExpectations(t)
		})
	}
}
