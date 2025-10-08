package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/insider/insider-messaging/internal/domain"
)

// MessageRepository defines the interface for message data operations
type MessageRepository interface {
	// Create creates a new message in the database
	Create(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error)

	// SelectUnsentForUpdate selects unsent messages for processing with row-level locking
	SelectUnsentForUpdate(ctx context.Context, limit int) ([]*domain.Message, error)

	// MarkSent marks a message as sent
	MarkSent(ctx context.Context, messageID int64) error

	// MarkFailed marks a message as failed with error details
	MarkFailed(ctx context.Context, messageID int64, errorMsg string) error

	// GetByID retrieves a message by its ID
	GetByID(ctx context.Context, messageID int64) (*domain.Message, error)

	// GetSentMessages retrieves sent messages with pagination
	GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error)

	// GetFailedMessages retrieves failed messages that can be retried
	GetFailedMessages(ctx context.Context, limit int) ([]*domain.Message, error)
}

// messageRepository implements MessageRepository using PostgreSQL
type messageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *sql.DB) MessageRepository {
	return &messageRepository{db: db}
}

// Create creates a new message in the database
func (r *messageRepository) Create(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error) {
	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default max retries
	}

	query := `
		INSERT INTO messages (recipient, content, webhook_url, max_retries, status, retry_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, recipient, content, webhook_url, status, retry_count, max_retries, created_at, updated_at, sent_at, failed_at, error_message
	`

	var msg domain.Message
	var sentAt, failedAt sql.NullTime
	var errorMessage sql.NullString

	err := r.db.QueryRowContext(ctx, query,
		req.Recipient,
		req.Content,
		req.WebhookURL,
		maxRetries,
		domain.MessageStatusPending,
		0,
	).Scan(
		&msg.ID,
		&msg.Recipient,
		&msg.Content,
		&msg.WebhookURL,
		&msg.Status,
		&msg.RetryCount,
		&msg.MaxRetries,
		&msg.CreatedAt,
		&msg.UpdatedAt,
		&sentAt,
		&failedAt,
		&errorMessage,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Handle nullable fields
	if sentAt.Valid {
		msg.SentAt = &sentAt.Time
	}
	if failedAt.Valid {
		msg.FailedAt = &failedAt.Time
	}
	if errorMessage.Valid {
		msg.ErrorMessage = &errorMessage.String
	}

	return &msg, nil
}

// SelectUnsentForUpdate selects unsent messages for processing with row-level locking
func (r *messageRepository) SelectUnsentForUpdate(ctx context.Context, limit int) ([]*domain.Message, error) {
	query := `
		SELECT id, recipient, content, webhook_url, status, retry_count, max_retries, 
		       created_at, updated_at, sent_at, failed_at, error_message
		FROM messages 
		WHERE status = $1 OR (status = $2 AND retry_count < max_retries)
		ORDER BY created_at ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.QueryContext(ctx, query, domain.MessageStatusPending, domain.MessageStatusFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to select unsent messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		var sentAt, failedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&msg.ID,
			&msg.Recipient,
			&msg.Content,
			&msg.WebhookURL,
			&msg.Status,
			&msg.RetryCount,
			&msg.MaxRetries,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&sentAt,
			&failedAt,
			&errorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// Handle nullable fields
		if sentAt.Valid {
			msg.SentAt = &sentAt.Time
		}
		if failedAt.Valid {
			msg.FailedAt = &failedAt.Time
		}
		if errorMessage.Valid {
			msg.ErrorMessage = &errorMessage.String
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over messages: %w", err)
	}

	return messages, nil
}

// MarkSent marks a message as sent
func (r *messageRepository) MarkSent(ctx context.Context, messageID int64) error {
	query := `
		UPDATE messages 
		SET status = $1, sent_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, domain.MessageStatusSent, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message with ID %d not found", messageID)
	}

	return nil
}

// MarkFailed marks a message as failed with error details
func (r *messageRepository) MarkFailed(ctx context.Context, messageID int64, errorMsg string) error {
	query := `
		UPDATE messages 
		SET status = $1, error_message = $2, failed_at = NOW(), updated_at = NOW(), retry_count = retry_count + 1
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, domain.MessageStatusFailed, errorMsg, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message with ID %d not found", messageID)
	}

	return nil
}

// GetByID retrieves a message by its ID
func (r *messageRepository) GetByID(ctx context.Context, messageID int64) (*domain.Message, error) {
	query := `
		SELECT id, recipient, content, webhook_url, status, retry_count, max_retries, 
		       created_at, updated_at, sent_at, failed_at, error_message
		FROM messages 
		WHERE id = $1
	`

	var msg domain.Message
	var sentAt, failedAt sql.NullTime
	var errorMessage sql.NullString

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&msg.ID,
		&msg.Recipient,
		&msg.Content,
		&msg.WebhookURL,
		&msg.Status,
		&msg.RetryCount,
		&msg.MaxRetries,
		&msg.CreatedAt,
		&msg.UpdatedAt,
		&sentAt,
		&failedAt,
		&errorMessage,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message with ID %d not found", messageID)
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Handle nullable fields
	if sentAt.Valid {
		msg.SentAt = &sentAt.Time
	}
	if failedAt.Valid {
		msg.FailedAt = &failedAt.Time
	}
	if errorMessage.Valid {
		msg.ErrorMessage = &errorMessage.String
	}

	return &msg, nil
}

// GetSentMessages retrieves sent messages with pagination
func (r *messageRepository) GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error) {
	// First, get the total count
	countQuery := `SELECT COUNT(*) FROM messages WHERE status = $1`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, domain.MessageStatusSent).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sent messages: %w", err)
	}

	// Then get the paginated results
	query := `
		SELECT id, recipient, content, webhook_url, status, retry_count, max_retries, 
		       created_at, updated_at, sent_at, failed_at, error_message
		FROM messages 
		WHERE status = $1
		ORDER BY sent_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, domain.MessageStatusSent, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get sent messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		var sentAt, failedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&msg.ID,
			&msg.Recipient,
			&msg.Content,
			&msg.WebhookURL,
			&msg.Status,
			&msg.RetryCount,
			&msg.MaxRetries,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&sentAt,
			&failedAt,
			&errorMessage,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan sent message: %w", err)
		}

		// Handle nullable fields
		if sentAt.Valid {
			msg.SentAt = &sentAt.Time
		}
		if failedAt.Valid {
			msg.FailedAt = &failedAt.Time
		}
		if errorMessage.Valid {
			msg.ErrorMessage = &errorMessage.String
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating over sent messages: %w", err)
	}

	return messages, total, nil
}

// GetFailedMessages retrieves failed messages that can be retried
func (r *messageRepository) GetFailedMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	query := `
		SELECT id, recipient, content, webhook_url, status, retry_count, max_retries, 
		       created_at, updated_at, sent_at, failed_at, error_message
		FROM messages 
		WHERE status = $1 AND retry_count < max_retries
		ORDER BY failed_at ASC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, domain.MessageStatusFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get failed messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		var sentAt, failedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&msg.ID,
			&msg.Recipient,
			&msg.Content,
			&msg.WebhookURL,
			&msg.Status,
			&msg.RetryCount,
			&msg.MaxRetries,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&sentAt,
			&failedAt,
			&errorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan failed message: %w", err)
		}

		// Handle nullable fields
		if sentAt.Valid {
			msg.SentAt = &sentAt.Time
		}
		if failedAt.Valid {
			msg.FailedAt = &failedAt.Time
		}
		if errorMessage.Valid {
			msg.ErrorMessage = &errorMessage.String
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over failed messages: %w", err)
	}

	return messages, nil
}
