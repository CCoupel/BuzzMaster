package main

// Listener on the BuzzControl /ws/logs endpoint. Counts, per coexistence
// phase, the server log lines that betray WiFi degradation on the buzzer
// path: LED_SET ACK retries / expirations, buzzer WebSocket disconnects.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// LogCounters is one phase's tally.
type LogCounters struct {
	Entries            int `json:"entries"`
	AckReceived        int `json:"ack_received"` // DEBUG level — only visible if server log level is DEBUG
	AckRetry           int `json:"ack_retry"`
	AckExpired         int `json:"ack_expired"`
	BuzzerConnected    int `json:"buzzer_connected"`
	BuzzerDisconnected int `json:"buzzer_disconnected"`
	ButtonPress        int `json:"button_press"`
	Warn               int `json:"warn"`
	Error              int `json:"error"`
}

// classifyLogLine updates counters for one LOG_ENTRY.
func classifyLogLine(level, message string, c *LogCounters) {
	c.Entries++
	switch strings.ToUpper(level) {
	case "WARN", "WARNING":
		c.Warn++
	case "ERROR":
		c.Error++
	}
	switch {
	case strings.Contains(message, "AckManager: retry"):
		c.AckRetry++
	case strings.Contains(message, "AckManager: EXPIRED"):
		c.AckExpired++
	case strings.HasPrefix(message, "ACK received from"):
		c.AckReceived++
	case strings.HasPrefix(message, "Buzzer connected via WebSocket"):
		c.BuzzerConnected++
	case strings.HasPrefix(message, "Buzzer disconnected from WebSocket"):
		c.BuzzerDisconnected++
	case strings.HasPrefix(message, "BUTTON from") && !strings.Contains(message, "ignored"):
		c.ButtonPress++
	}
}

type wsLogMessage struct {
	Action string `json:"ACTION"`
	Msg    struct {
		Entry struct {
			Timestamp int64  `json:"timestamp"`
			Level     string `json:"level"`
			Component string `json:"component"`
			Message   string `json:"message"`
		} `json:"entry"`
	} `json:"MSG"`
}

// parseLogMessage returns level, message, ok for a LOG_ENTRY frame.
func parseLogMessage(data []byte) (level, message string, ok bool) {
	var m wsLogMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", false
	}
	if m.Action != "LOG_ENTRY" {
		return "", "", false
	}
	return m.Msg.Entry.Level, m.Msg.Entry.Message, true
}

// LogWatcher keeps one connection to /ws/logs and buckets counters by phase.
type LogWatcher struct {
	mu      sync.Mutex
	phase   string
	byPhase map[string]*LogCounters
	conn    *websocket.Conn
	errs    []string
}

func logsURL(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q in %s", u.Scheme, serverURL)
	}
	u.Path = "/ws/logs"
	u.RawQuery = ""
	return u.String(), nil
}

// startLogWatcher dials the server and starts consuming frames until ctx ends.
func startLogWatcher(ctx context.Context, serverURL string) (*LogWatcher, error) {
	wsURL, err := logsURL(serverURL)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", wsURL, err)
	}
	w := &LogWatcher{phase: "init", byPhase: map[string]*LogCounters{}, conn: conn}
	go w.loop(ctx)
	return w, nil
}

func (w *LogWatcher) loop(ctx context.Context) {
	defer w.conn.Close()
	go func() {
		<-ctx.Done()
		_ = w.conn.SetReadDeadline(time.Now())
	}()
	for {
		_, data, err := w.conn.ReadMessage()
		if err != nil {
			if ctx.Err() == nil {
				w.mu.Lock()
				w.errs = append(w.errs, err.Error())
				w.mu.Unlock()
			}
			return
		}
		level, message, ok := parseLogMessage(data)
		if !ok {
			continue
		}
		w.mu.Lock()
		c := w.byPhase[w.phase]
		if c == nil {
			c = &LogCounters{}
			w.byPhase[w.phase] = c
		}
		classifyLogLine(level, message, c)
		w.mu.Unlock()
	}
}

// SetPhase switches the bucket new entries are counted into.
func (w *LogWatcher) SetPhase(name string) {
	w.mu.Lock()
	w.phase = name
	if _, ok := w.byPhase[name]; !ok {
		w.byPhase[name] = &LogCounters{}
	}
	w.mu.Unlock()
}

// Snapshot copies the per-phase counters.
func (w *LogWatcher) Snapshot() map[string]LogCounters {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := map[string]LogCounters{}
	for k, v := range w.byPhase {
		out[k] = *v
	}
	return out
}

// Errors returns read errors seen while the context was still live.
func (w *LogWatcher) Errors() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.errs...)
}
