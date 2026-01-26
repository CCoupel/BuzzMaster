package server

import (
	"buzzcontrol/internal/game"
	"sync"
	"testing"
	"time"
)

func TestNewLogBuffer(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		want     int
	}{
		{"positive capacity", 100, 100},
		{"zero capacity uses default", 0, DefaultLogBufferCapacity},
		{"negative capacity uses default", -1, DefaultLogBufferCapacity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLogBuffer(tt.capacity)
			if lb.Capacity() != tt.want {
				t.Errorf("NewLogBuffer(%d).Capacity() = %d, want %d", tt.capacity, lb.Capacity(), tt.want)
			}
			if lb.Size() != 0 {
				t.Errorf("NewLogBuffer(%d).Size() = %d, want 0", tt.capacity, lb.Size())
			}
		})
	}
}

func TestLogBuffer_Add(t *testing.T) {
	lb := NewLogBuffer(5)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "test",
		})
	}

	if lb.Size() != 3 {
		t.Errorf("Size() = %d, want 3", lb.Size())
	}
}

func TestLogBuffer_Circular(t *testing.T) {
	lb := NewLogBuffer(3)

	// Add 5 entries to a buffer of 3
	for i := 0; i < 5; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i * 100),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "msg",
		})
	}

	// Size should be 3 (capacity limit)
	if lb.Size() != 3 {
		t.Errorf("Size() = %d, want 3", lb.Size())
	}

	// Should contain entries 2, 3, 4 (timestamps 200, 300, 400)
	entries := lb.GetAll()
	if len(entries) != 3 {
		t.Fatalf("GetAll() returned %d entries, want 3", len(entries))
	}

	expectedTimestamps := []int64{200, 300, 400}
	for i, entry := range entries {
		if entry.Timestamp != expectedTimestamps[i] {
			t.Errorf("entries[%d].Timestamp = %d, want %d", i, entry.Timestamp, expectedTimestamps[i])
		}
	}
}

func TestLogBuffer_GetAll_Empty(t *testing.T) {
	lb := NewLogBuffer(5)
	entries := lb.GetAll()

	if len(entries) != 0 {
		t.Errorf("GetAll() on empty buffer returned %d entries, want 0", len(entries))
	}
}

func TestLogBuffer_GetAll_Order(t *testing.T) {
	lb := NewLogBuffer(5)

	// Add entries in order
	for i := 0; i < 5; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "test",
		})
	}

	entries := lb.GetAll()

	// Verify chronological order (oldest first)
	for i, entry := range entries {
		if entry.Timestamp != int64(i) {
			t.Errorf("entries[%d].Timestamp = %d, want %d", i, entry.Timestamp, i)
		}
	}
}

func TestLogBuffer_GetRecent(t *testing.T) {
	lb := NewLogBuffer(10)

	// Add 7 entries
	for i := 0; i < 7; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i * 100),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "test",
		})
	}

	tests := []struct {
		name       string
		n          int
		wantLen    int
		wantFirst  int64
		wantLast   int64
	}{
		{"get last 3", 3, 3, 400, 600},
		{"get last 5", 5, 5, 200, 600},
		{"get more than available", 10, 7, 0, 600},
		{"get zero", 0, 0, 0, 0},
		{"get negative", -1, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := lb.GetRecent(tt.n)
			if len(entries) != tt.wantLen {
				t.Errorf("GetRecent(%d) returned %d entries, want %d", tt.n, len(entries), tt.wantLen)
			}
			if tt.wantLen > 0 {
				if entries[0].Timestamp != tt.wantFirst {
					t.Errorf("GetRecent(%d)[0].Timestamp = %d, want %d", tt.n, entries[0].Timestamp, tt.wantFirst)
				}
				if entries[len(entries)-1].Timestamp != tt.wantLast {
					t.Errorf("GetRecent(%d)[last].Timestamp = %d, want %d", tt.n, entries[len(entries)-1].Timestamp, tt.wantLast)
				}
			}
		})
	}
}

func TestLogBuffer_Clear(t *testing.T) {
	lb := NewLogBuffer(5)

	// Add some entries
	for i := 0; i < 3; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "test",
		})
	}

	if lb.Size() != 3 {
		t.Fatalf("Size() before Clear() = %d, want 3", lb.Size())
	}

	lb.Clear()

	if lb.Size() != 0 {
		t.Errorf("Size() after Clear() = %d, want 0", lb.Size())
	}

	entries := lb.GetAll()
	if len(entries) != 0 {
		t.Errorf("GetAll() after Clear() returned %d entries, want 0", len(entries))
	}
}

func TestLogBuffer_Concurrency(t *testing.T) {
	lb := NewLogBuffer(1000)
	var wg sync.WaitGroup
	numGoroutines := 10
	entriesPerGoroutine := 100

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				lb.Add(game.LogEntry{
					Timestamp: time.Now().UnixMilli(),
					Level:     game.LogLevelInfo,
					Component: game.LogComponentApp,
					Message:   "concurrent test",
				})
			}
		}(g)
	}

	// Concurrent reads
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				_ = lb.GetAll()
				_ = lb.GetRecent(10)
				_ = lb.Size()
			}
		}()
	}

	wg.Wait()

	// Should have at most 1000 entries (capacity)
	if lb.Size() > 1000 {
		t.Errorf("Size() = %d, exceeds capacity 1000", lb.Size())
	}

	// Should have exactly 1000 entries (all goroutines wrote total of 1000 entries)
	expectedSize := numGoroutines * entriesPerGoroutine
	if expectedSize > 1000 {
		expectedSize = 1000
	}
	if lb.Size() != expectedSize {
		t.Errorf("Size() = %d, want %d", lb.Size(), expectedSize)
	}
}

func TestLogBuffer_CircularWithGetRecent(t *testing.T) {
	lb := NewLogBuffer(5)

	// Fill buffer and overflow
	for i := 0; i < 10; i++ {
		lb.Add(game.LogEntry{
			Timestamp: int64(i * 10),
			Level:     game.LogLevelInfo,
			Component: game.LogComponentApp,
			Message:   "test",
		})
	}

	// Get last 3 entries (should be 70, 80, 90)
	entries := lb.GetRecent(3)
	if len(entries) != 3 {
		t.Fatalf("GetRecent(3) returned %d entries, want 3", len(entries))
	}

	expectedTimestamps := []int64{70, 80, 90}
	for i, entry := range entries {
		if entry.Timestamp != expectedTimestamps[i] {
			t.Errorf("entries[%d].Timestamp = %d, want %d", i, entry.Timestamp, expectedTimestamps[i])
		}
	}
}
