package ratelimit

import (
    "sync"
    "time"
)

type bucket struct {
    tokens     float64
    capacity   float64
    refillRate float64 // tokens per second
    last       time.Time
}

type Limiter struct {
    mu sync.Mutex
    m  map[string]*bucket
}

func New() *Limiter { return &Limiter{m: make(map[string]*bucket)} }

// Allow returns true if one token can be consumed for key.
func (l *Limiter) Allow(key string, capacity, refillPerSec float64) bool {
    now := time.Now()
    l.mu.Lock()
    b, ok := l.m[key]
    if !ok {
        b = &bucket{tokens: capacity, capacity: capacity, refillRate: refillPerSec, last: now}
        l.m[key] = b
    }
    // refill
    elapsed := now.Sub(b.last).Seconds()
    if elapsed > 0 {
        b.tokens += elapsed * b.refillRate
        if b.tokens > b.capacity { b.tokens = b.capacity }
        b.last = now
    }
    if b.tokens >= 1 {
        b.tokens -= 1
        l.mu.Unlock()
        return true
    }
    l.mu.Unlock()
    return false
}


