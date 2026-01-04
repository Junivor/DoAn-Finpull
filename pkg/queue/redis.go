package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"FinPull/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// QueueMode defines the operation mode of the queue.
type QueueMode int

const (
	ModeProducerConsumer QueueMode = iota
	ModeProducerOnly
	ModeConsumerOnly
)

// RedisQueue represents a Redis-based queue.
type RedisQueue struct {
	logger    *logger.Logger
	config    *QueueConfig
	client    *redis.Client
	jobs      map[string]Job
	wg        sync.WaitGroup
	mu        sync.RWMutex
	isRunning bool
	stopCh    chan struct{}
	mode      QueueMode
	ctx       context.Context
	cancel    context.CancelFunc
	keyPrefix string
}

// RedisQueueOption configures RedisQueue.
type RedisQueueOption func(*RedisQueue)

// WithKeyPrefix sets custom key prefix.
func WithKeyPrefix(prefix string) RedisQueueOption {
	return func(r *RedisQueue) {
		r.keyPrefix = prefix
	}
}

// NewRedisQueue creates a new Redis queue.
func NewRedisQueue(lgr *logger.Logger, config *QueueConfig, client *redis.Client, mode QueueMode, opts ...RedisQueueOption) *RedisQueue {
	if config == nil {
		config = &QueueConfig{}
	}
	if config.Workers <= 0 {
		config.Workers = 1
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	rq := &RedisQueue{
		logger:    lgr,
		config:    config,
		client:    client,
		jobs:      make(map[string]Job),
		stopCh:    make(chan struct{}),
		mode:      mode,
		ctx:       ctx,
		cancel:    cancel,
		keyPrefix: "finpull:queue",
	}

	for _, opt := range opts {
		opt(rq)
	}

	return rq
}

// NewRedisPublisher creates a publisher-only queue.
func NewRedisPublisher(lgr *logger.Logger, client *redis.Client, opts ...RedisQueueOption) *RedisQueue {
	q := NewRedisQueue(lgr, &QueueConfig{}, client, ModeProducerOnly, opts...)
	if err := q.Start(); err != nil {
		lgr.Error("redis publisher start failed", logger.Error(err))
	}
	return q
}

// NewRedisConsumer creates a consumer-only queue.
func NewRedisConsumer(lgr *logger.Logger, config *QueueConfig, client *redis.Client, jobs []Job, opts ...RedisQueueOption) *RedisQueue {
	q := NewRedisQueue(lgr, config, client, ModeConsumerOnly, opts...)
	if len(jobs) > 0 {
		q.RegisterJobs(jobs)
	}
	return q
}

// RegisterJobs registers multiple jobs.
func (r *RedisQueue) RegisterJobs(jobs []Job) {
	for _, job := range jobs {
		r.RegisterJob(job)
	}
}

// RegisterJob registers a single job.
func (r *RedisQueue) RegisterJob(job Job) {
	if r.mode == ModeProducerOnly {
		r.logger.Warn("job registration ignored in producer-only mode",
			logger.String("job", job.Name()))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[job.Type()]; exists {
		r.logger.Warn("job already registered", logger.String("job", job.Name()))
		return
	}

	r.jobs[job.Type()] = job
	r.logger.Info("job registered",
		logger.String("job", job.Name()),
		logger.String("type", job.Type()))
}

// Start starts the queue server.
func (r *RedisQueue) Start() error {
	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return fmt.Errorf("queue already running")
	}
	r.isRunning = true
	r.mu.Unlock()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.client.Ping(ctx).Err(); err != nil {
		r.isRunning = false
		return fmt.Errorf("redis ping: %w", err)
	}

	// Start workers for consumer modes
	if r.mode != ModeProducerOnly {
		for i := 0; i < r.config.Workers; i++ {
			r.wg.Add(1)
			go r.worker(i)
		}
		r.StartRetryProcessor()
		r.logger.Info("redis queue started",
			logger.Int("workers", r.config.Workers),
			logger.String("addr", r.client.Options().Addr),
			logger.String("mode", r.getModeString()))
	} else {
		r.logger.Info("redis publisher started",
			logger.String("addr", r.client.Options().Addr))
	}

	return nil
}

// Stop gracefully stops the queue.
func (r *RedisQueue) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.isRunning {
		r.mu.Unlock()
		return nil
	}
	r.isRunning = false
	r.logger.Info("stopping redis queue...")
	r.cancel()

	if r.mode != ModeProducerOnly {
		close(r.stopCh)
	}
	r.mu.Unlock()

	doneCh := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		r.logger.Warn("timeout waiting for queue workers", logger.Error(ctx.Err()))
		return fmt.Errorf("timeout: %w", ctx.Err())
	case <-doneCh:
		r.logger.Info("redis queue stopped gracefully")
		return nil
	}
}

