package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/insider/insider-messaging/internal/domain"
	"github.com/insider/insider-messaging/internal/repo"
)

// MessageService defines the interface for message business logic
type MessageService interface {
	// CreateMessage creates a new message
	CreateMessage(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error)
	
	// ProcessUnsentMessages processes unsent messages for delivery
	ProcessUnsentMessages(ctx context.Context, batchSize int) (int, error)
	
	// ProcessPendingMessages processes pending messages (alias for ProcessUnsentMessages for scheduler compatibility)
	ProcessPendingMessages(ctx context.Context) error
	
	// GetMessage retrieves a message by ID
	GetMessage(ctx context.Context, messageID int64) (*domain.Message, error)
	
	// GetSentMessages retrieves sent messages with pagination
	GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error)
	
	// RetryFailedMessages retries failed messages that haven't exceeded max retries
	RetryFailedMessages(ctx context.Context, batchSize int) (int, error)
}
// messageService implements MessageService
type messageService struct {
	repo          repo.MessageRepository
	cache         *repo.RedisCacheRepository // Optional Redis cache
	webhookClient WebhookClient              // Optional webhook client
	logger        *slog.Logger
}

// NewMessageService creates a new message service without cache or webhook client
func NewMessageService(repo repo.MessageRepository, logger *slog.Logger) MessageService {
	return &messageService{
		repo:          repo,
		cache:         nil,
		webhookClient: nil,
		logger:        logger,
	}
}

// NewMessageServiceWithCache creates a new message service with Redis cache
func NewMessageServiceWithCache(repo repo.MessageRepository, cache *repo.RedisCacheRepository, logger *slog.Logger) MessageService {
	return &messageService{
		repo:          repo,
		cache:         cache,
		webhookClient: nil,
		logger:        logger,
	}
}

// NewMessageServiceWithWebhook creates a new message service with webhook client
func NewMessageServiceWithWebhook(repo repo.MessageRepository, webhookClient WebhookClient, logger *slog.Logger) MessageService {
	return &messageService{
		repo:          repo,
		cache:         nil,
		webhookClient: webhookClient,
		logger:        logger,
	}
}

// NewMessageServiceWithCacheAndWebhook creates a new message service with both Redis cache and webhook client
func NewMessageServiceWithCacheAndWebhook(repo repo.MessageRepository, cache *repo.RedisCacheRepository, webhookClient WebhookClient, logger *slog.Logger) MessageService {
	return &messageService{
		repo:          repo,
		cache:         cache,
		webhookClient: webhookClient,
		logger:        logger,
	}
}

