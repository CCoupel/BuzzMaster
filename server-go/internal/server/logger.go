package server

import (
	"buzzcontrol/internal/game"
	"fmt"
	"log"
	"sync"
	"time"
)

// BroadcastLogger captures logs and broadcasts them to subscribed WebSocket clients
type BroadcastLogger struct {
	buffer      *LogBuffer
	onNewEntry  func(entry game.LogEntry) // Callback when a new entry is added
	debugEnabled bool
}

// NewBroadcastLogger creates a new BroadcastLogger with the specified buffer capacity
func NewBroadcastLogger(capacity int) *BroadcastLogger {
	return &BroadcastLogger{
		buffer:      NewLogBuffer(capacity),
		debugEnabled: false,
	}
}

// SetOnNewEntry sets the callback function called when a new log entry is added
func (bl *BroadcastLogger) SetOnNewEntry(callback func(entry game.LogEntry)) {
	bl.onNewEntry = callback
}

// SetDebugEnabled enables or disables DEBUG level logging
func (bl *BroadcastLogger) SetDebugEnabled(enabled bool) {
	bl.debugEnabled = enabled
}

// log is the internal method that adds an entry to the buffer and notifies subscribers
func (bl *BroadcastLogger) log(level game.LogLevel, component game.LogComponent, format string, args ...interface{}) {
	// Skip DEBUG logs if debug is not enabled
	if level == game.LogLevelDebug && !bl.debugEnabled {
		return
	}

	message := fmt.Sprintf(format, args...)
	entry := game.LogEntry{
		Timestamp: time.Now().UnixMilli(),
		Level:     level,
		Component: component,
		Message:   message,
	}

	// Add to buffer
	bl.buffer.Add(entry)

	// Notify callback if set
	if bl.onNewEntry != nil {
		bl.onNewEntry(entry)
	}

	// Also log to standard logger with prefix
	prefix := fmt.Sprintf("[%s][%s]", component, level)
	log.Printf("%s %s", prefix, message)
}

// Debug logs a debug message
func (bl *BroadcastLogger) Debug(component game.LogComponent, format string, args ...interface{}) {
	bl.log(game.LogLevelDebug, component, format, args...)
}

// Info logs an info message
func (bl *BroadcastLogger) Info(component game.LogComponent, format string, args ...interface{}) {
	bl.log(game.LogLevelInfo, component, format, args...)
}

// Warn logs a warning message
func (bl *BroadcastLogger) Warn(component game.LogComponent, format string, args ...interface{}) {
	bl.log(game.LogLevelWarn, component, format, args...)
}

// Error logs an error message
func (bl *BroadcastLogger) Error(component game.LogComponent, format string, args ...interface{}) {
	bl.log(game.LogLevelError, component, format, args...)
}

// GetBuffer returns the log buffer
func (bl *BroadcastLogger) GetBuffer() *LogBuffer {
	return bl.buffer
}

// GetHistory returns all log entries
func (bl *BroadcastLogger) GetHistory() []game.LogEntry {
	return bl.buffer.GetAll()
}

// GetRecent returns the most recent n log entries
func (bl *BroadcastLogger) GetRecent(n int) []game.LogEntry {
	return bl.buffer.GetRecent(n)
}

// InitLogger creates a new BroadcastLogger and sets it as the global logger
func InitLogger(capacity int) *BroadcastLogger {
	bl := NewBroadcastLogger(capacity)
	SetGlobalLogger(bl)
	return bl
}

// Global logger instance for package-level functions
var globalLogger *BroadcastLogger
var globalLoggerMu sync.RWMutex

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(bl *BroadcastLogger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	globalLogger = bl
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *BroadcastLogger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	return globalLogger
}

// Package-level logging functions that use the global BroadcastLogger
// If no global logger is set, falls back to standard log.Printf

// LogDebug logs a debug message at package level
func LogDebug(component game.LogComponent, format string, args ...interface{}) {
	if logger := GetGlobalLogger(); logger != nil {
		logger.Debug(component, format, args...)
	} else {
		message := fmt.Sprintf(format, args...)
		log.Printf("[%s][DEBUG] %s", component, message)
	}
}

// LogInfo logs an info message at package level
func LogInfo(component game.LogComponent, format string, args ...interface{}) {
	if logger := GetGlobalLogger(); logger != nil {
		logger.Info(component, format, args...)
	} else {
		message := fmt.Sprintf(format, args...)
		log.Printf("[%s][INFO] %s", component, message)
	}
}

// LogWarn logs a warning message at package level
func LogWarn(component game.LogComponent, format string, args ...interface{}) {
	if logger := GetGlobalLogger(); logger != nil {
		logger.Warn(component, format, args...)
	} else {
		message := fmt.Sprintf(format, args...)
		log.Printf("[%s][WARN] %s", component, message)
	}
}

// LogError logs an error message at package level
func LogError(component game.LogComponent, format string, args ...interface{}) {
	if logger := GetGlobalLogger(); logger != nil {
		logger.Error(component, format, args...)
	} else {
		message := fmt.Sprintf(format, args...)
		log.Printf("[%s][ERROR] %s", component, message)
	}
}
