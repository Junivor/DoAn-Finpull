package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
)

// Proc is the minimal processor interface the pipeline needs.
type Proc interface {
	Process(ctx context.Context, t *models.Trade) error
}

// RealtimePipeline is a middleware between WebSocket and Kafka.
// It validates, filters/throttles, optionally transforms, and buffers when downstream is unavailable.
type RealtimePipeline struct {
	proc     Proc
	metrics  domrepo.Metrics
	maxRPS   int
	bufSize  int
	bufCh    chan *models.Trade
	stopCh   chan struct{}
	started  bool
	mu       sync.Mutex
	lastSeen map[string]time.Time // per-symbol last accepted time
	// simple format transform hook (optional)
	transform func(*models.Trade) *models.Trade
	// metrics
	bufDepthGauge func(int)
	throttleWarn  func(string)
}

type PipelineOption func(*RealtimePipeline)

// WithMaxRPS sets the max trades per second per symbol.
func WithMaxRPS(n int) PipelineOption {
	return func(p *RealtimePipeline) {
		if n > 0 {
			p.maxRPS = n
		}
	}
}

// WithBufferSize sets the temporary buffer size when downstream is unavailable.
func WithBufferSize(n int) PipelineOption {
	return func(p *RealtimePipeline) {
		if n > 0 {
			p.bufSize = n
		}
	}
}

// NewRealtimePipeline creates a new pipeline.
func NewRealtimePipeline(proc Proc, metrics domrepo.Metrics, opts ...PipelineOption) *RealtimePipeline {
	p := &RealtimePipeline{
		proc:     proc,
		metrics:  metrics,
		maxRPS:   20,   // default throttle per symbol
		bufSize:  1000, // default buffer
		bufCh:    make(chan *models.Trade, 1000),
		stopCh:   make(chan struct{}),
		lastSeen: make(map[string]time.Time),
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.bufSize != cap(p.bufCh) {
		p.bufCh = make(chan *models.Trade, p.bufSize)
	}
	// metrics hooks using domain metrics if available
	p.bufDepthGauge = func(n int) { p.metrics.RecordLatency("pipeline_buffer_depth", float64(n)) }
	p.throttleWarn = func(sym string) { p.metrics.RecordError("pipeline_throttle_" + sym) }
	return p
}

// Start launches background flushing of buffered trades.
func (p *RealtimePipeline) Start(ctx context.Context) {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	p.mu.Unlock()

	go func() {
		backoff := 50 * time.Millisecond
		for {
			select {
			case <-p.stopCh:
				return
			case t := <-p.bufCh:
				if t == nil {
					continue
				}
				if err := p.proc.Process(ctx, t); err != nil {
					// exponential backoff with cap
					if backoff < 2*time.Second {
						backoff *= 2
					}
					p.metrics.RecordError("pipeline_flush")
					time.Sleep(backoff)
					// requeue if space; drop otherwise
					select {
					case p.bufCh <- t:
					default:
						p.metrics.RecordError("pipeline_buffer_drop")
					}
				} else {
					backoff = 50 * time.Millisecond
				}
			}
		}
	}()
}

// Stop stops the background flushing.
func (p *RealtimePipeline) Stop() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	p.started = false
	p.mu.Unlock()
	close(p.stopCh)
}

// Process validates, throttles, and forwards trade to downstream, buffering on errors.
func (p *RealtimePipeline) Process(ctx context.Context, t *models.Trade) error {
	start := time.Now()
	if err := validateTrade(t); err != nil {
		p.metrics.RecordError("pipeline_validate")
		return err
	}
	if p.transform != nil {
		t = p.transform(t)
		if err := validateTrade(t); err != nil {
			p.metrics.RecordError("pipeline_transform_invalid")
			return err
		}
	}
	if !p.allow(t.Symbol, start) {
		// throttled; record and drop silently
		p.metrics.RecordError("pipeline_throttle")
		if p.throttleWarn != nil {
			p.throttleWarn(t.Symbol)
		}
		return nil
	}

	if err := p.proc.Process(ctx, t); err != nil {
		p.metrics.RecordError("pipeline_process")
		// buffer non-blocking
		select {
		case p.bufCh <- t:
			if p.bufDepthGauge != nil {
				p.bufDepthGauge(len(p.bufCh))
			}
		default:
			p.metrics.RecordError("pipeline_buffer_full")
		}
		return fmt.Errorf("pipeline downstream: %w", err)
	}
	p.metrics.RecordLatency("pipeline_process", time.Since(start).Seconds())
	return nil
}

// WithTransform sets a transformation hook to modify trade format.
func WithTransform(fn func(*models.Trade) *models.Trade) PipelineOption {
	return func(p *RealtimePipeline) { p.transform = fn }
}

func validateTrade(t *models.Trade) error {
	if t == nil {
		return fmt.Errorf("trade nil")
	}
	if t.Symbol == "" {
		return fmt.Errorf("symbol empty")
	}
	if t.Timestamp <= 0 {
		return fmt.Errorf("timestamp invalid")
	}
	if t.Price < 0 || t.Volume < 0 {
		return fmt.Errorf("negative price/volume")
	}
	return nil
}

func (p *RealtimePipeline) allow(symbol string, now time.Time) bool {
	if p.maxRPS <= 0 {
		return true
	}
	// simple throttle: ensure at most maxRPS per second
	last := p.lastSeen[symbol]
	if last.IsZero() {
		p.lastSeen[symbol] = now
		return true
	}
	// compute elapsed trades per second window
	if now.Sub(last) < time.Second/time.Duration(p.maxRPS) {
		return false
	}
	p.lastSeen[symbol] = now
	return true
}
