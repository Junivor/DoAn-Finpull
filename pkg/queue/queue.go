package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type QueueService interface {
	PublishMessage(ctx context.Context, msgType string, payload interface{}) error
}

// MessageHandler is a function that processes a message
type MessageHandler func(context.Context, interface{}) error

// QueueConfig contains the configuration for the queue
type QueueConfig struct {
	Workers    int           // number of workers
	QueueSize  int           // size of the queue
	RetryLimit int           // number of maximum retries
	RetryDelay time.Duration // time delay between retries
}

// Message represents a message in the queue
type Message struct {
	ID        string
	Type      string
	Payload   interface{}
	Attempts  int
	Timestamp time.Time
}

func ParsePayload[T any](payload interface{}) (*T, error) {
	var result T

	switch p := payload.(type) {
	case *T:
		return p, nil
	case T:
		return &p, nil
	case map[string]interface{}:
		jsonData, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal map to json: %w", err)
		}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal json to struct: %w", err)
		}
		return &result, nil
	case []interface{}:
		jsonData, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal slice to json: %w", err)
		}
		if err := json.Unmarshal(jsonData, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal json to struct slice: %w", err)
		}
		return &result, nil
	case json.RawMessage:
		if err := json.Unmarshal(p, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
		return &result, nil
	default:
		return nil, fmt.Errorf("invalid payload type: %T", payload)
	}
}
