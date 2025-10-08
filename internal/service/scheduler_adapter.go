package service

import (
	"context"
)

// SchedulerAdapter adapts MessageService to scheduler.MessageService interface
type SchedulerAdapter struct {
	messageService MessageService
}

// NewSchedulerAdapter creates a new scheduler adapter
func NewSchedulerAdapter(messageService MessageService) *SchedulerAdapter {
	return &SchedulerAdapter{
		messageService: messageService,
	}
}

// ProcessPendingMessages implements scheduler.MessageService interface
func (a *SchedulerAdapter) ProcessPendingMessages(ctx context.Context) error {
	return a.messageService.ProcessPendingMessages(ctx)
}

// RetryFailedMessages implements scheduler.MessageService interface
func (a *SchedulerAdapter) RetryFailedMessages(ctx context.Context) error {
	// Use a default batch size for scheduler processing
	const defaultBatchSize = 10

	_, err := a.messageService.RetryFailedMessages(ctx, defaultBatchSize)
	return err
}
