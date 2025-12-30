package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
)

var (
	ErrIncompleteMessage = errors.New("incomplete message")
	ErrInvalidJSON       = errors.New("invalid JSON")
)

// Parser handles null-terminated JSON messages from BuzzClick v1
type Parser struct {
	buffer []byte
}

// NewParser creates a new protocol parser
func NewParser() *Parser {
	return &Parser{
		buffer: make([]byte, 0, 8192),
	}
}

// Append adds data to the buffer
func (p *Parser) Append(data []byte) {
	p.buffer = append(p.buffer, data...)

	// Prevent buffer overflow
	if len(p.buffer) > 65536 {
		log.Printf("Parser buffer overflow, truncating")
		p.buffer = p.buffer[len(p.buffer)-8192:]
	}
}

// Parse extracts complete messages from buffer
// BuzzClick v1 uses null-terminated JSON: {...}\0
func (p *Parser) Parse() ([]*Message, error) {
	var messages []*Message

	for {
		// Find null terminator
		idx := bytes.IndexByte(p.buffer, 0)
		if idx == -1 {
			// No complete message yet
			break
		}

		// Extract JSON part (before null)
		jsonData := p.buffer[:idx]

		// Remove processed data from buffer (including null)
		p.buffer = p.buffer[idx+1:]

		// Skip empty messages
		if len(jsonData) == 0 {
			continue
		}

		// Trim any leading/trailing whitespace or newlines
		jsonData = bytes.TrimSpace(jsonData)
		if len(jsonData) == 0 {
			continue
		}

		// Parse JSON
		var msg Message
		if err := json.Unmarshal(jsonData, &msg); err != nil {
			log.Printf("Failed to parse JSON: %s, error: %v", string(jsonData), err)
			continue
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// Reset clears the buffer
func (p *Parser) Reset() {
	p.buffer = p.buffer[:0]
}

// BufferSize returns current buffer size
func (p *Parser) BufferSize() int {
	return len(p.buffer)
}

// ParseSingle parses a single JSON message (for WebSocket)
func ParseSingle(data []byte) (*Message, error) {
	// Trim null terminators and whitespace
	data = bytes.TrimRight(data, "\x00\n\r ")
	data = bytes.TrimSpace(data)

	if len(data) == 0 {
		return nil, ErrIncompleteMessage
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}
