package server

import (
	"buzzcontrol/internal/protocol"
	"net"
	"testing"
	"time"
)

func TestTCPServer_StartStop(t *testing.T) {
	server := NewTCPServer(0) // Use port 0 for random available port

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	server.Stop()
}

func TestTCPServer_AcceptConnection(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get the actual port
	addr := server.listener.Addr().(*net.TCPAddr)

	// Connect as client
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Wait for connection to be registered
	time.Sleep(100 * time.Millisecond)

	if server.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", server.ClientCount())
	}
}

func TestTCPServer_ReceiveMessage(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	// Connect as client
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send a BuzzClick v1 format message (JSON + null terminator)
	message := `{"ACTION":"HELLO","ID":"test-buzzer","MSG":{"VERSION":"1.0"}}` + "\x00"
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for message to be processed
	select {
	case incoming := <-server.Incoming:
		if incoming.Data.Action != protocol.ActionHello {
			t.Errorf("Expected action HELLO, got %s", incoming.Data.Action)
		}
		if incoming.Data.ID != "test-buzzer" {
			t.Errorf("Expected ID test-buzzer, got %s", incoming.Data.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestTCPServer_ReceiveMultipleMessages(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send multiple messages in one write
	messages := `{"ACTION":"HELLO","ID":"b1"}` + "\x00" + `{"ACTION":"BUTTON","ID":"b1"}` + "\x00"
	_, err = conn.Write([]byte(messages))
	if err != nil {
		t.Fatalf("Failed to send messages: %v", err)
	}

	// Should receive both
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < 2 {
		select {
		case <-server.Incoming:
			receivedCount++
		case <-timeout:
			t.Fatalf("Timeout: only received %d messages", receivedCount)
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 messages, received %d", receivedCount)
	}
}

func TestTCPServer_SendToClient(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// First, send a message to register the client ID
	_, err = conn.Write([]byte(`{"ACTION":"HELLO","ID":"test-client"}` + "\x00"))
	if err != nil {
		t.Fatalf("Failed to send hello: %v", err)
	}

	// Drain the incoming channel
	select {
	case <-server.Incoming:
	case <-time.After(1 * time.Second):
	}

	time.Sleep(100 * time.Millisecond)

	// Now send a message from server to client
	msg, _ := protocol.NewMessage(protocol.ActionPing, nil)
	err = server.SendToClient("test-client", msg)
	if err != nil {
		t.Errorf("SendToClient failed: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if n == 0 {
		t.Error("Expected data from server")
	}
}

func TestTCPServer_SendToAll(t *testing.T) {
	// NOTE: Cannot test with multiple localhost clients because TCP server
	// removes duplicate IP connections. Testing with single client only.
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	// Connect single client
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	// Send to all
	msg, _ := protocol.NewMessage(protocol.ActionStart, nil)
	server.SendToAll(msg)

	// Client should receive
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		t.Error("Client did not receive broadcast message")
	}
}

func TestTCPServer_GetClients(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	// Initial state
	clients := server.GetClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients initially, got %d", len(clients))
	}

	// Connect
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	clients = server.GetClients()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(clients))
	}
}

func TestTCPServer_ClientDisconnect(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if server.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", server.ClientCount())
	}

	// Disconnect
	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if server.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", server.ClientCount())
	}
}

func TestTCPServer_FragmentedMessage(t *testing.T) {
	server := NewTCPServer(0)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().(*net.TCPAddr)

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send message in fragments
	conn.Write([]byte(`{"ACTION":`))
	time.Sleep(50 * time.Millisecond)
	conn.Write([]byte(`"HELLO",`))
	time.Sleep(50 * time.Millisecond)
	conn.Write([]byte(`"ID":"fragmented"}` + "\x00"))

	// Should receive complete message
	select {
	case incoming := <-server.Incoming:
		if incoming.Data.Action != protocol.ActionHello {
			t.Errorf("Expected action HELLO, got %s", incoming.Data.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for fragmented message")
	}
}
