package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds logger configuration
type LogConfig struct {
	Level      string
	Format     string
	OutputPath string
	MaxSize    int  // megabytes
	MaxAge     int  // days
	MaxBackups int  // files
	Compress   bool // compress rotated files
	Development bool
	EnableCaller bool
	EnableStacktrace bool
	SamplingInitial int
	SamplingThereafter int
}

// Logger wraps zap logger with additional functionality
type Logger struct {
	*zap.Logger
	config     *LogConfig
	fields     map[string]interface{}
	mu         sync.RWMutex
	fileLogger *lumberjack.Logger
}

// DefaultConfig returns default logger configuration
func DefaultConfig() *LogConfig {
	return &LogConfig{
		Level:              "info",
		Format:             "json",
		OutputPath:         "logs/app.log",
		MaxSize:            100,
		MaxAge:            30,
		MaxBackups:        5,
		Compress:          true,
		Development:       false,
		EnableCaller:      true,
		EnableStacktrace:  true,
		SamplingInitial:   100,
		SamplingThereafter: 100,
	}
}

// NewLogger creates a new logger instance
func NewLogger(config *LogConfig) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(config.OutputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Initialize file logger for rotation
	fileLogger := &lumberjack.Logger{
		Filename:   config.OutputPath,
		MaxSize:    config.MaxSize,
		MaxAge:     config.MaxAge,
		MaxBackups: config.MaxBackups,
		Compress:   config.Compress,
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Configure log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %v", err)
	}

	// Create cores
	var core zapcore.Core
	if config.Format == "json" {
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(os.Stdout),
				zapcore.AddSync(fileLogger),
			),
			level,
		)
	} else {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(
				zapcore.AddSync(os.Stdout),
				zapcore.AddSync(fileLogger),
			),
			level,
		)
	}

	// Create logger
	zapLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Development(),
	)

	return &Logger{
		Logger:     zapLogger,
		config:     config,
		fields:     make(map[string]interface{}),
		fileLogger: fileLogger,
	}, nil
}

// WithFields adds fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		Logger:     l.Logger,
		config:     l.config,
		fields:     make(map[string]interface{}),
		fileLogger: l.fileLogger,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	// Create zap fields
	zapFields := make([]zap.Field, 0, len(newLogger.fields))
	for k, v := range newLogger.fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	newLogger.Logger = l.Logger.With(zapFields...)
	return newLogger
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.Logger.Debug(msg, l.convertFields(fields...)...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.Logger.Info(msg, l.convertFields(fields...)...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.Logger.Warn(msg, l.convertFields(fields...)...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.Logger.Error(msg, l.convertFields(fields...)...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...map[string]interface{}) {
	l.Logger.Fatal(msg, l.convertFields(fields...)...)
}

// convertFields converts map fields to zap fields
func (l *Logger) convertFields(fields ...map[string]interface{}) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zapFields := make([]zap.Field, 0)
	for _, fieldMap := range fields {
		for k, v := range fieldMap {
			zapFields = append(zapFields, zap.Any(k, v))
		}
	}
	return zapFields
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Close closes the logger and cleans up resources
func (l *Logger) Close() error {
	if err := l.Sync(); err != nil {
		return fmt.Errorf("failed to sync logger: %v", err)
	}
	return nil
}

// GetLogLevel returns the current log level
func (l *Logger) GetLogLevel() string {
	return l.config.Level
}

// SetLogLevel sets the log level
func (l *Logger) SetLogLevel(level string) error {
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %v", err)
	}

	l.config.Level = level
	l.Logger = l.Logger.WithOptions(zap.IncreaseLevel(parsedLevel))
	return nil
}

// AddCallerSkip increases the number of callers skipped by caller annotation
func (l *Logger) AddCallerSkip(skip int) *Logger {
	l.Logger = l.Logger.WithOptions(zap.AddCallerSkip(skip))
	return l
}

// WithName adds a sub-scope to the logger's name
func (l *Logger) WithName(name string) *Logger {
	l.Logger = l.Logger.Named(name)
	return l
}

// Development sets the logger to development mode
func (l *Logger) Development() *Logger {
	l.config.Development = true
	l.Logger = l.Logger.WithOptions(zap.Development())
	return l
}

// Production sets the logger to production mode
func (l *Logger) Production() *Logger {
	l.config.Development = false
	l.Logger = l.Logger.WithOptions(zap.Production())
	return l
}