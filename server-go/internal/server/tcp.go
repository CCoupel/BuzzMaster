package server

import (
	"buzzcontrol/internal/protocol"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// TCPClient represents a connected buzzer
type TCPClient struct {
	ID       string
	Conn     net.Conn
	Parser   *protocol.Parser
	LastSeen time.Time
}

// TCPServer handles TCP connections from BuzzClick buzzers
type TCPServer struct {
	port     int
	listener net.Listener
	clients  map[string]*TCPClient
	mu       sync.RWMutex

	// Channel for incoming messages
	Incoming chan *protocol.IncomingMessage

	// Callback for handling messages
	OnMessage func(clientID string, msg *protocol.Message)
}

// NewTCPServer creates a new TCP server
func NewTCPServer(port int) *TCPServer {
	return &TCPServer{
		port:     port,
		clients:  make(map[string]*TCPClient),
		Incoming: make(chan *protocol.IncomingMessage, 100),
	}
}

// Start begins listening for TCP connections
func (s *TCPServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}

	s.listener = listener
	log.Printf("[TCP] Server started on port %d", s.port)

	go s.acceptLoop()
	return nil
}

// Stop closes the TCP server
func (s *TCPServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	for _, client := range s.clients {
		client.Conn.Close()
	}
	s.clients = make(map[string]*TCPClient)
	s.mu.Unlock()

	close(s.Incoming)
}

func (s *TCPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("[TCP] Accept error: %v", err)
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *TCPServer) handleConnection(conn net.Conn) {
	clientAddr := conn.RemoteAddr().String()
	clientIP, _, _ := net.SplitHostPort(clientAddr)

	log.Printf("[TCP] New connection from %s", clientAddr)

	// Remove any existing connection from same IP
	s.removeClientByIP(clientIP)

	client := &TCPClient{
		ID:       clientIP,
		Conn:     conn,
		Parser:   protocol.NewParser(),
		LastSeen: time.Now(),
	}

	s.mu.Lock()
	s.clients[clientIP] = client
	s.mu.Unlock()

	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.clients, clientIP)
		s.mu.Unlock()
		log.Printf("[TCP] Connection closed: %s", clientAddr)
	}()

	// Read loop
	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("[TCP] Read error from %s: %v", clientAddr, err)
			return
		}

		if n > 0 {
			client.LastSeen = time.Now()
			client.Parser.Append(buf[:n])

			messages, err := client.Parser.Parse()
			if err != nil {
				log.Printf("[TCP] Parse error from %s: %v", clientAddr, err)
				continue
			}

			for _, msg := range messages {
				log.Printf("[TCP] Received from %s: ACTION=%s", clientAddr, msg.Action)

				// Use ID from message if available (MAC address)
				clientID := clientIP
				if msg.ID != "" {
					clientID = msg.ID
					// Update client ID to MAC address
					s.mu.Lock()
					if c, ok := s.clients[clientIP]; ok {
						c.ID = msg.ID
					}
					s.mu.Unlock()
				}

				incoming := &protocol.IncomingMessage{
					Source:    "TCP",
					Data:      msg,
					ClientID:  clientID,
					Timestamp: time.Now(),
				}

				select {
				case s.Incoming <- incoming:
				default:
					log.Printf("[TCP] Incoming channel full, dropping message")
				}

				if s.OnMessage != nil {
					s.OnMessage(clientID, msg)
				}
			}
		}
	}
}

func (s *TCPServer) removeClientByIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, client := range s.clients {
		clientIP, _, _ := net.SplitHostPort(client.Conn.RemoteAddr().String())
		if clientIP == ip {
			log.Printf("[TCP] Removing existing connection from %s", ip)
			client.Conn.Close()
			delete(s.clients, id)
		}
	}
}

// SendToClient sends a message to a specific client by ID (MAC address)
func (s *TCPServer) SendToClient(clientID string, msg *protocol.Message) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find client by ID (MAC) or IP
	for _, client := range s.clients {
		if client.ID == clientID {
			data, err := msg.Serialize()
			if err != nil {
				return err
			}
			_, err = client.Conn.Write(data)
			return err
		}
	}

	return fmt.Errorf("client not found: %s", clientID)
}

// SendToAll sends a message to all connected TCP clients
func (s *TCPServer) SendToAll(msg *protocol.Message) {
	data, err := msg.Serialize()
	if err != nil {
		log.Printf("[TCP] Failed to serialize message: %v", err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if _, err := client.Conn.Write(data); err != nil {
			log.Printf("[TCP] Failed to send to %s: %v", client.ID, err)
		}
	}
}

// GetClients returns list of connected client IDs
func (s *TCPServer) GetClients() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.clients))
	for _, client := range s.clients {
		ids = append(ids, client.ID)
	}
	return ids
}

// ClientCount returns number of connected clients
func (s *TCPServer) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}
