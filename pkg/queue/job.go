package queue

import "context"

// Job defines a queue job handler.
type Job interface {
	// Name returns the unique identifier of the job.
	Name() string

	// Type returns the type of message that the job handles.
	Type() string

	// Handle processes the job with the given payload.
	Handle(ctx context.Context, payload interface{}) error
}
