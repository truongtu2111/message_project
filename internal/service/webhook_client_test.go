package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/insider/insider-messaging/internal/domain"
	"github.com/insider/insider-messaging/pkg/config"
	"github.com/insider/insider-messaging/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookClient_SendMessage(t *testing.T) {
	cfg := &config.Config{
		BackoffMin: 100 * time.Millisecond,
		BackoffMax: 1 * time.Second,
	}
	log := logger.New().WithComponent("webhook-test")

	tests := []struct {
		name           string
		message        *domain.Message
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		expectRetry    bool
	}{
		{
			name: "successful delivery with 202",
			message: &domain.Message{
				ID:         1,
				Recipient:  "test@example.com",
				Content:    "Test message",
				WebhookURL: "", // Will be set to test server URL
				Status:     domain.MessageStatusPending,
				CreatedAt:  time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "insider-messaging/1.0", r.Header.Get("User-Agent"))

				// Verify payload
				var payload WebhookPayload
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(t, err)
				assert.Equal(t, int64(1), payload.MessageID)
				assert.Equal(t, "test@example.com", payload.Recipient)
				assert.Equal(t, "Test message", payload.Content)

				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte(`{"status": "accepted"}`))
			},
			expectError: false,
		},
		{
			name: "successful delivery with 200",
			message: &domain.Message{
				ID:         2,
				Recipient:  "test2@example.com",
				Content:    "Test message 2",
				WebhookURL: "", // Will be set to test server URL
				Status:     domain.MessageStatusPending,
				CreatedAt:  time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			},
			expectError: false,
		},
		{
			name: "4xx client error - non-retryable",
			message: &domain.Message{
				ID:         3,
				Recipient:  "invalid@example.com",
				Content:    "Test message",
				WebhookURL: "", // Will be set to test server URL
				Status:     domain.MessageStatusPending,
				CreatedAt:  time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "invalid payload"}`))
			},
			expectError: true,
			expectRetry: false,
		},
		{
			name: "5xx server error - retryable",
			message: &domain.Message{
				ID:         4,
				Recipient:  "test@example.com",
				Content:    "Test message",
				WebhookURL: "", // Will be set to test server URL
				Status:     domain.MessageStatusPending,
				CreatedAt:  time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "internal server error"}`))
			},
			expectError: true,
			expectRetry: true,
		},
		{
			name: "empty webhook URL - should skip",
			message: &domain.Message{
				ID:         5,
				Recipient:  "test@example.com",
				Content:    "Test message",
				WebhookURL: "",
				Status:     domain.MessageStatusPending,
				CreatedAt:  time.Now(),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Should not make HTTP request for empty webhook URL")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Set webhook URL to test server (unless it should be empty)
			if tt.message.WebhookURL == "" && tt.name != "empty webhook URL - should skip" {
				tt.message.WebhookURL = server.URL
			}

			client := NewWebhookClient(cfg, log)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := client.SendMessage(ctx, tt.message)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebhookClient_SendMessage_Timeout(t *testing.T) {
	cfg := &config.Config{
		BackoffMin: 100 * time.Millisecond,
		BackoffMax: 1 * time.Second,
	}
	log := logger.New().WithComponent("webhook-test")

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	message := &domain.Message{
		ID:         1,
		Recipient:  "test@example.com",
		Content:    "Test message",
		WebhookURL: server.URL,
		Status:     domain.MessageStatusPending,
		CreatedAt:  time.Now(),
	}

	client := NewWebhookClient(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.SendMessage(ctx, message)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestWebhookClient_SendMessage_RetryLogic(t *testing.T) {
	cfg := &config.Config{
		BackoffMin: 10 * time.Millisecond, // Very short for testing
		BackoffMax: 100 * time.Millisecond,
	}
	log := logger.New().WithComponent("webhook-test")

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 2 {
			// First two requests fail with 5xx
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		} else {
			// Third request succeeds
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"status": "accepted"}`))
		}
	}))
	defer server.Close()

	message := &domain.Message{
		ID:         1,
		Recipient:  "test@example.com",
		Content:    "Test message",
		WebhookURL: server.URL,
		Status:     domain.MessageStatusPending,
		CreatedAt:  time.Now(),
	}

	client := NewWebhookClient(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.SendMessage(ctx, message)
	// The webhook client allows 2 retries (3 total attempts), so this should succeed
	assert.NoError(t, err)
	assert.Equal(t, 3, requestCount, "Should have made 3 requests (1 initial + 2 retries)")
}

func TestWebhookPayload_JSON(t *testing.T) {
	now := time.Now()
	payload := WebhookPayload{
		MessageID: 123,
		Recipient: "test@example.com",
		Content:   "Test message",
		Status:    "pending",
		CreatedAt: now,
		SentAt:    now,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded WebhookPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload.MessageID, decoded.MessageID)
	assert.Equal(t, payload.Recipient, decoded.Recipient)
	assert.Equal(t, payload.Content, decoded.Content)
	assert.Equal(t, payload.Status, decoded.Status)
	assert.True(t, payload.CreatedAt.Equal(decoded.CreatedAt))
	assert.True(t, payload.SentAt.Equal(decoded.SentAt))
}
