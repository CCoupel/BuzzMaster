package server

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func init() {
	// The engine cannot import this package; it reports the ARDOISE invariant
	// (#246) through this hook, routed to the global logger at ERROR level.
	game.ArdoiseDivergenceReporter = func(msg string) {
		LogError(game.LogComponentEngine, "%s", msg)
	}
}

// BroadcastLogger captures logs and broadcasts them to subscribed WebSocket clients
type BroadcastLogger struct {
	buffer      *LogBuffer
	onNewEntry  func(entry game.LogEntry) // Callback when a new entry is added
	debugEnabled bool
	file        *fileSink // persistent log file (#246), nil when disabled
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

	// Persistent file (#246, contracts/logging.md §1)
	if fs := bl.file; fs != nil {
		fs.write(entry)
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

// --- Persistent log file (#246, contracts/logging.md §1) -------------------------

func levelRank(l game.LogLevel) int {
	switch l {
	case game.LogLevelDebug:
		return 0
	case game.LogLevelInfo:
		return 1
	case game.LogLevelWarn:
		return 2
	case game.LogLevelError:
		return 3
	}
	return 1
}

// fileSink writes entries at or above minLevel to a size-rotated file. Writes
// are synchronous. A disk error never stops the logger: at most one stdout
// ERROR per minute, entries keep flowing to memory.
type fileSink struct {
	mu       sync.Mutex
	path     string
	minLevel game.LogLevel
	maxBytes int64
	maxFiles int
	f        *os.File
	size     int64
	lastErr  time.Time
}

func (fs *fileSink) reportErr(err error) {
	if time.Since(fs.lastErr) >= time.Minute {
		fs.lastErr = time.Now()
		log.Printf("[Logger][ERROR] log file %s: %v (continuing in memory)", fs.path, err)
	}
}

func (fs *fileSink) open() error {
	if err := os.MkdirAll(filepath.Dir(fs.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(fs.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	fs.f, fs.size = f, st.Size()
	return nil
}

func (fs *fileSink) rotate() error {
	if fs.f != nil {
		fs.f.Close()
		fs.f = nil
	}
	// base, base.1 ... base.(maxFiles-1): drop the oldest, shift the others.
	_ = os.Remove(fmt.Sprintf("%s.%d", fs.path, fs.maxFiles-1))
	for i := fs.maxFiles - 2; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", fs.path, i), fmt.Sprintf("%s.%d", fs.path, i+1))
	}
	if fs.maxFiles > 1 {
		_ = os.Rename(fs.path, fs.path+".1")
	} else {
		_ = os.Remove(fs.path)
	}
	return fs.open()
}

func (fs *fileSink) write(entry game.LogEntry) {
	if levelRank(entry.Level) < levelRank(fs.minLevel) {
		return
	}
	line := fmt.Sprintf("%s [%s] [%s] %s\n",
		time.UnixMilli(entry.Timestamp).Format("2006-01-02T15:04:05.000Z07:00"),
		entry.Level, entry.Component, strings.TrimRight(entry.Message, "\r\n"))
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.f == nil {
		if err := fs.open(); err != nil {
			fs.reportErr(err)
			return
		}
	}
	if fs.size > 0 && fs.size+int64(len(line)) > fs.maxBytes {
		if err := fs.rotate(); err != nil {
			fs.reportErr(err)
			return
		}
	}
	n, err := fs.f.WriteString(line)
	fs.size += int64(n)
	if err != nil {
		fs.reportErr(err)
	}
}

// SetFileOutput enables the persistent log file from the `logging` config
// section. Disabled -> previous behaviour (memory + stdout). The returned error
// (directory not creatable, ...) is informative only: the logger keeps working.
func (bl *BroadcastLogger) SetFileOutput(cfg config.LoggingConfig) error {
	if !cfg.FileEnabled {
		bl.file = nil
		return nil
	}
	d := config.DefaultLoggingConfig()
	if cfg.FilePath == "" {
		cfg.FilePath = d.FilePath
	}
	if cfg.FileMaxSizeMB <= 0 {
		cfg.FileMaxSizeMB = d.FileMaxSizeMB
	}
	if cfg.FileMaxFiles <= 0 {
		cfg.FileMaxFiles = d.FileMaxFiles
	}
	min := game.LogLevel(strings.ToUpper(cfg.FileMinLevel))
	if levelRank(min) == 1 && min != game.LogLevelInfo {
		min = game.LogLevelWarn
	}
	fs := &fileSink{
		path: cfg.FilePath, minLevel: min,
		maxBytes: int64(cfg.FileMaxSizeMB) * 1024 * 1024, maxFiles: cfg.FileMaxFiles,
	}
	bl.file = fs
	fs.mu.Lock()
	err := fs.open()
	fs.mu.Unlock()
	if err != nil {
		fs.reportErr(err)
	}
	return err
}
