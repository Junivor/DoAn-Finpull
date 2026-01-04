package logger

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Publisher interface {
	PublishMessage(ctx context.Context, topic string, payload interface{}) error
}

type CollectionConfig struct {
	TimeInterval   time.Duration // flush interval (e.g., 30s)
	CountThreshold int           // max unique logs before flush (e.g., 100)
	Topic          string        // topic to send aggregated logs
	Publisher      Publisher     // interface to send aggregated logs
}

type AggregatedLogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
	Caller    string                 `json:"caller"`
	Count     int                    `json:"count"`
	FirstSeen time.Time              `json:"first_seen"`
	LastSeen  time.Time              `json:"last_seen"`
}

type LogCollector struct {
	config *CollectionConfig
	logMap map[string]*AggregatedLogEntry
	mutex  sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewLogCollector(config *CollectionConfig) *LogCollector {
	ctx, cancel := context.WithCancel(context.Background())

	collector := &LogCollector{
		config: config,
		logMap: make(map[string]*AggregatedLogEntry),
		ctx:    ctx,
		cancel: cancel,
	}

	// Start periodic flush goroutine
	collector.wg.Add(1)
	go collector.periodicFlush()

	return collector
}

func (d *LogCollector) AddLog(level, message string, fields map[string]interface{}, caller string) {
	now := time.Now()
	key := d.generateKey(level, message, fields, caller)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if entry, exists := d.logMap[key]; exists {
		// Update existing entry
		entry.Count++
		entry.LastSeen = now
	} else {
		// Create new entry
		d.logMap[key] = &AggregatedLogEntry{
			Level:     level,
			Message:   message,
			Fields:    fields,
			Caller:    caller,
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
	}

	// Check count threshold
	if len(d.logMap) >= d.config.CountThreshold {
		d.flushLogs()
	}
}

func (d *LogCollector) generateKey(level, message string, fields map[string]interface{}, caller string) string {
	// Create a consistent hash from level + message + fields + caller
	data := struct {
		Level   string                 `json:"level"`
		Message string                 `json:"message"`
		Fields  map[string]interface{} `json:"fields"`
		Caller  string                 `json:"caller"`
	}{
		Level:   level,
		Message: message,
		Fields:  fields,
		Caller:  caller,
	}

	jsonData, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash)
}

func (d *LogCollector) periodicFlush() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.TimeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.mutex.Lock()
			if len(d.logMap) > 0 {
				d.flushLogs()
			}
			d.mutex.Unlock()
		case <-d.ctx.Done():
			// Final flush before shutdown
			d.mutex.Lock()
			if len(d.logMap) > 0 {
				d.flushLogs()
			}
			d.mutex.Unlock()
			return
		}
	}
}

func (d *LogCollector) flushLogs() {
	if len(d.logMap) == 0 {
		return
	}

	// Create slice of aggregated logs
	logs := make([]AggregatedLogEntry, 0, len(d.logMap))
	for _, entry := range d.logMap {
		logs = append(logs, *entry)
	}

	// Reset the map
	d.logMap = make(map[string]*AggregatedLogEntry)

	// Send logs in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := d.config.Publisher.PublishMessage(ctx, d.config.Topic, logs); err != nil {
			fmt.Printf("Failed to send aggregated logs: %v\n", err)
		}
	}()
}

func (d *LogCollector) Close() {
	d.cancel()
	d.wg.Wait()
}
