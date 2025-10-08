package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// MessageMetadata represents cached metadata for sent messages
type MessageMetadata struct {
	ID         int       `json:"id"`
	Recipient  string    `json:"recipient"`
	Status     string    `json:"status"`
	SentAt     time.Time `json:"sent_at"`
	RetryCount int       `json:"retry_count"`
	MaxRetries int       `json:"max_retries"`
	WebhookURL string    `json:"webhook_url"`
}

// RedisCacheRepository provides Redis-based caching for message metadata
type RedisCacheRepository struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCacheRepository creates a new Redis cache repository
func NewRedisCacheRepository(redisURL string, ttl time.Duration) (*RedisCacheRepository, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCacheRepository{
		client: client,
		ttl:    ttl,
	}, nil
}

// CacheMessageMetadata stores message metadata in Redis
func (r *RedisCacheRepository) CacheMessageMetadata(ctx context.Context, metadata *MessageMetadata) error {
	key := fmt.Sprintf("message:metadata:%d", metadata.ID)

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache metadata: %w", err)
	}

	return nil
}

// GetMessageMetadata retrieves message metadata from Redis
func (r *RedisCacheRepository) GetMessageMetadata(ctx context.Context, messageID int) (*MessageMetadata, error) {
	key := fmt.Sprintf("message:metadata:%d", messageID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get metadata from cache: %w", err)
	}

	var metadata MessageMetadata
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}

// DeleteMessageMetadata removes message metadata from Redis
func (r *RedisCacheRepository) DeleteMessageMetadata(ctx context.Context, messageID int) error {
	key := fmt.Sprintf("message:metadata:%d", messageID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete metadata from cache: %w", err)
	}

	return nil
}

// CacheRecentlySentMessages stores a list of recently sent message IDs
func (r *RedisCacheRepository) CacheRecentlySentMessages(ctx context.Context, messageIDs []int) error {
	key := "messages:recently_sent"

	// Convert IDs to strings for Redis list
	values := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		values[i] = fmt.Sprintf("%d", id)
	}

	pipe := r.client.Pipeline()
	pipe.Del(ctx, key) // Clear existing list
	if len(values) > 0 {
		// Use RPush to maintain order (right push adds to end of list)
		pipe.RPush(ctx, key, values...)
		pipe.Expire(ctx, key, r.ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cache recently sent messages: %w", err)
	}

	return nil
}

// GetRecentlySentMessages retrieves recently sent message IDs from Redis
func (r *RedisCacheRepository) GetRecentlySentMessages(ctx context.Context, limit int) ([]int, error) {
	key := "messages:recently_sent"

	results, err := r.client.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		if err == redis.Nil {
			return []int{}, nil
		}
		return nil, fmt.Errorf("failed to get recently sent messages: %w", err)
	}

	messageIDs := make([]int, 0, len(results))
	for _, result := range results {
		var id int
		if _, err := fmt.Sscanf(result, "%d", &id); err == nil {
			messageIDs = append(messageIDs, id)
		}
	}

	return messageIDs, nil
}

// Close closes the Redis connection
func (r *RedisCacheRepository) Close() error {
	return r.client.Close()
}

// Health checks if Redis connection is healthy
func (r *RedisCacheRepository) Health(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
