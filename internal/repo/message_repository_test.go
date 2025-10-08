package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/insider/insider-messaging/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		req := &domain.CreateMessageRequest{
			Recipient:  "test@example.com",
			Content:    "Test message",
			WebhookURL: "https://example.com/webhook",
			MaxRetries: 5,
		}

		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, req.Recipient, req.Content, req.WebhookURL, domain.MessageStatusPending, 
			0, req.MaxRetries, now, now, nil, nil, nil,
		)

		mock.ExpectQuery(`INSERT INTO messages`).
			WithArgs(req.Recipient, req.Content, req.WebhookURL, req.MaxRetries, domain.MessageStatusPending, 0).
			WillReturnRows(rows)

		msg, err := repo.Create(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int64(1), msg.ID)
		assert.Equal(t, req.Recipient, msg.Recipient)
		assert.Equal(t, req.Content, msg.Content)
		assert.Equal(t, req.WebhookURL, msg.WebhookURL)
		assert.Equal(t, domain.MessageStatusPending, msg.Status)
		assert.Equal(t, 0, msg.RetryCount)
		assert.Equal(t, req.MaxRetries, msg.MaxRetries)
		assert.Nil(t, msg.SentAt)
		assert.Nil(t, msg.FailedAt)
		assert.Nil(t, msg.ErrorMessage)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("default max retries", func(t *testing.T) {
		req := &domain.CreateMessageRequest{
			Recipient:  "test@example.com",
			Content:    "Test message",
			WebhookURL: "https://example.com/webhook",
			MaxRetries: 0, // Should default to 3
		}

		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, req.Recipient, req.Content, req.WebhookURL, domain.MessageStatusPending, 
			0, 3, now, now, nil, nil, nil,
		)

		mock.ExpectQuery(`INSERT INTO messages`).
			WithArgs(req.Recipient, req.Content, req.WebhookURL, 3, domain.MessageStatusPending, 0).
			WillReturnRows(rows)

		msg, err := repo.Create(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 3, msg.MaxRetries)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_SelectUnsentForUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful selection", func(t *testing.T) {
		now := time.Now()
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, "test1@example.com", "Message 1", "https://example.com/webhook1", 
			domain.MessageStatusPending, 0, 3, now, now, nil, nil, nil,
		).AddRow(
			2, "test2@example.com", "Message 2", "https://example.com/webhook2", 
			domain.MessageStatusFailed, 1, 3, now, now, nil, now, "Previous error",
		)

		mock.ExpectQuery(`SELECT .+ FROM messages .+ FOR UPDATE SKIP LOCKED`).
			WithArgs(domain.MessageStatusPending, domain.MessageStatusFailed, 10).
			WillReturnRows(rows)

		messages, err := repo.SelectUnsentForUpdate(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, messages, 2)

		// Check first message
		assert.Equal(t, int64(1), messages[0].ID)
		assert.Equal(t, domain.MessageStatusPending, messages[0].Status)
		assert.Equal(t, 0, messages[0].RetryCount)

		// Check second message
		assert.Equal(t, int64(2), messages[1].ID)
		assert.Equal(t, domain.MessageStatusFailed, messages[1].Status)
		assert.Equal(t, 1, messages[1].RetryCount)
		assert.NotNil(t, messages[1].ErrorMessage)
		assert.Equal(t, "Previous error", *messages[1].ErrorMessage)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no messages found", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		})

		mock.ExpectQuery(`SELECT .+ FROM messages .+ FOR UPDATE SKIP LOCKED`).
			WithArgs(domain.MessageStatusPending, domain.MessageStatusFailed, 10).
			WillReturnRows(rows)

		messages, err := repo.SelectUnsentForUpdate(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, messages, 0)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_MarkSent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful mark as sent", func(t *testing.T) {
		mock.ExpectExec(`UPDATE messages SET status = .+, sent_at = NOW\(\), updated_at = NOW\(\)`).
			WithArgs(domain.MessageStatusSent, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.MarkSent(ctx, 1)
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("message not found", func(t *testing.T) {
		mock.ExpectExec(`UPDATE messages SET status = .+, sent_at = NOW\(\), updated_at = NOW\(\)`).
			WithArgs(domain.MessageStatusSent, int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.MarkSent(ctx, 999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message with ID 999 not found")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_MarkFailed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful mark as failed", func(t *testing.T) {
		errorMsg := "Connection timeout"
		mock.ExpectExec(`UPDATE messages SET status = .+, error_message = .+, failed_at = NOW\(\), updated_at = NOW\(\), retry_count = retry_count \+ 1`).
			WithArgs(domain.MessageStatusFailed, errorMsg, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.MarkFailed(ctx, 1, errorMsg)
		require.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("message not found", func(t *testing.T) {
		errorMsg := "Connection timeout"
		mock.ExpectExec(`UPDATE messages SET status = .+, error_message = .+, failed_at = NOW\(\), updated_at = NOW\(\), retry_count = retry_count \+ 1`).
			WithArgs(domain.MessageStatusFailed, errorMsg, int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.MarkFailed(ctx, 999, errorMsg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message with ID 999 not found")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful get by ID", func(t *testing.T) {
		now := time.Now()
		sentAt := now.Add(time.Hour)
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, "test@example.com", "Test message", "https://example.com/webhook", 
			domain.MessageStatusSent, 0, 3, now, now, sentAt, nil, nil,
		)

		mock.ExpectQuery(`SELECT .+ FROM messages WHERE id = \$1`).
			WithArgs(int64(1)).
			WillReturnRows(rows)

		msg, err := repo.GetByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), msg.ID)
		assert.Equal(t, "test@example.com", msg.Recipient)
		assert.Equal(t, domain.MessageStatusSent, msg.Status)
		assert.NotNil(t, msg.SentAt)
		assert.Equal(t, sentAt, *msg.SentAt)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("message not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT .+ FROM messages WHERE id = \$1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		msg, err := repo.GetByID(ctx, 999)
		require.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "message with ID 999 not found")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_GetSentMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful get sent messages", func(t *testing.T) {
		// Mock count query
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(25)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM messages WHERE status = \$1`).
			WithArgs(domain.MessageStatusSent).
			WillReturnRows(countRows)

		// Mock data query
		now := time.Now()
		sentAt := now.Add(time.Hour)
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, "test1@example.com", "Message 1", "https://example.com/webhook1", 
			domain.MessageStatusSent, 0, 3, now, now, sentAt, nil, nil,
		).AddRow(
			2, "test2@example.com", "Message 2", "https://example.com/webhook2", 
			domain.MessageStatusSent, 0, 3, now, now, sentAt, nil, nil,
		)

		mock.ExpectQuery(`SELECT .+ FROM messages WHERE status = \$1 ORDER BY sent_at DESC LIMIT \$2 OFFSET \$3`).
			WithArgs(domain.MessageStatusSent, 10, 0).
			WillReturnRows(rows)

		messages, total, err := repo.GetSentMessages(ctx, 0, 10)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, 25, total)
		assert.Equal(t, domain.MessageStatusSent, messages[0].Status)
		assert.Equal(t, domain.MessageStatusSent, messages[1].Status)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMessageRepository_GetFailedMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	t.Run("successful get failed messages", func(t *testing.T) {
		now := time.Now()
		failedAt := now.Add(time.Hour)
		errorMsg := "Connection timeout"
		rows := sqlmock.NewRows([]string{
			"id", "recipient", "content", "webhook_url", "status", "retry_count", 
			"max_retries", "created_at", "updated_at", "sent_at", "failed_at", "error_message",
		}).AddRow(
			1, "test1@example.com", "Message 1", "https://example.com/webhook1", 
			domain.MessageStatusFailed, 1, 3, now, now, nil, failedAt, errorMsg,
		).AddRow(
			2, "test2@example.com", "Message 2", "https://example.com/webhook2", 
			domain.MessageStatusFailed, 2, 3, now, now, nil, failedAt, errorMsg,
		)

		mock.ExpectQuery(`SELECT .+ FROM messages WHERE status = \$1 AND retry_count < max_retries ORDER BY failed_at ASC LIMIT \$2`).
			WithArgs(domain.MessageStatusFailed, 10).
			WillReturnRows(rows)

		messages, err := repo.GetFailedMessages(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, domain.MessageStatusFailed, messages[0].Status)
		assert.Equal(t, 1, messages[0].RetryCount)
		assert.Equal(t, domain.MessageStatusFailed, messages[1].Status)
		assert.Equal(t, 2, messages[1].RetryCount)
		assert.NotNil(t, messages[0].ErrorMessage)
		assert.Equal(t, errorMsg, *messages[0].ErrorMessage)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}