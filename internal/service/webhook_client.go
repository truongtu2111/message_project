package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/insider/insider-messaging/internal/domain"
	"github.com/insider/insider-messaging/pkg/config"
	"github.com/insider/insider-messaging/pkg/logger"
	"github.com/sethvargo/go-retry"
)

// WebhookClient handles HTTP requests to webhook URLs
type WebhookClient interface {
	SendMessage(ctx context.Context, message *domain.Message) error
}

type webhookClient struct {
	httpClient *http.Client
	logger     *logger.Logger
	config     *config.Config
}

// WebhookPayload represents the payload sent to webhook URLs
type WebhookPayload struct {
	MessageID int64     `json:"message_id"`
	Recipient string    `json:"recipient"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	SentAt    time.Time `json:"sent_at"`
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(cfg *config.Config, logger *logger.Logger) WebhookClient {
	return &webhookClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		config: cfg,
	}
}

// SendMessage sends a message to the webhook URL with retry logic
func (w *webhookClient) SendMessage(ctx context.Context, message *domain.Message) error {
	if message.WebhookURL == "" {
		w.logger.Debug("No webhook URL provided, skipping webhook delivery", "message_id", message.ID)
		return nil
	}

	payload := WebhookPayload{
		MessageID: message.ID,
		Recipient: message.Recipient,
		Content:   message.Content,
		Status:    string(message.Status),
		CreatedAt: message.CreatedAt,
		SentAt:    time.Now(),
	}

	// Use exponential backoff with jitter for retries
	backoff := retry.NewExponential(w.config.BackoffMin)
	backoff = retry.WithMaxRetries(2, backoff) // Allow 2 retries (3 total attempts)
	backoff = retry.WithMaxDuration(w.config.BackoffMax, backoff)
	backoff = retry.WithJitter(time.Second, backoff)

	return retry.Do(ctx, backoff, func(ctx context.Context) error {
		return w.sendHTTPRequest(ctx, message.WebhookURL, payload)
	})
}

// sendHTTPRequest performs the actual HTTP request
func (w *webhookClient) sendHTTPRequest(ctx context.Context, webhookURL string, payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "insider-messaging/1.0")

	w.logger.Debug("Sending webhook request",
		"url", webhookURL,
		"message_id", payload.MessageID,
		"recipient", payload.Recipient)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Error("HTTP request failed",
			"url", webhookURL,
			"error", err,
			"message_id", payload.MessageID)
		return retry.RetryableError(fmt.Errorf("HTTP request failed: %w", err))
	}
	defer resp.Body.Close()

	// Read response body for logging
	body, _ := io.ReadAll(resp.Body)

	w.logger.Debug("Webhook response received",
		"url", webhookURL,
		"status_code", resp.StatusCode,
		"message_id", payload.MessageID,
		"response_body", string(body))

	// Handle different HTTP status codes according to the commit plan
	switch {
	case resp.StatusCode == http.StatusAccepted: // 202 - Success
		w.logger.Info("Webhook delivered successfully",
			"url", webhookURL,
			"message_id", payload.MessageID)
		return nil

	case resp.StatusCode >= 400 && resp.StatusCode < 500: // 4xx - Non-retryable
		w.logger.Error("Webhook delivery failed with client error",
			"url", webhookURL,
			"status_code", resp.StatusCode,
			"message_id", payload.MessageID,
			"response_body", string(body))
		return fmt.Errorf("webhook delivery failed with status %d: %s", resp.StatusCode, string(body))

	case resp.StatusCode >= 500: // 5xx - Retryable
		w.logger.Warn("Webhook delivery failed with server error, will retry",
			"url", webhookURL,
			"status_code", resp.StatusCode,
			"message_id", payload.MessageID,
			"response_body", string(body))
		return retry.RetryableError(fmt.Errorf("webhook delivery failed with status %d: %s", resp.StatusCode, string(body)))

	default:
		// Other 2xx codes (200, 201, etc.) are also considered success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			w.logger.Info("Webhook delivered successfully",
				"url", webhookURL,
				"status_code", resp.StatusCode,
				"message_id", payload.MessageID)
			return nil
		}

		// Unexpected status codes
		w.logger.Error("Webhook delivery failed with unexpected status",
			"url", webhookURL,
			"status_code", resp.StatusCode,
			"message_id", payload.MessageID,
			"response_body", string(body))
		return fmt.Errorf("webhook delivery failed with unexpected status %d: %s", resp.StatusCode, string(body))
	}
}