// Enqueue adds a message to the queue.
func (r *RedisQueue) Enqueue(ctx context.Context, msgType string, payload interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isRunning {
		return fmt.Errorf("queue not running")
	}

	if r.mode != ModeProducerOnly {
		if _, exists := r.jobs[msgType]; !exists {
			return fmt.Errorf("no job registered for type: %s", msgType)
		}
	}

	msg := Message{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now(),
		Attempts:  0,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	if err := r.client.LPush(ctx, r.getQueueKey(), msgData).Err(); err != nil {
		return fmt.Errorf("lpush: %w", err)
	}

	return nil
}

// PublishMessage publishes a message (implements QueueService).
func (r *RedisQueue) PublishMessage(ctx context.Context, msgType string, payload interface{}) error {
	return r.Enqueue(ctx, msgType, payload)
}

func (r *RedisQueue) worker(id int) {
	defer r.wg.Done()
	r.logger.Info("queue worker started", logger.Int("worker_id", id))

	queueKey := r.getQueueKey()

	for {
		select {
		case <-r.stopCh:
			r.logger.Info("queue worker stopping", logger.Int("worker_id", id))
			return
		case <-r.ctx.Done():
			r.logger.Info("queue worker cancelled", logger.Int("worker_id", id))
			return
		default:
			r.processNextMessage(queueKey)
		}
	}
}

func (r *RedisQueue) processNextMessage(queueKey string) {
	ctx, cancel := context.WithTimeout(r.ctx, 1*time.Second)
	defer cancel()

	result, err := r.client.BRPop(ctx, 1*time.Second, queueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		r.logger.Error("brpop error", logger.Error(err))
		time.Sleep(1 * time.Second)
		return
	}

	if len(result) < 2 {
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
		r.logger.Error("unmarshal message", logger.Error(err))
		return
	}

	r.processMessage(msg)
}

func (r *RedisQueue) processMessage(msg Message) {
	job, exists := r.jobs[msg.Type]
	if !exists {
		r.logger.Error("no job found",
			logger.String("type", msg.Type),
			logger.String("id", msg.ID))
		return
	}

	payload := r.convertPayload(msg.Payload)
	start := time.Now()
	err := job.Handle(r.ctx, payload)
	elapsed := time.Since(start)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			r.logger.Warn("message cancelled",
				logger.String("id", msg.ID),
				logger.String("job", job.Name()),
				logger.Int64("elapsed_ms", elapsed.Milliseconds()))
			return
		}
		r.handleProcessingError(msg, job, err)
	}
}

func (r *RedisQueue) convertPayload(payload interface{}) interface{} {
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		return payload
	}

	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		r.logger.Error("convert payload", logger.Error(err))
		return payload
	}

	return json.RawMessage(jsonBytes)
}

func (r *RedisQueue) handleProcessingError(msg Message, job Job, err error) {
	r.logger.Error("message processing error",
		logger.String("id", msg.ID),
		logger.String("job", job.Name()),
		logger.Int("attempt", msg.Attempts+1),
		logger.Error(err))

	if msg.Attempts < r.config.RetryLimit {
		msg.Attempts++
		retryTime := time.Now().Add(r.config.RetryDelay)
		r.scheduleRetry(msg, retryTime)
		r.logger.Info("scheduled retry",
			logger.String("id", msg.ID),
			logger.String("job", job.Name()),
			logger.Int("attempt", msg.Attempts),
			logger.String("retry_at", retryTime.Format(time.RFC3339)))
	} else {
		r.logger.Error("max retries reached",
			logger.String("id", msg.ID),
			logger.String("job", job.Name()))
		r.moveToDeadLetterQueue(msg)
	}
}

func (r *RedisQueue) scheduleRetry(msg Message, retryTime time.Time) {
	msgData, err := json.Marshal(msg)
	if err != nil {
		r.logger.Error("marshal retry", logger.Error(err))
		return
	}

	err = r.client.ZAdd(context.Background(), r.getRetryKey(), redis.Z{
		Score:  float64(retryTime.Unix()),
		Member: msgData,
	}).Err()

	if err != nil {
		r.logger.Error("zadd retry", logger.Error(err))
	}
}

func (r *RedisQueue) moveToDeadLetterQueue(msg Message) {
	msgData, err := json.Marshal(msg)
	if err != nil {
		r.logger.Error("marshal dlq", logger.Error(err))
		return
	}

	if err := r.client.LPush(context.Background(), r.getDeadLetterKey(), msgData).Err(); err != nil {
		r.logger.Error("lpush dlq", logger.Error(err))
	}
}

// StartRetryProcessor starts retry processor goroutine.
func (r *RedisQueue) StartRetryProcessor() {
	if r.mode == ModeProducerOnly {
		return
	}

	r.wg.Add(1)
	go r.retryProcessor()
}

func (r *RedisQueue) retryProcessor() {
	defer r.wg.Done()
	r.logger.Info("retry processor started")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			r.logger.Info("retry processor stopping")
			return
		case <-r.ctx.Done():
			r.logger.Info("retry processor cancelled")
			return
		case <-ticker.C:
			r.processRetryMessages()
		}
	}
}

func (r *RedisQueue) processRetryMessages() {
	now := float64(time.Now().Unix())

	result, err := r.client.ZRangeByScoreWithScores(r.ctx, r.getRetryKey(), &redis.ZRangeBy{
		Min: "0",
		Max: strconv.FormatFloat(now, 'f', 0, 64),
	}).Result()

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		r.logger.Error("fetch retry messages", logger.Error(err))
		return
	}

	for _, z := range result {
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		msgData := z.Member.(string)

		pipe := r.client.TxPipeline()
		pipe.ZRem(r.ctx, r.getRetryKey(), msgData)
		pipe.LPush(r.ctx, r.getQueueKey(), msgData)

		if _, err := pipe.Exec(r.ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			r.logger.Error("move retry to queue", logger.Error(err))
		}
	}
}

func (r *RedisQueue) getModeString() string {
	switch r.mode {
	case ModeProducerOnly:
		return "producer-only"
	case ModeConsumerOnly:
		return "consumer-only"
	default:
		return "producer-consumer"
	}
}

func (r *RedisQueue) getQueueKey() string {
	return fmt.Sprintf("%s:messages", r.keyPrefix)
}

func (r *RedisQueue) getRetryKey() string {
	return fmt.Sprintf("%s:retry", r.keyPrefix)
}

func (r *RedisQueue) getDeadLetterKey() string {
	return fmt.Sprintf("%s:dlq", r.keyPrefix)
}
