package protocol

import (
	"testing"
)

func TestParser_SingleMessage(t *testing.T) {
	p := NewParser()

	// BuzzClick v1 format: JSON followed by null terminator
	data := []byte(`{"ACTION":"HELLO","ID":"buzzer1"}` + "\x00")
	p.Append(data)

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", messages[0].Action)
	}

	if messages[0].ID != "buzzer1" {
		t.Errorf("Expected ID buzzer1, got %s", messages[0].ID)
	}
}

func TestParser_MultipleMessages(t *testing.T) {
	p := NewParser()

	// Two messages in sequence
	data := []byte(`{"ACTION":"HELLO","ID":"b1"}` + "\x00" + `{"ACTION":"BUTTON","ID":"b1"}` + "\x00")
	p.Append(data)

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Action != ActionHello {
		t.Errorf("Expected first action HELLO, got %s", messages[0].Action)
	}

	if messages[1].Action != ActionButton {
		t.Errorf("Expected second action BUTTON, got %s", messages[1].Action)
	}
}

func TestParser_FragmentedMessage(t *testing.T) {
	p := NewParser()

	// First fragment (incomplete)
	p.Append([]byte(`{"ACTION":"HEL`))

	messages, _ := p.Parse()
	if len(messages) != 0 {
		t.Fatalf("Expected 0 messages from incomplete data, got %d", len(messages))
	}

	// Second fragment (completes the message)
	p.Append([]byte(`LO","ID":"buzzer1"}` + "\x00"))

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", messages[0].Action)
	}
}

func TestParser_EmptyMessage(t *testing.T) {
	p := NewParser()

	// Empty content before null terminator
	data := []byte("\x00" + `{"ACTION":"HELLO"}` + "\x00")
	p.Append(data)

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message (empty skipped), got %d", len(messages))
	}
}

func TestParser_WhitespaceHandling(t *testing.T) {
	p := NewParser()

	// Message with leading/trailing whitespace
	data := []byte("  \n  " + `{"ACTION":"HELLO"}` + "  \n  \x00")
	p.Append(data)

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
}

func TestParser_InvalidJSON(t *testing.T) {
	p := NewParser()

	// Invalid JSON followed by valid message
	data := []byte(`{invalid json}` + "\x00" + `{"ACTION":"HELLO"}` + "\x00")
	p.Append(data)

	messages, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should skip invalid and parse valid
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message (invalid skipped), got %d", len(messages))
	}

	if messages[0].Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", messages[0].Action)
	}
}

func TestParser_BufferReset(t *testing.T) {
	p := NewParser()

	p.Append([]byte(`{"ACTION":"HELLO"}`))
	if p.BufferSize() == 0 {
		t.Error("Buffer should not be empty after append")
	}

	p.Reset()
	if p.BufferSize() != 0 {
		t.Error("Buffer should be empty after reset")
	}
}

func TestParser_BufferOverflowProtection(t *testing.T) {
	p := NewParser()

	// Add large amount of data without null terminator
	bigData := make([]byte, 70000)
	for i := range bigData {
		bigData[i] = 'x'
	}
	p.Append(bigData)

	// Buffer should be truncated
	if p.BufferSize() > 65536 {
		t.Errorf("Buffer overflow protection failed, size: %d", p.BufferSize())
	}
}

func TestParseSingle_Valid(t *testing.T) {
	data := []byte(`{"ACTION":"HELLO","ID":"buzzer1"}`)

	msg, err := ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}

	if msg.Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", msg.Action)
	}
}

func TestParseSingle_WithNullTerminator(t *testing.T) {
	data := []byte(`{"ACTION":"HELLO"}` + "\x00")

	msg, err := ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}

	if msg.Action != ActionHello {
		t.Errorf("Expected action HELLO, got %s", msg.Action)
	}
}

func TestParseSingle_Empty(t *testing.T) {
	_, err := ParseSingle([]byte{})
	if err != ErrIncompleteMessage {
		t.Errorf("Expected ErrIncompleteMessage for empty data")
	}
}

func TestParseSingle_OnlyWhitespace(t *testing.T) {
	_, err := ParseSingle([]byte("  \n\r  "))
	if err != ErrIncompleteMessage {
		t.Errorf("Expected ErrIncompleteMessage for whitespace-only data")
	}
}
