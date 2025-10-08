package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageStatus_IsValid(t *testing.T) {
	tests := []struct {
		status   MessageStatus
		expected bool
	}{
		{MessageStatusPending, true},
		{MessageStatusSent, true},
		{MessageStatusFailed, true},
		{MessageStatus("invalid"), false},
		{MessageStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsValid())
		})
	}
}

func TestMessage_CanRetry(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		expected bool
	}{
		{
			name: "failed message with retries available",
			message: Message{
				Status:     MessageStatusFailed,
				RetryCount: 1,
				MaxRetries: 3,
			},
			expected: true,
		},
		{
			name: "failed message with no retries left",
			message: Message{
				Status:     MessageStatusFailed,
				RetryCount: 3,
				MaxRetries: 3,
			},
			expected: false,
		},
		{
			name: "pending message",
			message: Message{
				Status:     MessageStatusPending,
				RetryCount: 0,
				MaxRetries: 3,
			},
			expected: false,
		},
		{
			name: "sent message",
			message: Message{
				Status:     MessageStatusSent,
				RetryCount: 0,
				MaxRetries: 3,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.message.CanRetry())
		})
	}
}

func TestMessage_MarkAsSent(t *testing.T) {
	message := &Message{
		Status: MessageStatusPending,
	}

	before := time.Now()
	message.MarkAsSent()
	after := time.Now()

	assert.Equal(t, MessageStatusSent, message.Status)
	assert.NotNil(t, message.SentAt)
	assert.True(t, message.SentAt.After(before) || message.SentAt.Equal(before))
	assert.True(t, message.SentAt.Before(after) || message.SentAt.Equal(after))
	assert.True(t, message.UpdatedAt.After(before) || message.UpdatedAt.Equal(before))
	assert.True(t, message.UpdatedAt.Before(after) || message.UpdatedAt.Equal(after))
}

func TestMessage_MarkAsFailed(t *testing.T) {
	message := &Message{
		Status:     MessageStatusPending,
		RetryCount: 0,
	}

	errorMsg := "connection timeout"
	before := time.Now()
	message.MarkAsFailed(errorMsg)
	after := time.Now()

	assert.Equal(t, MessageStatusFailed, message.Status)
	assert.Equal(t, 1, message.RetryCount)
	assert.NotNil(t, message.ErrorMessage)
	assert.Equal(t, errorMsg, *message.ErrorMessage)
	assert.NotNil(t, message.FailedAt)
	assert.True(t, message.FailedAt.After(before) || message.FailedAt.Equal(before))
	assert.True(t, message.FailedAt.Before(after) || message.FailedAt.Equal(after))
	assert.True(t, message.UpdatedAt.After(before) || message.UpdatedAt.Equal(before))
	assert.True(t, message.UpdatedAt.Before(after) || message.UpdatedAt.Equal(after))
}
