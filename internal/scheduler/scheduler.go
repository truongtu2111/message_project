package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/insider/insider-messaging/pkg/logger"
)

// MessageService defines the interface for message processing
type MessageService interface {
	ProcessPendingMessages(ctx context.Context) error
	RetryFailedMessages(ctx context.Context) error
}

// Scheduler manages background message processing
type Scheduler struct {
	messageService MessageService
	logger         *logger.Logger
	
	// Configuration
	processingInterval time.Duration
	retryInterval      time.Duration
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Status
	running bool
	mu      sync.RWMutex
}

// Config holds scheduler configuration
type Config struct {
	ProcessingInterval time.Duration
	RetryInterval      time.Duration
}

// DefaultConfig returns default scheduler configuration
func DefaultConfig() *Config {
	return &Config{
		ProcessingInterval: 30 * time.Second,
		RetryInterval:      5 * time.Minute,
	}
}

// NewScheduler creates a new scheduler instance
func NewScheduler(messageService MessageService, logger *logger.Logger, config *Config) *Scheduler {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Scheduler{
		messageService:     messageService,
		logger:            logger.WithComponent("scheduler"),
		processingInterval: config.ProcessingInterval,
		retryInterval:     config.RetryInterval,
	}
}

// Start begins the scheduler background processing
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		return fmt.Errorf("scheduler is already running")
	}
	
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	
	s.logger.Info("Starting scheduler",
		"processing_interval", s.processingInterval,
		"retry_interval", s.retryInterval,
	)
	
	// Start processing goroutine
	s.wg.Add(1)
	go s.processMessages()
	
	// Start retry goroutine
	s.wg.Add(1)
	go s.retryFailedMessages()
	
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}
	
	s.logger.Info("Stopping scheduler")
	
	// Cancel context to signal goroutines to stop
	s.cancel()
	
	// Wait for all goroutines to finish
	s.wg.Wait()
	
	s.running = false
	s.logger.Info("Scheduler stopped")
	
	return nil
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// processMessages runs the main message processing loop
func (s *Scheduler) processMessages() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.processingInterval)
	defer ticker.Stop()
	
	s.logger.Info("Message processing loop started")
	
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Message processing loop stopped")
			return
		case <-ticker.C:
			s.processMessagesOnce()
		}
	}
}

// retryFailedMessages runs the retry processing loop
func (s *Scheduler) retryFailedMessages() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.retryInterval)
	defer ticker.Stop()
	
	s.logger.Info("Retry processing loop started")
	
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Retry processing loop stopped")
			return
		case <-ticker.C:
			s.retryFailedMessagesOnce()
		}
	}
}

// processMessagesOnce processes pending messages once
func (s *Scheduler) processMessagesOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	
	s.logger.Debug("Processing pending messages")
	
	if err := s.messageService.ProcessPendingMessages(ctx); err != nil {
		s.logger.Error("Failed to process pending messages", "error", err)
		return
	}
	
	s.logger.Debug("Pending messages processed successfully")
}

// retryFailedMessagesOnce retries failed messages once
func (s *Scheduler) retryFailedMessagesOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	
	s.logger.Debug("Retrying failed messages")
	
	if err := s.messageService.RetryFailedMessages(ctx); err != nil {
		s.logger.Error("Failed to retry failed messages", "error", err)
		return
	}
	
	s.logger.Debug("Failed messages retry completed")
}

// GetStatus returns the current scheduler status
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"running":             s.running,
		"processing_interval": s.processingInterval.String(),
		"retry_interval":      s.retryInterval.String(),
	}
}