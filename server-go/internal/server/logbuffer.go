package server

import (
	"buzzcontrol/internal/game"
	"sync"
)

const (
	// DefaultLogBufferCapacity is the default capacity for the log buffer
	DefaultLogBufferCapacity = 1000
)

// LogBuffer is a thread-safe circular buffer for log entries
type LogBuffer struct {
	entries  []game.LogEntry
	capacity int
	head     int  // Index of the oldest entry
	size     int  // Current number of entries
	mu       sync.RWMutex
}

// NewLogBuffer creates a new LogBuffer with the specified capacity
func NewLogBuffer(capacity int) *LogBuffer {
	if capacity <= 0 {
		capacity = DefaultLogBufferCapacity
	}
	return &LogBuffer{
		entries:  make([]game.LogEntry, capacity),
		capacity: capacity,
		head:     0,
		size:     0,
	}
}

// Add adds a new log entry to the buffer
// If the buffer is full, the oldest entry is overwritten
func (lb *LogBuffer) Add(entry game.LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Calculate the index where the new entry should be written
	writeIndex := (lb.head + lb.size) % lb.capacity

	if lb.size < lb.capacity {
		// Buffer not full yet
		lb.entries[writeIndex] = entry
		lb.size++
	} else {
		// Buffer is full, overwrite oldest entry
		lb.entries[lb.head] = entry
		lb.head = (lb.head + 1) % lb.capacity
	}
}

// GetAll returns all log entries in chronological order (oldest first)
func (lb *LogBuffer) GetAll() []game.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]game.LogEntry, lb.size)
	for i := 0; i < lb.size; i++ {
		result[i] = lb.entries[(lb.head+i)%lb.capacity]
	}
	return result
}

// GetRecent returns the most recent n log entries in chronological order
func (lb *LogBuffer) GetRecent(n int) []game.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n <= 0 {
		return []game.LogEntry{}
	}
	if n > lb.size {
		n = lb.size
	}

	result := make([]game.LogEntry, n)
	startIndex := lb.size - n
	for i := 0; i < n; i++ {
		result[i] = lb.entries[(lb.head+startIndex+i)%lb.capacity]
	}
	return result
}

// Size returns the current number of entries in the buffer
func (lb *LogBuffer) Size() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.size
}

// Clear removes all entries from the buffer
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.head = 0
	lb.size = 0
}

// Capacity returns the maximum capacity of the buffer
func (lb *LogBuffer) Capacity() int {
	return lb.capacity
}
