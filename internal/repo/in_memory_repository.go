package repo

import (
	"context"
	"sync"
	"time"

	"github.com/insider/insider-messaging/internal/domain"
)

// inMemoryMessageRepository implements MessageRepository using in-memory storage
type inMemoryMessageRepository struct {
	mu       sync.RWMutex
	messages map[int64]*domain.Message
	nextID   int64
}

// NewInMemoryMessageRepository creates a new in-memory message repository
func NewInMemoryMessageRepository() MessageRepository {
	return &inMemoryMessageRepository{
		messages: make(map[int64]*domain.Message),
		nextID:   1,
	}
}

// Create creates a new message in memory
func (r *inMemoryMessageRepository) Create(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default max retries
	}

	message := &domain.Message{
		ID:         r.nextID,
		Recipient:  req.Recipient,
		Content:    req.Content,
		WebhookURL: req.WebhookURL,
		Status:     domain.MessageStatusPending,
		MaxRetries: maxRetries,
		RetryCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	r.messages[r.nextID] = message
	r.nextID++

	return message, nil
}

// SelectUnsentForUpdate selects unsent messages for processing
func (r *inMemoryMessageRepository) SelectUnsentForUpdate(ctx context.Context, limit int) ([]*domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var messages []*domain.Message
	count := 0

	for _, message := range r.messages {
		if message.Status == domain.MessageStatusPending && count < limit {
			messages = append(messages, message)
			count++
		}
	}

	return messages, nil
}

// MarkSent marks a message as sent
func (r *inMemoryMessageRepository) MarkSent(ctx context.Context, messageID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	message, exists := r.messages[messageID]
	if !exists {
		return domain.ErrMessageNotFound
	}

	now := time.Now()
	message.Status = domain.MessageStatusSent
	message.SentAt = &now
	message.UpdatedAt = now

	return nil
}

// MarkFailed marks a message as failed with error details
func (r *inMemoryMessageRepository) MarkFailed(ctx context.Context, messageID int64, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	message, exists := r.messages[messageID]
	if !exists {
		return domain.ErrMessageNotFound
	}

	message.Status = domain.MessageStatusFailed
	message.ErrorMessage = &errorMsg
	message.RetryCount++
	message.UpdatedAt = time.Now()

	return nil
}

// GetByID retrieves a message by its ID
func (r *inMemoryMessageRepository) GetByID(ctx context.Context, messageID int64) (*domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	message, exists := r.messages[messageID]
	if !exists {
		return nil, domain.ErrMessageNotFound
	}

	return message, nil
}

// GetSentMessages retrieves sent messages with pagination
func (r *inMemoryMessageRepository) GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sentMessages []*domain.Message
	for _, message := range r.messages {
		if message.Status == domain.MessageStatusSent {
			sentMessages = append(sentMessages, message)
		}
	}

	total := len(sentMessages)

	// Apply pagination
	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	if start >= total {
		return []*domain.Message{}, total, nil
	}

	return sentMessages[start:end], total, nil
}

// GetFailedMessages retrieves failed messages that can be retried
func (r *inMemoryMessageRepository) GetFailedMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var failedMessages []*domain.Message
	count := 0

	for _, message := range r.messages {
		if message.Status == domain.MessageStatusFailed && message.RetryCount < message.MaxRetries && count < limit {
			failedMessages = append(failedMessages, message)
			count++
		}
	}

	return failedMessages, nil
}
