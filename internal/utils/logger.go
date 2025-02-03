package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// Log levels
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Logger provides structured logging capabilities
type Logger struct {
	level     LogLevel
	outputs   []io.Writer
	prefix    string
	timeFormat string
	mu        sync.Mutex
	fields    map[string]interface{}
}

// LoggerOption configures the logger
type LoggerOption func(*Logger)

// LogEntry represents a single log entry
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Fields  map[string]interface{}
	Caller  string
}

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

// NewLogger creates a new logger instance
func NewLogger(opts ...LoggerOption) *Logger {
	l := &Logger{
		level:      INFO,
		outputs:    []io.Writer{os.Stdout},
		timeFormat: "2006-01-02 15:04:05.000",
		fields:     make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// WithLevel sets the log level
func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

// WithOutput adds an output writer
func WithOutput(w io.Writer) LoggerOption {
	return func(l *Logger) {
		l.outputs = append(l.outputs, w)
	}
}

// WithPrefix sets the logger prefix
func WithPrefix(prefix string) LoggerOption {
	return func(l *Logger) {
		l.prefix = prefix
	}
}

// WithField adds a field to all log entries
func WithField(key string, value interface{}) LoggerOption {
	return func(l *Logger) {
		l.fields[key] = value
	}
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// AddOutput adds an additional output writer
func (l *Logger) AddOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, w)
}

// WithFields creates a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := &Logger{
		level:      l.level,
		outputs:    l.outputs,
		prefix:     l.prefix,
		timeFormat: l.timeFormat,
		fields:     make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// log handles the actual logging
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log entry
	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
		Fields:  make(map[string]interface{}),
		Caller:  l.getCaller(),
	}

	// Add logger fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Add additional fields
	for k, v := range fields {
		entry.Fields[k] = v
	}

	// Format and write the log entry
	formattedLog := l.formatLogEntry(entry)
	for _, output := range l.outputs {
		fmt.Fprintln(output, formattedLog)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
}

// formatLogEntry formats a log entry for output
func (l *Logger) formatLogEntry(entry LogEntry) string {
	var color string
	var level string

	switch entry.Level {
	case DEBUG:
		color = colorBlue
		level = "DEBUG"
	case INFO:
		color = colorGreen
		level = "INFO "
	case WARN:
		color = colorYellow
		level = "WARN "
	case ERROR:
		color = colorRed
		level = "ERROR"
	case FATAL:
		color = colorRed
		level = "FATAL"
	}

	// Build the log message
	var builder strings.Builder

	// Add timestamp
	builder.WriteString(entry.Time.Format(l.timeFormat))
	builder.WriteString(" ")

	// Add colored level
	builder.WriteString(color)
	builder.WriteString(level)
	builder.WriteString(colorReset)
	builder.WriteString(" ")

	// Add prefix if set
	if l.prefix != "" {
		builder.WriteString("[")
		builder.WriteString(l.prefix)
		builder.WriteString("] ")
	}

	// Add caller information
	builder.WriteString(entry.Caller)
	builder.WriteString(" ")

	// Add message
	builder.WriteString(entry.Message)

	// Add fields if any
	if len(entry.Fields) > 0 {
		builder.WriteString(" ")
		first := true
		for k, v := range entry.Fields {
			if !first {
				builder.WriteString(", ")
			}
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(fmt.Sprint(v))
			first = false
		}
	}

	return builder.String()
}

// getCaller returns the caller information
func (l *Logger) getCaller() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "???"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

// String representations of log levels
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", l)
	}
}