package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/insider/insider-messaging/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMessageRepository is a mock implementation of MessageRepository
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) Create(ctx context.Context, req *domain.CreateMessageRequest) (*domain.Message, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) SelectUnsentForUpdate(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) MarkSent(ctx context.Context, messageID int64) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockMessageRepository) MarkFailed(ctx context.Context, messageID int64, errorMsg string) error {
	args := m.Called(ctx, messageID, errorMsg)
	return args.Error(0)
}

func (m *MockMessageRepository) GetByID(ctx context.Context, messageID int64) (*domain.Message, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *MockMessageRepository) GetSentMessages(ctx context.Context, offset, limit int) ([]*domain.Message, int, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Message), args.Int(1), args.Error(2)
}

func (m *MockMessageRepository) GetFailedMessages(ctx context.Context, limit int) ([]*domain.Message, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func TestMessageService_CreateMessage(t *testing.T) {
	mockRepo := new(MockMessageRepository)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewMessageService(mockRepo, logger)
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		req := &domain.CreateMessageRequest{
			Recipient:  "test@example.com",
			Content:    "Test message",
			WebhookURL: "https://example.com/webhook",
			MaxRetries: 3,
		}

		expectedMessage := &domain.Message{
			ID:         1,
			Recipient:  req.Recipient,
			Content:    req.Content,
			WebhookURL: req.WebhookURL,
			Status:     domain.MessageStatusPending,
			MaxRetries: req.MaxRetries,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		mockRepo.On("Create", ctx, req).Return(expectedMessage, nil)

		message, err := service.CreateMessage(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, expectedMessage, message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("validation errors", func(t *testing.T) {
		testCases := []struct {
			name string
			req  *domain.CreateMessageRequest
			err  string
		}{
			{
				name: "empty recipient",
				req: &domain.CreateMessageRequest{
					Content:    "Test message",
					WebhookURL: "https://example.com/webhook",
				},
				err: "recipient is required",
			},
			{
				name: "empty content",
				req: &domain.CreateMessageRequest{
					Recipient:  "test@example.com",
					WebhookURL: "https://example.com/webhook",
				},
				err: "content is required",
			},
			{
				name: "empty webhook URL",
				req: &domain.CreateMessageRequest{
					Recipient: "test@example.com",
					Content:   "Test message",
				},
				err: "webhook URL is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				message, err := service.CreateMessage(ctx, tc.req)
				require.Error(t, err)
				assert.Nil(t, message)
				assert.Contains(t, err.Error(), tc.err)
			})
		}
	})

	t.Run("repository error", func(t *testing.T) {
		req := &domain.CreateMessageRequest{
			Recipient:  "test@example.com",
			Content:    "Test message",
			WebhookURL: "https://example.com/webhook",
		}

		mockRepo.On("Create", ctx, req).Return(nil, errors.New("database error"))

		message, err := service.CreateMessage(ctx, req)
		require.Error(t, err)
		assert.Nil(t, message)
		assert.Contains(t, err.Error(), "failed to create message")

		mockRepo.AssertExpectations(t)
	})
}

func TestMessageService_ProcessUnsentMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	t.Run("successful processing", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		messages := []*domain.Message{
			{
				ID:         1,
				Recipient:  "test1@example.com",
				Content:    "Message 1",
				WebhookURL: "https://example.com/webhook1",
				Status:     domain.MessageStatusPending,
			},
			{
				ID:         2,
				Recipient:  "test2@example.com",
				Content:    "Message 2",
				WebhookURL: "https://example.com/webhook2",
				Status:     domain.MessageStatusPending,
			},
		}

		mockRepo.On("SelectUnsentForUpdate", ctx, 10).Return(messages, nil)
		mockRepo.On("MarkSent", ctx, int64(1)).Return(nil)
		mockRepo.On("MarkSent", ctx, int64(2)).Return(nil)

		processed, err := service.ProcessUnsentMessages(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, 2, processed)

		mockRepo.AssertExpectations(t)
	})

	t.Run("no messages found", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		mockRepo.On("SelectUnsentForUpdate", ctx, 10).Return([]*domain.Message{}, nil)

		processed, err := service.ProcessUnsentMessages(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, 0, processed)

		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		mockRepo.On("SelectUnsentForUpdate", ctx, 10).Return(nil, errors.New("database error"))

		processed, err := service.ProcessUnsentMessages(ctx, 10)
		require.Error(t, err)
		assert.Equal(t, 0, processed)
		assert.Contains(t, err.Error(), "failed to select unsent messages")

		mockRepo.AssertExpectations(t)
	})
}

