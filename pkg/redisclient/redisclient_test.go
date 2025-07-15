package redisclient

import (
    "context"
    "testing"

    "github.com/go-redis/redis/v8"
    redismock "github.com/go-redis/redismock/v8"
)

// TestAddToStream_Success verifies that AddToStream writes to the Redis Stream on first attempt.
func TestAddToStream_Success(t *testing.T) {
    db, mock := redismock.NewClientMock()
    client := &Client{rdb: db}

    // Expect a single XADD with proper stream and values
    mock.ExpectXAdd(&redis.XAddArgs{
        Stream: "s",
        Values: map[string]interface{}{"foo": "bar"},
    }).SetVal("0-1")

    // Invoke AddToStream
    if err := client.AddToStream(context.Background(), "s", map[string]interface{}{"foo": "bar"}); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Ensure expectations were met
    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("unfulfilled expectations: %v", err)
    }
}

// TestAddToStream_RetryOnError ensures AddToStream retries on a transient Redis error.
func TestAddToStream_RetryOnError(t *testing.T) {
    db, mock := redismock.NewClientMock()
    client := &Client{rdb: db}

    // First call returns redis.Nil error, second call succeeds
    mock.ExpectXAdd(&redis.XAddArgs{Stream: "s", Values: map[string]interface{}{}}).SetErr(redis.Nil)
    mock.ExpectXAdd(&redis.XAddArgs{Stream: "s", Values: map[string]interface{}{}}).SetVal("0-2")

    // Use empty map for Values
    if err := client.AddToStream(context.Background(), "s", map[string]interface{}{}); err != nil {
        t.Fatalf("expected success after retry, got %v", err)
    }
    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("unfulfilled expectations: %v", err)
    }
}
