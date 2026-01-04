package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Logger struct {
	zl        zerolog.Logger
	collector *LogCollector
}

type Config struct {
	Level      string // debug, info, warn, error, fatal, panic
	Format     string // json or console
	Output     string // stdout, stderr, or file path
	TimeFormat string // time format for log messages
}

func New(cfg *Config) (*Logger, error) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	zerolog.SetGlobalLevel(level)

	// Configure output writer
	var output io.Writer
	switch cfg.Output {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("could not open log file: %w", err)
		}
		output = file
	}

	// Configure time format (ensure it's not empty)
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = time.RFC3339Nano
	}
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// If format is "console", use human-readable, otherwise use JSON
	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: cfg.TimeFormat,
			NoColor:    false,
		}
	}

	// Create logger instance
	logger := zerolog.New(output).
		With().
		Timestamp().
		CallerWithSkipFrameCount(3).
		Logger()

	return &Logger{zl: logger}, nil
}

func (l *Logger) addToCollector(level, msg string, fields []Field) {
	if l.collector == nil {
		return
	}

	// Get caller info (skip frames: this function -> Error/Warn -> user code)
	_, file, line, ok := runtime.Caller(2)
	var caller string
	if ok {
		parts := strings.Split(file, "FinPull")
		filename := parts[len(parts)-1]
		caller = fmt.Sprintf("%s:%d", filename, line)
	} else {
		caller = "unknown"
	}

	// Convert fields to map
	fieldMap := make(map[string]interface{})
	for _, field := range fields {
		key, value := field.GetKeyValue()
		fieldMap[key] = value
	}

	l.collector.AddLog(level, msg, fieldMap, caller)
}

// --- Logger methods ---

func (l *Logger) Info(msg string, fields ...Field) {
	event := l.zl.Info()
	for _, field := range fields {
		field.AddTo(event)
	}
	event.Msg(msg)
}

func (l *Logger) Error(msg string, fields ...Field) {
	event := l.zl.Error()
	for _, field := range fields {
		field.AddTo(event)
	}
	event.Msg(msg)

	// Add to collector
	l.addToCollector("error", msg, fields)
}

func (l *Logger) Debug(msg string, fields ...Field) {
	event := l.zl.Debug()
	for _, field := range fields {
		field.AddTo(event)
	}
	event.Msg(msg)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	event := l.zl.Warn()
	for _, field := range fields {
		field.AddTo(event)
	}
	event.Msg(msg)

	// Add to collector
	//l.addToCollector("warn", msg, fields)
}

func (l *Logger) AddCollector(config *CollectionConfig) {
	if l.collector != nil {
		l.collector.Close() // Close existing collector if any
	}
	l.collector = NewLogCollector(config)
}

func (l *Logger) RemoveCollector() {
	if l.collector != nil {
		l.collector.Close()
	}
}

// Field types for structured logging.
type Field interface {
	AddTo(event *zerolog.Event)
	GetKeyValue() (string, interface{})
}

type StringField struct {
	Key   string
	Value string
}

func (f StringField) AddTo(event *zerolog.Event) {
	event.Str(f.Key, f.Value)
}

func (f StringField) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value
}

type IntField struct {
	Key   string
	Value int
}

func (f IntField) AddTo(event *zerolog.Event) {
	event.Int(f.Key, f.Value)
}

func (f IntField) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value
}

type Int64Field struct {
	Key   string
	Value int64
}

func (f Int64Field) AddTo(event *zerolog.Event) {
	event.Int64(f.Key, f.Value)
}

func (f Int64Field) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value
}

type ErrorField struct {
	Key   string
	Value error
}

func (f ErrorField) AddTo(event *zerolog.Event) {
	event.Err(f.Value)
}

func (f ErrorField) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value.Error()
}

type AnyField struct {
	Key   string
	Value interface{}
}

func (f AnyField) AddTo(event *zerolog.Event) {
	event.Interface(f.Key, f.Value)
}

func (f AnyField) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value
}

type BoolField struct {
	Key   string
	Value bool
}

func (f BoolField) AddTo(event *zerolog.Event) {
	event.Bool(f.Key, f.Value)
}

func (f BoolField) GetKeyValue() (string, interface{}) {
	return f.Key, f.Value
}

// --- Field constructors ---

func String(key, value string) Field {
	return StringField{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return IntField{Key: key, Value: value}
}

func Error(err error) Field {
	return ErrorField{Key: "error", Value: err}
}

func Any(key string, value interface{}) Field {
	return AnyField{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return IntField{Key: key, Value: int(value / time.Millisecond)}
}

func Int32(key string, value int32) Field {
	return IntField{Key: key, Value: int(value)}
}

func Int64(key string, value int64) Field {
	return Int64Field{Key: key, Value: value}
}

func Strings(key string, value []string) Field {
	return String(key, strings.Join(value, ", "))
}

func Uint(key string, value uint) Field {
	return IntField{Key: key, Value: int(value)}
}

func Uint64(key string, value uint64) Field {
	return Int64Field{Key: key, Value: int64(value)}
}

func Bool(s string, v bool) Field {
	return BoolField{Key: s, Value: v}
}