func TestMessageService_GetMessage(t *testing.T) {
	mockRepo := new(MockMessageRepository)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewMessageService(mockRepo, logger)
	ctx := context.Background()

	t.Run("successful get", func(t *testing.T) {
		expectedMessage := &domain.Message{
			ID:         1,
			Recipient:  "test@example.com",
			Content:    "Test message",
			WebhookURL: "https://example.com/webhook",
			Status:     domain.MessageStatusSent,
		}

		mockRepo.On("GetByID", ctx, int64(1)).Return(expectedMessage, nil)

		message, err := service.GetMessage(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, expectedMessage, message)

		mockRepo.AssertExpectations(t)
	})

	t.Run("message not found", func(t *testing.T) {
		mockRepo.On("GetByID", ctx, int64(999)).Return(nil, errors.New("message not found"))

		message, err := service.GetMessage(ctx, 999)
		require.Error(t, err)
		assert.Nil(t, message)
		assert.Contains(t, err.Error(), "failed to get message")

		mockRepo.AssertExpectations(t)
	})
}

func TestMessageService_GetSentMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	t.Run("successful get sent messages", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		messages := []*domain.Message{
			{
				ID:        1,
				Recipient: "test1@example.com",
				Status:    domain.MessageStatusSent,
			},
			{
				ID:        2,
				Recipient: "test2@example.com",
				Status:    domain.MessageStatusSent,
			},
		}

		mockRepo.On("GetSentMessages", ctx, 0, 10).Return(messages, 25, nil)

		result, total, err := service.GetSentMessages(ctx, 0, 10)
		require.NoError(t, err)
		assert.Equal(t, messages, result)
		assert.Equal(t, 25, total)

		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		mockRepo.On("GetSentMessages", ctx, 0, 10).Return(([]*domain.Message)(nil), 0, errors.New("database error"))

		result, total, err := service.GetSentMessages(ctx, 0, 10)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, 0, total)
		assert.Contains(t, err.Error(), "failed to get sent messages")

		mockRepo.AssertExpectations(t)
	})
}

func TestMessageService_RetryFailedMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	ctx := context.Background()

	t.Run("successful retry", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		messages := []*domain.Message{
			{
				ID:         1,
				Recipient:  "test1@example.com",
				Status:     domain.MessageStatusFailed,
				RetryCount: 1,
				MaxRetries: 3,
			},
			{
				ID:         2,
				Recipient:  "test2@example.com",
				Status:     domain.MessageStatusFailed,
				RetryCount: 2,
				MaxRetries: 3,
			},
		}

		mockRepo.On("GetFailedMessages", ctx, 10).Return(messages, nil)
		mockRepo.On("MarkSent", ctx, int64(1)).Return(nil)
		mockRepo.On("MarkSent", ctx, int64(2)).Return(nil)

		retried, err := service.RetryFailedMessages(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, 2, retried)

		mockRepo.AssertExpectations(t)
	})

	t.Run("no failed messages", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		mockRepo.On("GetFailedMessages", ctx, 10).Return([]*domain.Message{}, nil)

		retried, err := service.RetryFailedMessages(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, 0, retried)

		mockRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo := new(MockMessageRepository)
		service := NewMessageService(mockRepo, logger)

		mockRepo.On("GetFailedMessages", ctx, 10).Return(([]*domain.Message)(nil), errors.New("database error"))

		retried, err := service.RetryFailedMessages(ctx, 10)
		require.Error(t, err)
		assert.Equal(t, 0, retried)
		assert.Contains(t, err.Error(), "failed to get failed messages")

		mockRepo.AssertExpectations(t)
	})
}
