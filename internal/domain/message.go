package domain

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrMessageNotFound = errors.New("message not found")
)

// MessageStatus represents the status of a message
type MessageStatus string

const (
	MessageStatusPending MessageStatus = "pending"
	MessageStatusSent    MessageStatus = "sent"
	MessageStatusFailed  MessageStatus = "failed"
)

// Message represents a message in the system
type Message struct {
	ID           int64         `json:"id" db:"id"`
	Recipient    string        `json:"recipient" db:"recipient" validate:"required,email"`
	Content      string        `json:"content" db:"content" validate:"required"`
	WebhookURL   string        `json:"webhook_url" db:"webhook_url" validate:"required,url"`
	Status       MessageStatus `json:"status" db:"status"`
	RetryCount   int           `json:"retry_count" db:"retry_count"`
	MaxRetries   int           `json:"max_retries" db:"max_retries"`
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at" db:"updated_at"`
	SentAt       *time.Time    `json:"sent_at,omitempty" db:"sent_at"`
	FailedAt     *time.Time    `json:"failed_at,omitempty" db:"failed_at"`
	ErrorMessage *string       `json:"error_message,omitempty" db:"error_message"`
}

// IsValid checks if the message status is valid
func (s MessageStatus) IsValid() bool {
	switch s {
	case MessageStatusPending, MessageStatusSent, MessageStatusFailed:
		return true
	default:
		return false
	}
}

// CanRetry checks if the message can be retried
func (m *Message) CanRetry() bool {
	return m.Status == MessageStatusFailed && m.RetryCount < m.MaxRetries
}

// MarkAsSent marks the message as sent
func (m *Message) MarkAsSent() {
	m.Status = MessageStatusSent
	now := time.Now()
	m.SentAt = &now
	m.UpdatedAt = now
}

// MarkAsFailed marks the message as failed with an error
func (m *Message) MarkAsFailed(errorMsg string) {
	m.Status = MessageStatusFailed
	m.ErrorMessage = &errorMsg
	now := time.Now()
	m.FailedAt = &now
	m.UpdatedAt = now
	m.RetryCount++
}

// CreateMessageRequest represents the request to create a new message
type CreateMessageRequest struct {
	Recipient  string `json:"recipient" validate:"required,email"`
	Content    string `json:"content" validate:"required"`
	WebhookURL string `json:"webhook_url" validate:"required,url"`
	MaxRetries int    `json:"max_retries,omitempty"`
}
