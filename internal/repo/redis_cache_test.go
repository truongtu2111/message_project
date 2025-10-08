package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisCacheRepository_Integration(t *testing.T) {
	// Skip if Redis is not available
	redisURL := "redis://localhost:6379"
	cache, err := NewRedisCacheRepository(redisURL, time.Hour)
	if err != nil {
		t.Skipf("Redis not available, skipping integration tests: %v", err)
		return
	}
	defer cache.Close()

	ctx := context.Background()

	t.Run("CacheAndGetMessageMetadata", func(t *testing.T) {
		metadata := &MessageMetadata{
			ID:         123,
			Recipient:  "test@example.com",
			Status:     "sent",
			SentAt:     time.Now().Truncate(time.Second), // Truncate for comparison
			RetryCount: 2,
			MaxRetries: 3,
			WebhookURL: "https://example.com/webhook",
		}

		// Cache the metadata
		err := cache.CacheMessageMetadata(ctx, metadata)
		require.NoError(t, err)

		// Retrieve the metadata
		retrieved, err := cache.GetMessageMetadata(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, metadata.ID, retrieved.ID)
		assert.Equal(t, metadata.Recipient, retrieved.Recipient)
		assert.Equal(t, metadata.Status, retrieved.Status)
		assert.Equal(t, metadata.SentAt.Unix(), retrieved.SentAt.Unix()) // Compare Unix timestamps
		assert.Equal(t, metadata.RetryCount, retrieved.RetryCount)
		assert.Equal(t, metadata.MaxRetries, retrieved.MaxRetries)
		assert.Equal(t, metadata.WebhookURL, retrieved.WebhookURL)
	})

	t.Run("GetNonExistentMetadata", func(t *testing.T) {
		// Try to get metadata that doesn't exist
		retrieved, err := cache.GetMessageMetadata(ctx, 999)
		require.NoError(t, err)
		assert.Nil(t, retrieved) // Should return nil for cache miss
	})

	t.Run("DeleteMessageMetadata", func(t *testing.T) {
		metadata := &MessageMetadata{
			ID:         456,
			Recipient:  "delete@example.com",
			Status:     "sent",
			SentAt:     time.Now(),
			RetryCount: 0,
			MaxRetries: 3,
			WebhookURL: "https://example.com/webhook",
		}

		// Cache the metadata
		err := cache.CacheMessageMetadata(ctx, metadata)
		require.NoError(t, err)

		// Verify it exists
		retrieved, err := cache.GetMessageMetadata(ctx, 456)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Delete the metadata
		err = cache.DeleteMessageMetadata(ctx, 456)
		require.NoError(t, err)

		// Verify it's gone
		retrieved, err = cache.GetMessageMetadata(ctx, 456)
		require.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("CacheAndGetRecentlySentMessages", func(t *testing.T) {
		messageIDs := []int{100, 101, 102, 103, 104}

		// Cache recently sent messages
		err := cache.CacheRecentlySentMessages(ctx, messageIDs)
		require.NoError(t, err)

		// Retrieve all messages
		retrieved, err := cache.GetRecentlySentMessages(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, messageIDs, retrieved)

		// Retrieve with limit
		retrieved, err = cache.GetRecentlySentMessages(ctx, 3)
		require.NoError(t, err)
		assert.Equal(t, messageIDs[:3], retrieved)
	})

	t.Run("CacheEmptyRecentlySentMessages", func(t *testing.T) {
		// Cache empty list
		err := cache.CacheRecentlySentMessages(ctx, []int{})
		require.NoError(t, err)

		// Should return empty list
		retrieved, err := cache.GetRecentlySentMessages(ctx, 10)
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		err := cache.Health(ctx)
		assert.NoError(t, err)
	})
}

func TestRedisCacheRepository_InvalidURL(t *testing.T) {
	_, err := NewRedisCacheRepository("invalid-url", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse Redis URL")
}

func TestRedisCacheRepository_ConnectionFailure(t *testing.T) {
	// Try to connect to non-existent Redis instance
	_, err := NewRedisCacheRepository("redis://localhost:9999", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}