// CreateMessage creates a new message
func (s *messageService) CreateMessage(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error) {
	// Validate the request
	if req.Recipient == "" {
		return nil, fmt.Errorf("recipient is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	s.logger.Info("Creating new message",
		"recipient", req.Recipient,
		"webhook_url", req.WebhookURL,
		"max_retries", req.MaxRetries,
	)

	message, err := s.repo.Create(ctx, req)
	if err != nil {
		s.logger.Error("Failed to create message",
			"error", err,
			"recipient", req.Recipient,
		)
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	s.logger.Info("Message created successfully",
		"message_id", message.ID,
		"recipient", message.Recipient,
	)

	return message, nil
}

// ProcessUnsentMessages processes unsent messages for delivery
func (s *messageService) ProcessUnsentMessages(ctx context.Context, batchSize int) (int, error) {
	s.logger.Info("Processing unsent messages", "batch_size", batchSize)

	messages, err := s.repo.SelectUnsentForUpdate(ctx, batchSize)
	if err != nil {
		s.logger.Error("Failed to select unsent messages", "error", err)
		return 0, fmt.Errorf("failed to select unsent messages: %w", err)
	}

	if len(messages) == 0 {
		s.logger.Debug("No unsent messages found")
		return 0, nil
	}

	processed := 0
	for _, message := range messages {
		if err := s.processMessage(ctx, message); err != nil {
			s.logger.Error("Failed to process message",
				"message_id", message.ID,
				"error", err,
			)
			// Continue processing other messages even if one fails
			continue
		}
		processed++
	}

	s.logger.Info("Processed unsent messages",
		"total_found", len(messages),
		"successfully_processed", processed,
	)

	return processed, nil
}

// processMessage processes a single message
func (s *messageService) processMessage(ctx context.Context, message *domain.Message) error {
	s.logger.Debug("Processing message",
		"message_id", message.ID,
		"recipient", message.Recipient,
		"retry_count", message.RetryCount,
	)

	// Use webhook client if available, otherwise skip webhook delivery
	if s.webhookClient != nil {
		if err := s.webhookClient.SendMessage(ctx, message); err != nil {
			s.logger.Error("Failed to send webhook",
				"message_id", message.ID,
				"webhook_url", message.WebhookURL,
				"error", err,
			)
			
			// Mark message as failed
			if markErr := s.repo.MarkFailed(ctx, message.ID, err.Error()); markErr != nil {
				s.logger.Error("Failed to mark message as failed",
					"message_id", message.ID,
					"error", markErr,
				)
				return fmt.Errorf("failed to mark message as failed: %w", markErr)
			}
			return fmt.Errorf("webhook delivery failed: %w", err)
		}
	} else {
		s.logger.Debug("No webhook client configured, skipping webhook delivery",
			"message_id", message.ID,
		)
	}
	
	// Mark message as sent
	if err := s.repo.MarkSent(ctx, message.ID); err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	// Cache message metadata if Redis cache is available
	if s.cache != nil {
		metadata := &repo.MessageMetadata{
			ID:         int(message.ID),
			Recipient:  message.Recipient,
			Status:     "sent",
			SentAt:     time.Now(),
			RetryCount: message.RetryCount,
			MaxRetries: message.MaxRetries,
			WebhookURL: message.WebhookURL,
		}
		
		if err := s.cache.CacheMessageMetadata(ctx, metadata); err != nil {
			// Log error but don't fail the operation
			s.logger.Warn("Failed to cache message metadata",
				"message_id", message.ID,
				"error", err,
			)
		}
	}

	s.logger.Info("Message processed successfully",
		"message_id", message.ID,
		"recipient", message.Recipient,
	)

	return nil
}

// GetMessage retrieves a message by ID
func (s *messageService) GetMessage(ctx context.Context, messageID int64) (*domain.Message, error) {
	s.logger.Debug("Getting message", "message_id", messageID)

	message, err := s.repo.GetByID(ctx, messageID)
	if err != nil {
		s.logger.Error("Failed to get message",
			"message_id", messageID,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return message, nil
}

// GetSentMessages retrieves sent messages with pagination
func (s *messageService) GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error) {
	s.logger.Debug("Getting sent messages",
		"offset", offset,
		"limit", limit,
	)

	messages, total, err := s.repo.GetSentMessages(ctx, offset, limit)
	if err != nil {
		s.logger.Error("Failed to get sent messages",
			"offset", offset,
			"limit", limit,
			"error", err,
		)
		return nil, 0, fmt.Errorf("failed to get sent messages: %w", err)
	}

	s.logger.Debug("Retrieved sent messages",
		"count", len(messages),
		"total", total,
	)

	return messages, total, nil
}

// RetryFailedMessages retries failed messages that haven't exceeded max retries
func (s *messageService) RetryFailedMessages(ctx context.Context, batchSize int) (int, error) {
	s.logger.Info("Retrying failed messages", "batch_size", batchSize)

	messages, err := s.repo.GetFailedMessages(ctx, batchSize)
	if err != nil {
		s.logger.Error("Failed to get failed messages", "error", err)
		return 0, fmt.Errorf("failed to get failed messages: %w", err)
	}

	if len(messages) == 0 {
		s.logger.Debug("No failed messages found for retry")
		return 0, nil
	}

	retried := 0
	for _, message := range messages {
		if !message.CanRetry() {
			s.logger.Debug("Message cannot be retried",
				"message_id", message.ID,
				"retry_count", message.RetryCount,
				"max_retries", message.MaxRetries,
			)
			continue
		}

		if err := s.processMessage(ctx, message); err != nil {
			s.logger.Error("Failed to retry message",
				"message_id", message.ID,
				"error", err,
			)
			// Mark as failed again with the new error
			if markErr := s.repo.MarkFailed(ctx, message.ID, err.Error()); markErr != nil {
				s.logger.Error("Failed to mark message as failed",
					"message_id", message.ID,
					"error", markErr,
				)
			}
			continue
		}
		retried++
	}

	s.logger.Info("Retried failed messages",
		"total_found", len(messages),
		"successfully_retried", retried,
	)

	return retried, nil
}

// ProcessPendingMessages processes pending messages (scheduler compatibility method)
func (s *messageService) ProcessPendingMessages(ctx context.Context) error {
	// Use a default batch size for scheduler processing
	const defaultBatchSize = 10
	
	_, err := s.ProcessUnsentMessages(ctx, defaultBatchSize)
	return err
}

// RetryFailedMessagesForScheduler retries failed messages (scheduler compatibility method)
func (s *messageService) RetryFailedMessagesForScheduler(ctx context.Context) error {
	// Use a default batch size for scheduler processing
	const defaultBatchSize = 10
	
	_, err := s.RetryFailedMessages(ctx, defaultBatchSize)
	return err
}