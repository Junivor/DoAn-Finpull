package kafka

import (
    "context"
    "fmt"
    "time"

    "github.com/segmentio/kafka-go"
)

// ConsumerHook defines lifecycle hooks around message handling.
// Hooks can mutate context, message, and payload.
// Returning a non-nil error from BeforeHandle will skip handler execution
// and trigger error processing (OnError, DLQ, and offset commit).
type ConsumerHook interface {
    BeforeHandle(ctx context.Context, topic string, km kafka.Message, data []byte) (context.Context, kafka.Message, []byte, error)
    AfterHandle(ctx context.Context, topic string, km kafka.Message, data []byte, err error)
    OnError(ctx context.Context, topic string, km kafka.Message, data []byte, err error)
}

// NoopHook is a default hook that does nothing and is fully panic-safe.
type NoopHook struct{}

func (NoopHook) BeforeHandle(ctx context.Context, topic string, km kafka.Message, data []byte) (context.Context, kafka.Message, []byte, error) {
    return ctx, km, data, nil
}

func (NoopHook) AfterHandle(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {}

func (NoopHook) OnError(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {}

// HookError represents an error produced by a hook.
// Code can be used to classify errors (e.g., "ERR_VALIDATION", "ERR_TRANSFORM").
type HookError struct {
    Code string
    Err  error
}

func (e *HookError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Code, e.Err)
    }
    return e.Code
}

func (e *HookError) Unwrap() error { return e.Err }

// HookFuncs is an adapter that implements ConsumerHook from plain functions.
// All functions are optional; nil functions are treated as no-ops.
type HookFuncs struct {
    Before func(context.Context, string, kafka.Message, []byte) (context.Context, kafka.Message, []byte, error)
    After  func(context.Context, string, kafka.Message, []byte, error)
    Err    func(context.Context, string, kafka.Message, []byte, error)
}

func (h HookFuncs) BeforeHandle(ctx context.Context, topic string, km kafka.Message, data []byte) (context.Context, kafka.Message, []byte, error) {
    if h.Before == nil {
        return ctx, km, data, nil
    }
    return h.Before(ctx, topic, km, data)
}

func (h HookFuncs) AfterHandle(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    if h.After != nil {
        h.After(ctx, topic, km, data, err)
    }
}

func (h HookFuncs) OnError(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    if h.Err != nil {
        h.Err(ctx, topic, km, data, err)
    }
}

// HookChain composes multiple ConsumerHooks into one. It ensures:
// - BeforeHandle is applied in order; context/message/data are threaded through.
// - If any BeforeHandle returns error, OnError is invoked for all hooks and error is returned.
// - AfterHandle is applied in reverse order to mimic stack unwinding.
// - Hooks are executed panic-safe so they cannot crash the consumer.
type HookChain struct {
    hooks []ConsumerHook
}

// NewHookChain creates a composable hook chain. Nil hooks are ignored.
func NewHookChain(hooks ...ConsumerHook) *HookChain {
    filtered := make([]ConsumerHook, 0, len(hooks))
    for _, h := range hooks {
        if h != nil {
            filtered = append(filtered, h)
        }
    }
    return &HookChain{hooks: filtered}
}

func (c *HookChain) BeforeHandle(ctx context.Context, topic string, km kafka.Message, data []byte) (context.Context, kafka.Message, []byte, error) {
    curCtx, curMsg, curData := ctx, km, data
    for _, h := range c.hooks {
        var (
            nextCtx = curCtx
            nextMsg = curMsg
            nextData = curData
            err     error
        )
        // panic-safe execution
        func() {
            defer func() {
                if r := recover(); r != nil {
                    err = &HookError{Code: "ERR_PANIC", Err: fmt.Errorf("hook panic: %v", r)}
                }
            }()
            nextCtx, nextMsg, nextData, err = h.BeforeHandle(curCtx, topic, curMsg, curData)
        }()
        if err != nil {
            // notify error to all hooks
            for _, eh := range c.hooks {
                safeOnError(eh, curCtx, topic, curMsg, curData, err)
            }
            return curCtx, curMsg, curData, err
        }
        curCtx, curMsg, curData = nextCtx, nextMsg, nextData
    }
    return curCtx, curMsg, curData, nil
}

func (c *HookChain) AfterHandle(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    // reverse order for after hooks
    for i := len(c.hooks) - 1; i >= 0; i-- {
        safeAfter(c.hooks[i], ctx, topic, km, data, err)
    }
}

func (c *HookChain) OnError(ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    for _, h := range c.hooks {
        safeOnError(h, ctx, topic, km, data, err)
    }
}

// Context keys for common hook metadata.
type ctxKey string

const (
    // CtxStartTime holds time.Time for when handling started.
    CtxStartTime ctxKey = "kafka_hook_start_time"
    // CtxTraceID holds correlation/trace id extracted from headers.
    CtxTraceID   ctxKey = "kafka_hook_trace_id"
)

// WithStartTime sets start time in the context.
func WithStartTime(ctx context.Context, t time.Time) context.Context {
    return context.WithValue(ctx, CtxStartTime, t)
}

// WithTraceID sets trace id in the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
    if traceID == "" {
        return ctx
    }
    return context.WithValue(ctx, CtxTraceID, traceID)
}

// ExtractTraceID tries to get trace id from Kafka headers.
func ExtractTraceID(msg kafka.Message) string {
    for _, h := range msg.Headers {
        if h.Key == "trace_id" && len(h.Value) > 0 {
            return string(h.Value)
        }
    }
    return ""
}

// safeAfter executes AfterHandle and recovers from panic.
func safeAfter(h ConsumerHook, ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    defer func() {
        if r := recover(); r != nil {
            // swallow panic: hooks should never crash the consumer
        }
    }()
    h.AfterHandle(ctx, topic, km, data, err)
}

// safeOnError executes OnError and recovers from panic.
func safeOnError(h ConsumerHook, ctx context.Context, topic string, km kafka.Message, data []byte, err error) {
    defer func() {
        if r := recover(); r != nil {
            // swallow panic: hooks should never crash the consumer
        }
    }()
    h.OnError(ctx, topic, km, data, err)
}