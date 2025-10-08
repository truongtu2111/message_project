package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/insider/insider-messaging/pkg/logger"
)

// mockMessageService implements MessageService for testing
type mockMessageService struct {
	mu                   sync.Mutex
	processPendingCalled int
	retryFailedCalled    int
	processPendingError  error
	retryFailedError     error
	processPendingDelay  time.Duration
	retryFailedDelay     time.Duration
}

func (m *mockMessageService) ProcessPendingMessages(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processPendingCalled++

	if m.processPendingDelay > 0 {
		select {
		case <-time.After(m.processPendingDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.processPendingError
}

func (m *mockMessageService) RetryFailedMessages(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retryFailedCalled++

	if m.retryFailedDelay > 0 {
		select {
		case <-time.After(m.retryFailedDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.retryFailedError
}

func (m *mockMessageService) getCallCounts() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processPendingCalled, m.retryFailedCalled
}

func TestNewScheduler(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")

	t.Run("with default config", func(t *testing.T) {
		scheduler := NewScheduler(mockService, logger, nil)

		if scheduler.processingInterval != 30*time.Second {
			t.Errorf("Expected processing interval 30s, got %v", scheduler.processingInterval)
		}
		if scheduler.retryInterval != 5*time.Minute {
			t.Errorf("Expected retry interval 5m, got %v", scheduler.retryInterval)
		}
		if scheduler.running {
			t.Error("Expected scheduler to not be running initially")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			ProcessingInterval: 10 * time.Second,
			RetryInterval:      2 * time.Minute,
		}
		scheduler := NewScheduler(mockService, logger, config)

		if scheduler.processingInterval != 10*time.Second {
			t.Errorf("Expected processing interval 10s, got %v", scheduler.processingInterval)
		}
		if scheduler.retryInterval != 2*time.Minute {
			t.Errorf("Expected retry interval 2m, got %v", scheduler.retryInterval)
		}
	})
}

func TestScheduler_StartStop(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")
	config := &Config{
		ProcessingInterval: 100 * time.Millisecond,
		RetryInterval:      200 * time.Millisecond,
	}
	scheduler := NewScheduler(mockService, logger, config)

	ctx := context.Background()

	// Test starting scheduler
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("Expected scheduler to be running")
	}

	// Test starting already running scheduler
	if err := scheduler.Start(ctx); err == nil {
		t.Error("Expected error when starting already running scheduler")
	}

	// Let it run for a bit to ensure it's processing
	time.Sleep(300 * time.Millisecond)

	// Test stopping scheduler
	if err := scheduler.Stop(); err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("Expected scheduler to not be running")
	}

	// Test stopping already stopped scheduler
	if err := scheduler.Stop(); err == nil {
		t.Error("Expected error when stopping already stopped scheduler")
	}
}

func TestScheduler_Processing(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")
	config := &Config{
		ProcessingInterval: 50 * time.Millisecond,
		RetryInterval:      100 * time.Millisecond,
	}
	scheduler := NewScheduler(mockService, logger, config)

	ctx := context.Background()

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for multiple processing cycles
	time.Sleep(250 * time.Millisecond)

	processPending, retryFailed := mockService.getCallCounts()

	// Should have called ProcessPendingMessages multiple times
	if processPending < 2 {
		t.Errorf("Expected at least 2 ProcessPendingMessages calls, got %d", processPending)
	}

	// Should have called RetryFailedMessages at least once
	if retryFailed < 1 {
		t.Errorf("Expected at least 1 RetryFailedMessages call, got %d", retryFailed)
	}
}

func TestScheduler_ErrorHandling(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")
	config := &Config{
		ProcessingInterval: 50 * time.Millisecond,
		RetryInterval:      100 * time.Millisecond,
	}
	scheduler := NewScheduler(mockService, logger, config)

	// Set up errors
	mockService.processPendingError = errors.New("processing error")
	mockService.retryFailedError = errors.New("retry error")

	ctx := context.Background()

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for processing cycles with errors
	time.Sleep(250 * time.Millisecond)

	processPending, retryFailed := mockService.getCallCounts()

	// Should still attempt processing despite errors
	if processPending < 2 {
		t.Errorf("Expected at least 2 ProcessPendingMessages calls despite errors, got %d", processPending)
	}

	if retryFailed < 1 {
		t.Errorf("Expected at least 1 RetryFailedMessages call despite errors, got %d", retryFailed)
	}
}

func TestScheduler_GracefulShutdown(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")
	config := &Config{
		ProcessingInterval: 10 * time.Millisecond,
		RetryInterval:      20 * time.Millisecond,
	}
	scheduler := NewScheduler(mockService, logger, config)

	ctx := context.Background()

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Let it run for a bit to ensure it's processing
	time.Sleep(50 * time.Millisecond)

	// Stop should complete without error
	if err := scheduler.Stop(); err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	if scheduler.IsRunning() {
		t.Error("Expected scheduler to not be running after stop")
	}

	// Verify that processing was happening
	processPending, retryFailed := mockService.getCallCounts()
	if processPending == 0 && retryFailed == 0 {
		t.Error("Expected some processing to have occurred before shutdown")
	}
}

func TestScheduler_GetStatus(t *testing.T) {
	mockService := &mockMessageService{}
	logger := logger.New().WithComponent("scheduler-test")
	config := &Config{
		ProcessingInterval: 1 * time.Minute,
		RetryInterval:      5 * time.Minute,
	}
	scheduler := NewScheduler(mockService, logger, config)

	status := scheduler.GetStatus()

	if status["running"] != false {
		t.Error("Expected running to be false initially")
	}

	if status["processing_interval"] != "1m0s" {
		t.Errorf("Expected processing_interval to be '1m0s', got %v", status["processing_interval"])
	}

	if status["retry_interval"] != "5m0s" {
		t.Errorf("Expected retry_interval to be '5m0s', got %v", status["retry_interval"])
	}

	// Start scheduler and check status
	ctx := context.Background()
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	status = scheduler.GetStatus()
	if status["running"] != true {
		t.Error("Expected running to be true after start")
	}
}
